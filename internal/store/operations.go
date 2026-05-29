package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	OperationStatusQueued    = "queued"
	OperationStatusRunning   = "running"
	OperationStatusSucceeded = "succeeded"
	OperationStatusFailed    = "failed"

	OperationStepStatusPending   = "pending"
	OperationStepStatusRunning   = "running"
	OperationStepStatusSucceeded = "succeeded"
	OperationStepStatusFailed    = "failed"
)

type Operation struct {
	ID            string           `json:"id"`
	Kind          string           `json:"kind"`
	ResourceType  string           `json:"resource_type"`
	ResourceID    string           `json:"resource_id,omitempty"`
	ResourceName  string           `json:"resource_name,omitempty"`
	Status        string           `json:"status"`
	Summary       string           `json:"summary,omitempty"`
	Error         string           `json:"error,omitempty"`
	ActorUserID   string           `json:"actor_user_id,omitempty"`
	ActorUsername string           `json:"actor_username,omitempty"`
	Metadata      json.RawMessage  `json:"metadata,omitempty"`
	Steps         []*OperationStep `json:"steps,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	StartedAt     *time.Time       `json:"started_at,omitempty"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type OperationStep struct {
	ID          string     `json:"id"`
	OperationID string     `json:"operation_id"`
	Position    int        `json:"position"`
	StepKey     string     `json:"step_key"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Details     string     `json:"details,omitempty"`
	Output      string     `json:"output,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (db *DB) CreateOperation(op *Operation) error {
	metadata := ""
	if len(op.Metadata) > 0 {
		metadata = string(op.Metadata)
	}
	_, err := db.Exec(`
		INSERT INTO operations (
			id, kind, resource_type, resource_id, resource_name, status, summary, error,
			actor_user_id, actor_username, metadata, created_at, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.ID, op.Kind, op.ResourceType, op.ResourceID, op.ResourceName, op.Status, op.Summary, op.Error,
		op.ActorUserID, op.ActorUsername, metadata, op.CreatedAt, op.StartedAt, op.FinishedAt,
	)
	return err
}

func (db *DB) SetOperationRunning(id string, startedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE operations
		SET status = ?, started_at = ?, error = ''
		WHERE id = ?`,
		OperationStatusRunning, startedAt, id,
	)
	return err
}

func (db *DB) CompleteOperation(id, summary string, finishedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE operations
		SET status = ?, summary = ?, error = '', finished_at = ?
		WHERE id = ?`,
		OperationStatusSucceeded, summary, finishedAt, id,
	)
	return err
}

func (db *DB) FailOperation(id, summary, failure string, finishedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE operations
		SET status = ?, summary = ?, error = ?, finished_at = ?
		WHERE id = ?`,
		OperationStatusFailed, summary, failure, finishedAt, id,
	)
	return err
}

func (db *DB) UpdateOperationResource(id, resourceType, resourceID, resourceName string) error {
	_, err := db.Exec(`
		UPDATE operations
		SET resource_type = ?, resource_id = ?, resource_name = ?
		WHERE id = ?`,
		resourceType, resourceID, resourceName, id,
	)
	return err
}

func (db *DB) SetOperationSummary(id, summary string) error {
	_, err := db.Exec(`UPDATE operations SET summary = ? WHERE id = ?`, summary, id)
	return err
}

func (db *DB) CreateOperationStep(step *OperationStep) error {
	_, err := db.Exec(`
		INSERT INTO operation_steps (
			id, operation_id, position, step_key, title, status, details, output, created_at, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.ID, step.OperationID, step.Position, step.StepKey, step.Title, step.Status,
		step.Details, step.Output, step.CreatedAt, step.StartedAt, step.FinishedAt,
	)
	return err
}

func (db *DB) StartOperationStep(id string, startedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE operation_steps
		SET status = ?, started_at = ?, finished_at = NULL
		WHERE id = ?`,
		OperationStepStatusRunning, startedAt, id,
	)
	return err
}

func (db *DB) FinishOperationStep(id, status, details string, finishedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE operation_steps
		SET status = ?, details = ?, finished_at = ?
		WHERE id = ?`,
		status, details, finishedAt, id,
	)
	return err
}

func (db *DB) AppendOperationStepOutput(id, chunk string) error {
	if chunk == "" {
		return nil
	}
	_, err := db.Exec(`
		UPDATE operation_steps
		SET output = CASE
			WHEN output = '' THEN ?
			ELSE output || char(10) || ?
		END
		WHERE id = ?`,
		chunk, chunk, id,
	)
	return err
}

func (db *DB) ListOperations(limit int, resourceType, resourceID string) ([]*Operation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT id, kind, resource_type, COALESCE(resource_id, ''), COALESCE(resource_name, ''), status,
		       COALESCE(summary, ''), COALESCE(error, ''), COALESCE(actor_user_id, ''),
		       COALESCE(actor_username, ''), COALESCE(metadata, ''), created_at, started_at, finished_at, updated_at
		FROM operations`
	args := make([]any, 0, 3)
	where := ""
	if resourceType != "" {
		where = " WHERE resource_type = ?"
		args = append(args, resourceType)
		if resourceID != "" {
			where += " AND resource_id = ?"
			args = append(args, resourceID)
		}
	} else if resourceID != "" {
		where = " WHERE resource_id = ?"
		args = append(args, resourceID)
	}
	query += where + " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []*Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (db *DB) GetOperation(id string) (*Operation, error) {
	row := db.QueryRow(`
		SELECT id, kind, resource_type, COALESCE(resource_id, ''), COALESCE(resource_name, ''), status,
		       COALESCE(summary, ''), COALESCE(error, ''), COALESCE(actor_user_id, ''),
		       COALESCE(actor_username, ''), COALESCE(metadata, ''), created_at, started_at, finished_at, updated_at
		FROM operations
		WHERE id = ?`, id)
	op, err := scanOperation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	steps, err := db.ListOperationSteps(id)
	if err != nil {
		return nil, err
	}
	op.Steps = steps
	return op, nil
}

func (db *DB) ListOperationSteps(operationID string) ([]*OperationStep, error) {
	rows, err := db.Query(`
		SELECT id, operation_id, position, step_key, title, status, COALESCE(details, ''), COALESCE(output, ''),
		       created_at, started_at, finished_at, updated_at
		FROM operation_steps
		WHERE operation_id = ?
		ORDER BY position ASC, created_at ASC`, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*OperationStep
	for rows.Next() {
		step, err := scanOperationStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

type operationScanner interface {
	Scan(dest ...interface{}) error
}

func scanOperation(row operationScanner) (*Operation, error) {
	op := &Operation{}
	var resourceID sql.NullString
	var metadata string
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := row.Scan(
		&op.ID,
		&op.Kind,
		&op.ResourceType,
		&resourceID,
		&op.ResourceName,
		&op.Status,
		&op.Summary,
		&op.Error,
		&op.ActorUserID,
		&op.ActorUsername,
		&metadata,
		&op.CreatedAt,
		&startedAt,
		&finishedAt,
		&op.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if resourceID.Valid {
		op.ResourceID = resourceID.String
	}
	if metadata != "" {
		op.Metadata = json.RawMessage(metadata)
	}
	if startedAt.Valid {
		op.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		op.FinishedAt = &finishedAt.Time
	}
	return op, nil
}

func scanOperationStep(row operationScanner) (*OperationStep, error) {
	step := &OperationStep{}
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := row.Scan(
		&step.ID,
		&step.OperationID,
		&step.Position,
		&step.StepKey,
		&step.Title,
		&step.Status,
		&step.Details,
		&step.Output,
		&step.CreatedAt,
		&startedAt,
		&finishedAt,
		&step.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if startedAt.Valid {
		step.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		step.FinishedAt = &finishedAt.Time
	}
	return step, nil
}

func (db *DB) DebugOperationCount() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM operations`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count operations: %w", err)
	}
	return count, nil
}
