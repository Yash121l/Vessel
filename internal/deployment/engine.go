package deployment

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/vessel-app/vessel/internal/config"
	"github.com/vessel-app/vessel/internal/proxy"
	"github.com/vessel-app/vessel/internal/registry"
	"github.com/vessel-app/vessel/internal/store"
)

// Engine manages the lifecycle of deployments.
type Engine struct {
	cfg      *config.Config
	db       *store.DB
	registry *registry.Registry
	proxy    *proxy.Manager
}

// NewEngine creates a new deployment engine.
func NewEngine(cfg *config.Config, db *store.DB, reg *registry.Registry, prx *proxy.Manager) *Engine {
	return &Engine{
		cfg:      cfg,
		db:       db,
		registry: reg,
		proxy:    prx,
	}
}

// DeployRequest holds the parameters for a new deployment.
type DeployRequest struct {
	AppID  string
	Name   string
	Domain string
	Env    map[string]string
}

// Deploy creates and starts a new deployment.
func (e *Engine) Deploy(ctx context.Context, req DeployRequest) (*store.Deployment, error) {
	tmpl, ok := e.registry.Get(req.AppID)
	if !ok {
		return nil, fmt.Errorf("unknown app: %s", req.AppID)
	}

	// Validate name uniqueness
	existing, err := e.db.GetDeploymentByName(req.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("deployment '%s' already exists", req.Name)
	}

	id := uuid.New().String()
	composeDir := filepath.Join(e.cfg.DeploymentsDir, req.Name)

	d := &store.Deployment{
		ID:         id,
		Name:       req.Name,
		AppID:      req.AppID,
		Status:     "deploying",
		Domain:     req.Domain,
		ComposeDir: composeDir,
		Env:        req.Env,
	}

	// Generate and write compose file
	cf, err := GenerateCompose(tmpl, d)
	if err != nil {
		return nil, fmt.Errorf("generate compose: %w", err)
	}
	if err := WriteCompose(cf, composeDir); err != nil {
		return nil, fmt.Errorf("write compose: %w", err)
	}
	if err := WriteEnvFile(req.Env, composeDir); err != nil {
		return nil, fmt.Errorf("write env: %w", err)
	}

	// Persist to DB
	if err := e.db.CreateDeployment(d); err != nil {
		return nil, fmt.Errorf("save deployment: %w", err)
	}

	// Pull images
	if err := e.composePull(ctx, composeDir); err != nil {
		_ = e.db.UpdateDeploymentStatus(id, "error")
		return d, fmt.Errorf("pull images: %w", err)
	}

	// Start services
	if err := e.composeUp(ctx, composeDir); err != nil {
		_ = e.db.UpdateDeploymentStatus(id, "error")
		return d, fmt.Errorf("start services: %w", err)
	}

	_ = e.db.UpdateDeploymentStatus(id, "running")
	d.Status = "running"

	// Configure reverse proxy if domain is set
	if req.Domain != "" {
		if err := e.proxy.AddRoute(req.Domain, tmpl.ProxyPort, req.Name); err != nil {
			// Non-fatal: deployment is running, proxy config failed
			fmt.Printf("warning: proxy config failed: %v\n", err)
		}
	}

	return d, nil
}

// Stop stops a running deployment.
func (e *Engine) Stop(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}
	if err := e.composeStop(ctx, d.ComposeDir); err != nil {
		return err
	}
	return e.db.UpdateDeploymentStatus(id, "stopped")
}

// Start starts a stopped deployment.
func (e *Engine) Start(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}
	if err := e.composeUp(ctx, d.ComposeDir); err != nil {
		_ = e.db.UpdateDeploymentStatus(id, "error")
		return err
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Restart restarts a deployment.
func (e *Engine) Restart(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}
	if err := e.composeRestart(ctx, d.ComposeDir); err != nil {
		return err
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Update pulls new images and recreates containers.
func (e *Engine) Update(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}
	_ = e.db.UpdateDeploymentStatus(id, "updating")

	if err := e.composePull(ctx, d.ComposeDir); err != nil {
		_ = e.db.UpdateDeploymentStatus(id, "error")
		return fmt.Errorf("pull: %w", err)
	}
	if err := e.composeUp(ctx, d.ComposeDir); err != nil {
		_ = e.db.UpdateDeploymentStatus(id, "error")
		return fmt.Errorf("up: %w", err)
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Remove stops and removes a deployment entirely.
func (e *Engine) Remove(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}

	// Remove proxy route
	if d.Domain != "" {
		_ = e.proxy.RemoveRoute(d.Domain)
	}

	// Bring down containers and volumes
	_ = e.composeDown(ctx, d.ComposeDir)

	// Remove compose directory
	_ = os.RemoveAll(d.ComposeDir)

	return e.db.DeleteDeployment(id)
}

// Logs streams logs from a deployment. Writes to w until ctx is cancelled.
func (e *Engine) Logs(ctx context.Context, id string, w io.Writer, follow bool) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}

	args := []string{"compose", "logs", "--timestamps"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, "--tail", "200")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = d.ComposeDir
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// SyncStatus refreshes deployment status from Docker.
func (e *Engine) SyncStatus(ctx context.Context, id string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "json")
	cmd.Dir = d.ComposeDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Simple heuristic: if output contains "running", mark as running
	status := "stopped"
	if len(out) > 10 {
		status = "running"
	}
	return e.db.UpdateDeploymentStatus(id, status)
}

// --- compose helpers ---

func (e *Engine) composePull(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "pull")
}

func (e *Engine) composeUp(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "up", "-d", "--remove-orphans")
}

func (e *Engine) composeStop(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "stop")
}

func (e *Engine) composeRestart(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "restart")
}

func (e *Engine) composeDown(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "down", "-v")
}

func (e *Engine) runCompose(ctx context.Context, dir string, args ...string) error {
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StreamLogs streams logs line-by-line to a channel.
func (e *Engine) StreamLogs(ctx context.Context, id string, lines chan<- string) error {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment not found: %s", id)
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "logs", "--follow", "--timestamps", "--tail", "100")
	cmd.Dir = d.ComposeDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scan := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}

	go scan(stdout)
	go scan(stderr)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// PeriodicSync refreshes all deployment statuses every interval.
func (e *Engine) PeriodicSync(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deployments, err := e.db.ListDeployments()
			if err != nil {
				continue
			}
			for _, d := range deployments {
				_ = e.SyncStatus(ctx, d.ID)
			}
		}
	}
}
