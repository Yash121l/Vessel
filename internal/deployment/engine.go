package deployment

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/docker"
	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/nginx"
	"github.com/Yash121l/Vessel/internal/operations"
	"github.com/Yash121l/Vessel/internal/proxy"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
	"github.com/google/uuid"
)

// Engine manages the lifecycle of deployments.
type Engine struct {
	cfg      *config.Config
	db       *store.DB
	registry *registry.Registry
	nginx    *nginx.Manager
}

// NewEngine creates a new deployment engine.
func NewEngine(cfg *config.Config, db *store.DB, reg *registry.Registry, prx *proxy.Manager) *Engine {
	_ = prx
	return &Engine{
		cfg:      cfg,
		db:       db,
		registry: reg,
		nginx:    nginx.NewManager(),
	}
}

// DeployRequest holds the parameters for a new deployment.
type DeployRequest struct {
	AppID  string
	Name   string
	Domain string
	Env    map[string]string
	// SkipServices is a set of optional sidecar service names to omit from the
	// generated compose file. The user is responsible for providing those
	// services externally (e.g. an existing database).
	SkipServices map[string]bool
}

// Deploy creates and starts a new deployment.
func (e *Engine) Deploy(ctx context.Context, req DeployRequest) (*store.Deployment, error) {
	return e.DeployWithRun(ctx, req, nil)
}

func (e *Engine) DeployWithRun(ctx context.Context, req DeployRequest, run *operations.Run) (_ *store.Deployment, err error) {
	logger.Infof("Initiating deployment for app '%s' with name '%s'...", req.AppID, req.Name)
	tmpl, ok := e.registry.Get(req.AppID)
	if !ok {
		return nil, fmt.Errorf("unknown app: %s", req.AppID)
	}
	return e.deployWithTemplate(ctx, tmpl, req, run)
}

func (e *Engine) DeployTemplateWithRun(ctx context.Context, tmpl *registry.AppTemplate, req DeployRequest, run *operations.Run) (_ *store.Deployment, err error) {
	if tmpl == nil {
		return nil, fmt.Errorf("template is required")
	}
	return e.deployWithTemplate(ctx, tmpl, req, run)
}

func (e *Engine) deployWithTemplate(ctx context.Context, tmpl *registry.AppTemplate, req DeployRequest, run *operations.Run) (_ *store.Deployment, err error) {
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

	var (
		deploymentCreated bool
		siteConfigured    bool
	)

	defer func() {
		if err == nil {
			return
		}
		if run != nil {
			_ = run.Step(context.Background(), "cleanup_partial_deploy", "Cleanup partial deployment", func(_ context.Context, step *operations.Step) error {
				step.Logf("Cleaning up deployment directory %s", composeDir)
				if deploymentCreated {
					_ = e.db.UpdateDeploymentStatus(id, "error")
				}
				if composeDir != "" {
					if derr := e.composeDown(context.Background(), composeDir, step.Writer()); derr != nil {
						step.Logf("docker compose down cleanup failed: %v", derr)
					}
					if derr := os.RemoveAll(composeDir); derr != nil {
						step.Logf("failed to remove deployment directory: %v", derr)
					}
				}
				if siteConfigured {
					siteName := req.Name + ".conf"
					if derr := e.removeDeploymentSite(siteName, req.Domain); derr != nil {
						step.Logf("failed to remove nginx site during cleanup: %v", derr)
					}
				}
				if deploymentCreated {
					if derr := e.db.DeleteDeployment(id); derr != nil {
						step.Logf("failed to delete deployment record: %v", derr)
					}
				}
				return nil
			})
		} else {
			if deploymentCreated {
				_ = e.db.UpdateDeploymentStatus(id, "error")
			}
			_ = e.composeDown(context.Background(), composeDir, nil)
			_ = os.RemoveAll(composeDir)
			if siteConfigured {
				_ = e.removeDeploymentSite(req.Name+".conf", req.Domain)
			}
			if deploymentCreated {
				_ = e.db.DeleteDeployment(id)
			}
		}
	}()

	var cf *ComposeFile
	if err := stepRun(ctx, run, "generate_compose", "Generate compose bundle", func(_ *operations.Step) error {
		var stepErr error
		cf, stepErr = GenerateCompose(tmpl, d, req.SkipServices)
		if stepErr != nil {
			return fmt.Errorf("generate compose: %w", stepErr)
		}
		if stepErr = WriteCompose(cf, composeDir); stepErr != nil {
			return fmt.Errorf("write compose: %w", stepErr)
		}
		if stepErr = WriteEnvFile(req.Env, composeDir); stepErr != nil {
			return fmt.Errorf("write env: %w", stepErr)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := stepRun(ctx, run, "persist_deployment", "Persist deployment metadata", func(_ *operations.Step) error {
		if stepErr := e.db.CreateDeployment(d); stepErr != nil {
			return fmt.Errorf("save deployment: %w", stepErr)
		}
		deploymentCreated = true
		if run != nil {
			run.BindResource("deployment", d.ID, d.Name)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := stepRun(ctx, run, "pull_images", "Pull container images", func(step *operations.Step) error {
		if stepErr := e.composePull(ctx, composeDir, stepWriter(step)); stepErr != nil {
			return fmt.Errorf("pull images: %w", stepErr)
		}
		return nil
	}); err != nil {
		return d, err
	}

	if err := stepRun(ctx, run, "start_services", "Start compose services", func(step *operations.Step) error {
		if stepErr := e.composeUp(ctx, composeDir, stepWriter(step)); stepErr != nil {
			return fmt.Errorf("start services: %w", stepErr)
		}
		return nil
	}); err != nil {
		return d, err
	}

	if err := e.db.UpdateDeploymentStatus(id, "running"); err != nil {
		return d, err
	}
	d.Status = "running"

	if req.Domain != "" {
		if err := stepRun(ctx, run, "configure_proxy", "Configure nginx site", func(step *operations.Step) error {
			siteName := req.Name + ".conf"
			logger.Infof("Configuring reverse proxy route for domain '%s' targeting port '%d'...", req.Domain, proxyTargetPort(tmpl))
			if stepErr := e.nginx.ConfigureSiteForDeployment(siteName, req.Domain, proxyTargetPort(tmpl), "", req.Name); stepErr != nil {
				return fmt.Errorf("configure nginx site: %w", stepErr)
			}
			siteConfigured = true
			if stepErr := e.nginx.ObtainCertificate(req.Domain); stepErr != nil {
				step.Logf("certificate setup warning: %v", stepErr)
			}
			return nil
		}); err != nil {
			return d, err
		}
	}

	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s is running", d.Name))
	}
	return d, nil
}

func proxyTargetPort(tmpl *registry.AppTemplate) int {
	for _, p := range tmpl.Ports {
		if p.Internal == tmpl.ProxyPort {
			if p.External != 0 {
				return p.External
			}
			return p.Internal
		}
	}
	if tmpl.ProxyPort != 0 {
		return tmpl.ProxyPort
	}
	if len(tmpl.Ports) > 0 {
		if tmpl.Ports[0].External != 0 {
			return tmpl.Ports[0].External
		}
		return tmpl.Ports[0].Internal
	}
	return 80
}

// Stop stops a running deployment.
func (e *Engine) Stop(ctx context.Context, id string) error {
	return e.StopWithRun(ctx, id, nil)
}

func (e *Engine) StopWithRun(ctx context.Context, id string, run *operations.Run) error {
	logger.Infof("Stopping deployment ID '%s'...", id)
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		logger.Errorf("Stop failed: deployment not found for ID: %s", id)
		return fmt.Errorf("deployment not found: %s", id)
	}
	if run != nil {
		run.BindResource("deployment", d.ID, d.Name)
	}
	if err := stepRun(ctx, run, "stop_services", "Stop compose services", func(step *operations.Step) error {
		if stepErr := e.composeStop(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			logger.Errorf("Stop failed to halt docker compose services: %v", stepErr)
			return stepErr
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Infof("Successfully stopped deployment ID '%s'", id)
	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s stopped", d.Name))
	}
	return e.db.UpdateDeploymentStatus(id, "stopped")
}

// Start starts a stopped deployment.
func (e *Engine) Start(ctx context.Context, id string) error {
	return e.StartWithRun(ctx, id, nil)
}

func (e *Engine) StartWithRun(ctx context.Context, id string, run *operations.Run) error {
	logger.Infof("Starting deployment ID '%s'...", id)
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		logger.Errorf("Start failed: deployment not found for ID: %s", id)
		return fmt.Errorf("deployment not found: %s", id)
	}
	if run != nil {
		run.BindResource("deployment", d.ID, d.Name)
	}
	if err := stepRun(ctx, run, "start_services", "Start compose services", func(step *operations.Step) error {
		if stepErr := e.composeUp(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			logger.Errorf("Start failed to bring compose services up: %v", stepErr)
			_ = e.db.UpdateDeploymentStatus(id, "error")
			return stepErr
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Infof("Successfully started deployment ID '%s'", id)
	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s started", d.Name))
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Restart restarts a deployment.
func (e *Engine) Restart(ctx context.Context, id string) error {
	return e.RestartWithRun(ctx, id, nil)
}

func (e *Engine) RestartWithRun(ctx context.Context, id string, run *operations.Run) error {
	logger.Infof("Restarting deployment ID '%s'...", id)
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		logger.Errorf("Restart failed: deployment not found for ID: %s", id)
		return fmt.Errorf("deployment not found: %s", id)
	}
	if run != nil {
		run.BindResource("deployment", d.ID, d.Name)
	}
	if err := stepRun(ctx, run, "restart_services", "Restart compose services", func(step *operations.Step) error {
		if stepErr := e.composeRestart(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			logger.Errorf("Restart failed: %v", stepErr)
			return stepErr
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Infof("Successfully restarted deployment ID '%s'", id)
	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s restarted", d.Name))
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Update pulls new images and recreates containers.
func (e *Engine) Update(ctx context.Context, id string) error {
	return e.UpdateWithRun(ctx, id, nil)
}

func (e *Engine) UpdateWithRun(ctx context.Context, id string, run *operations.Run) error {
	logger.Infof("Initiating image pull and update for deployment ID '%s'...", id)
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		logger.Errorf("Update failed: deployment not found for ID: %s", id)
		return fmt.Errorf("deployment not found: %s", id)
	}
	if run != nil {
		run.BindResource("deployment", d.ID, d.Name)
	}
	_ = e.db.UpdateDeploymentStatus(id, "updating")

	if err := stepRun(ctx, run, "pull_images", "Pull updated images", func(step *operations.Step) error {
		if stepErr := e.composePull(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			logger.Errorf("Update failed during compose pull phase: %v", stepErr)
			_ = e.db.UpdateDeploymentStatus(id, "error")
			return fmt.Errorf("pull: %w", stepErr)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := stepRun(ctx, run, "recreate_services", "Recreate compose services", func(step *operations.Step) error {
		if stepErr := e.composeUp(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			logger.Errorf("Update failed during compose up phase: %v", stepErr)
			_ = e.db.UpdateDeploymentStatus(id, "error")
			return fmt.Errorf("up: %w", stepErr)
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Infof("Successfully completed update for deployment ID '%s'", id)
	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s updated", d.Name))
	}
	return e.db.UpdateDeploymentStatus(id, "running")
}

// Remove stops and removes a deployment entirely.
func (e *Engine) Remove(ctx context.Context, id string) error {
	return e.RemoveWithRun(ctx, id, nil)
}

func (e *Engine) RemoveWithRun(ctx context.Context, id string, run *operations.Run) error {
	logger.Infof("Removing deployment ID '%s' entirely...", id)
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		logger.Errorf("Remove failed: deployment not found for ID: %s", id)
		return fmt.Errorf("deployment not found: %s", id)
	}
	if run != nil {
		run.BindResource("deployment", d.ID, d.Name)
	}

	if d.Domain != "" {
		if err := stepRun(ctx, run, "remove_proxy", "Remove nginx site", func(step *operations.Step) error {
			logger.Infof("Removing proxy route for domain: %s", d.Domain)
			if stepErr := e.removeDeploymentSite(d.Name+".conf", d.Domain); stepErr != nil {
				step.Logf("remove nginx site warning: %v", stepErr)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if err := stepRun(ctx, run, "remove_services", "Remove compose services and volumes", func(step *operations.Step) error {
		logger.Infof("Stopping and tearing down docker containers and volumes in: %s", d.ComposeDir)
		if stepErr := e.composeDown(ctx, d.ComposeDir, stepWriter(step)); stepErr != nil {
			step.Logf("docker compose down warning: %v", stepErr)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := stepRun(ctx, run, "delete_bundle", "Delete deployment bundle", func(_ *operations.Step) error {
		logger.Infof("Removing compose directory from filesystem: %s", d.ComposeDir)
		if stepErr := os.RemoveAll(d.ComposeDir); stepErr != nil {
			return stepErr
		}
		return nil
	}); err != nil {
		return err
	}

	logger.Infof("Successfully removed deployment ID '%s' from store", id)
	if run != nil {
		run.SetSummary(fmt.Sprintf("Deployment %s removed", d.Name))
	}
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

// parseSyncStatusOutput parses the output of `docker compose ps --format json`
// and returns the deployment status string ("running" or "stopped"), or an
// empty string if the output is empty or unparseable (caller should leave
// status unchanged in that case).
//
// The output may be a JSON array (newer Docker Compose) or NDJSON — one JSON
// object per line (older versions). Both formats are handled.
func parseSyncStatusOutput(out []byte) (status string, ok bool) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return "", false
	}

	type svcEntry struct {
		State string `json:"State"`
	}
	var services []svcEntry

	if out[0] == '[' {
		if err := json.Unmarshal(out, &services); err != nil {
			return "", false // unparseable — leave unchanged
		}
	} else {
		// NDJSON: one JSON object per line
		for _, line := range bytes.Split(out, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var svc svcEntry
			if err := json.Unmarshal(line, &svc); err != nil {
				return "", false
			}
			services = append(services, svc)
		}
	}

	result := "stopped"
	for _, svc := range services {
		if svc.State == "running" {
			result = "running"
			break
		}
	}
	return result, true
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
		return nil // leave status unchanged on error
	}

	status, ok := parseSyncStatusOutput(out)
	if !ok {
		return nil // empty or unparseable — leave unchanged
	}
	return e.db.UpdateDeploymentStatus(id, status)
}

// --- compose helpers ---

func (e *Engine) composePull(ctx context.Context, dir string, output io.Writer) error {
	return e.runCompose(ctx, dir, output, "pull")
}

func (e *Engine) composeUp(ctx context.Context, dir string, output io.Writer) error {
	return e.runCompose(ctx, dir, output, "up", "-d", "--remove-orphans")
}

func (e *Engine) composeStop(ctx context.Context, dir string, output io.Writer) error {
	return e.runCompose(ctx, dir, output, "stop")
}

func (e *Engine) composeRestart(ctx context.Context, dir string, output io.Writer) error {
	return e.runCompose(ctx, dir, output, "restart")
}

func (e *Engine) composeDown(ctx context.Context, dir string, output io.Writer) error {
	return e.runCompose(ctx, dir, output, "down", "-v")
}

func (e *Engine) runCompose(ctx context.Context, dir string, output io.Writer, args ...string) error {
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	cmd.Dir = dir
	if output != nil {
		cmd.Stdout = output
		cmd.Stderr = output
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	logger.Debugf("Executing command: docker compose %s (working directory: %s)", strings.Join(args, " "), dir)
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

// SyncImportedStatus refreshes the container_id for imported deployments by looking up by name.
func (e *Engine) SyncImportedStatus(ctx context.Context) error {
	deployments, err := e.db.ListDeployments()
	if err != nil {
		return err
	}
	containers, err := docker.ListContainers(ctx)
	if err != nil {
		return nil // docker not available, skip
	}

	// Build name→container map
	byName := make(map[string]docker.Container)
	for _, c := range containers {
		byName[c.Name] = c
	}

	for _, d := range deployments {
		if !d.Imported {
			continue
		}
		c, ok := byName[d.Name]
		if !ok {
			// Container not found by name — mark stopped
			_ = e.db.UpdateDeploymentStatus(d.ID, "stopped")
			continue
		}
		// Update container_id in case it changed (restart)
		if c.ID != d.ContainerID {
			_ = e.db.UpdateContainerID(d.ID, c.ID)
		}
		status := "stopped"
		if c.State == "running" {
			status = "running"
		}
		_ = e.db.UpdateDeploymentStatus(d.ID, status)
	}
	return nil
}

// ComposeServiceInfo holds runtime info about a single service in a compose stack.
type ComposeServiceInfo struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created"`
}

// ComposeDetail holds the full detail of a managed deployment's compose stack.
type ComposeDetail struct {
	DeploymentID   string               `json:"deployment_id"`
	DeploymentName string               `json:"deployment_name"`
	ComposeDir     string               `json:"compose_dir"`
	ComposeYAML    string               `json:"compose_yaml"`
	Services       []ComposeServiceInfo `json:"services"`
}

// GetComposeDetail returns the compose file content and live service states for a deployment.
func (e *Engine) GetComposeDetail(ctx context.Context, id string) (*ComposeDetail, error) {
	d, err := e.db.GetDeployment(id)
	if err != nil || d == nil {
		return nil, fmt.Errorf("deployment not found: %s", id)
	}
	if d.Imported || d.ComposeDir == "" {
		return nil, fmt.Errorf("no compose file for this deployment")
	}

	detail := &ComposeDetail{
		DeploymentID:   d.ID,
		DeploymentName: d.Name,
		ComposeDir:     d.ComposeDir,
	}

	// Read compose YAML
	yamlPath := filepath.Join(d.ComposeDir, "docker-compose.yml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		detail.ComposeYAML = string(data)
	}

	// Get live service states via docker compose ps
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format",
		"table {{.Name}}\t{{.Image}}\t{{.State}}\t{{.Ports}}\t{{.CreatedAt}}")
	cmd.Dir = d.ComposeDir
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for i, line := range lines {
			if i == 0 || line == "" {
				continue // skip header
			}
			parts := strings.Split(line, "\t")
			svc := ComposeServiceInfo{}
			if len(parts) > 0 {
				svc.Name = strings.TrimSpace(parts[0])
			}
			if len(parts) > 1 {
				svc.Image = strings.TrimSpace(parts[1])
			}
			if len(parts) > 2 {
				svc.State = strings.TrimSpace(parts[2])
			}
			if len(parts) > 3 {
				svc.Ports = strings.TrimSpace(parts[3])
			}
			if len(parts) > 4 {
				svc.Created = strings.TrimSpace(parts[4])
			}
			detail.Services = append(detail.Services, svc)
		}
	}

	return detail, nil
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
			_ = e.SyncImportedStatus(ctx)
			deployments, err := e.db.ListDeployments()
			if err != nil {
				continue
			}
			for _, d := range deployments {
				if !d.Imported {
					_ = e.SyncStatus(ctx, d.ID)
				}
			}
		}
	}
}

func stepRun(ctx context.Context, run *operations.Run, key, title string, fn func(*operations.Step) error) error {
	if run == nil {
		return fn(nil)
	}
	return run.Step(ctx, key, title, func(stepCtx context.Context, step *operations.Step) error {
		_ = stepCtx
		return fn(step)
	})
}

func stepWriter(step *operations.Step) io.Writer {
	if step == nil {
		return nil
	}
	return step.Writer()
}

func (e *Engine) removeDeploymentSite(siteName, domain string) error {
	if siteName == "" {
		return nil
	}
	if err := e.nginx.DisableSite(siteName); err != nil {
		logger.Debugf("disable nginx site %s returned: %v", siteName, err)
	}
	if err := e.nginx.DeleteSite(siteName); err != nil {
		return err
	}
	if domain != "" {
		if out, ok := e.nginx.TestConfig(); !ok {
			return fmt.Errorf("nginx config test failed after deleting site: %s", strings.TrimSpace(out))
		}
	}
	if err := e.nginx.Reload(); err != nil {
		return err
	}
	return nil
}
