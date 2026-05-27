package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Yash121l/Vessel/internal/logger"
)

// Deployment represents a deployed application instance.
type Deployment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	AppID       string            `json:"app_id"`
	Status      string            `json:"status"` // stopped, running, error, updating
	Domain      string            `json:"domain"`
	ComposeDir  string            `json:"compose_dir"`
	Imported    bool              `json:"imported"`     // true = discovered from docker ps, not deployed by Vessel
	ContainerID string            `json:"container_id"` // for imported containers
	Image       string            `json:"image"`        // for imported containers
	Ports       string            `json:"ports"`        // for imported containers, comma-separated
	Env         map[string]string `json:"env,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CreateDeployment inserts a new deployment record.
func (db *DB) CreateDeployment(d *Deployment) error {
	logger.Infof("Creating store database record for deployment '%s'...", d.Name)
	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("failed to start database transaction for CreateDeployment: %v", err)
		return err
	}
	defer tx.Rollback()

	imported := 0
	if d.Imported {
		imported = 1
	}
	_, err = tx.Exec(`
		INSERT INTO deployments (id, name, app_id, status, domain, compose_dir, imported, container_id, image, ports)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.AppID, d.Status, d.Domain, d.ComposeDir, imported, d.ContainerID, d.Image, d.Ports,
	)
	if err != nil {
		return fmt.Errorf("insert deployment: %w", err)
	}

	for k, v := range d.Env {
		_, err = tx.Exec(`
			INSERT INTO deployment_env (deployment_id, key, value) VALUES (?, ?, ?)`,
			d.ID, k, v,
		)
		if err != nil {
			return fmt.Errorf("insert env %s: %w", k, err)
		}
	}

	return tx.Commit()
}

// GetDeployment retrieves a deployment by ID.
func (db *DB) GetDeployment(id string) (*Deployment, error) {
	d := &Deployment{}
	var imported int
	err := db.QueryRow(`
		SELECT id, name, app_id, status, COALESCE(domain,''), compose_dir, imported,
		       COALESCE(container_id,''), COALESCE(image,''), COALESCE(ports,''), created_at, updated_at
		FROM deployments WHERE id = ?`, id,
	).Scan(&d.ID, &d.Name, &d.AppID, &d.Status, &d.Domain, &d.ComposeDir, &imported,
		&d.ContainerID, &d.Image, &d.Ports, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Imported = imported == 1
	d.Env, err = db.getEnv(d.ID)
	return d, err
}

// GetDeploymentByName retrieves a deployment by name.
func (db *DB) GetDeploymentByName(name string) (*Deployment, error) {
	d := &Deployment{}
	var imported int
	err := db.QueryRow(`
		SELECT id, name, app_id, status, COALESCE(domain,''), compose_dir, imported,
		       COALESCE(container_id,''), COALESCE(image,''), COALESCE(ports,''), created_at, updated_at
		FROM deployments WHERE name = ?`, name,
	).Scan(&d.ID, &d.Name, &d.AppID, &d.Status, &d.Domain, &d.ComposeDir, &imported,
		&d.ContainerID, &d.Image, &d.Ports, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Imported = imported == 1
	d.Env, err = db.getEnv(d.ID)
	return d, err
}

// ListDeployments returns all deployments.
func (db *DB) ListDeployments() ([]*Deployment, error) {
	rows, err := db.Query(`
		SELECT id, name, app_id, status, COALESCE(domain,''), compose_dir, imported,
		       COALESCE(container_id,''), COALESCE(image,''), COALESCE(ports,''), created_at, updated_at
		FROM deployments ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*Deployment
	for rows.Next() {
		d := &Deployment{}
		var imported int
		if err := rows.Scan(&d.ID, &d.Name, &d.AppID, &d.Status, &d.Domain, &d.ComposeDir, &imported,
			&d.ContainerID, &d.Image, &d.Ports, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Imported = imported == 1
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

// UpdateDeploymentStatus updates the status field.
func (db *DB) UpdateDeploymentStatus(id, status string) error {
	logger.Debugf("Updating database deployment ID '%s' status to '%s'...", id, status)
	_, err := db.Exec(`UPDATE deployments SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		logger.Errorf("failed to update status: %v", err)
	}
	return err
}

// UpdateDeploymentDomain updates the domain field.
func (db *DB) UpdateDeploymentDomain(id, domain string) error {
	logger.Debugf("Updating database deployment ID '%s' domain to '%s'...", id, domain)
	_, err := db.Exec(`UPDATE deployments SET domain = ? WHERE id = ?`, domain, id)
	if err != nil {
		logger.Errorf("failed to update domain: %v", err)
	}
	return err
}

// UpdateContainerID updates the container_id for an imported deployment.
func (db *DB) UpdateContainerID(id, containerID string) error {
	logger.Debugf("Updating database deployment ID '%s' container_id to '%s'...", id, containerID)
	_, err := db.Exec(`UPDATE deployments SET container_id = ? WHERE id = ?`, containerID, id)
	if err != nil {
		logger.Errorf("failed to update container_id: %v", err)
	}
	return err
}

// DeleteDeployment removes a deployment and its env vars.
func (db *DB) DeleteDeployment(id string) error {
	logger.Infof("Deleting database deployment record for ID '%s'...", id)
	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("failed to start database transaction for DeleteDeployment: %v", err)
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM deployment_env WHERE deployment_id = ?`, id); err != nil {
		logger.Errorf("failed to delete env: %v", err)
		return err
	}
	if _, err := tx.Exec(`DELETE FROM deployments WHERE id = ?`, id); err != nil {
		logger.Errorf("failed to delete deployment: %v", err)
		return err
	}
	return tx.Commit()
}

// UpdateDeploymentEnv replaces all env vars for a deployment.
func (db *DB) UpdateDeploymentEnv(id string, env map[string]string) error {
	logger.Infof("Replacing database environment variables for deployment ID '%s'...", id)
	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("failed to start database transaction for UpdateDeploymentEnv: %v", err)
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM deployment_env WHERE deployment_id = ?`, id); err != nil {
		logger.Errorf("failed to delete env: %v", err)
		return err
	}
	for k, v := range env {
		if _, err := tx.Exec(`INSERT INTO deployment_env (deployment_id, key, value) VALUES (?, ?, ?)`, id, k, v); err != nil {
			logger.Errorf("failed to insert env '%s': %v", k, err)
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) getEnv(deploymentID string) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM deployment_env WHERE deployment_id = ?`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	env := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		env[k] = v
	}
	return env, rows.Err()
}

// GetSetting retrieves a settings value.
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting upserts a settings value.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
