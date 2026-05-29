package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yash121l/Vessel/internal/backup"
	"github.com/Yash121l/Vessel/internal/deployment"
	"github.com/Yash121l/Vessel/internal/docker"
	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/nginx"
	"github.com/Yash121l/Vessel/internal/operations"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	dnsLookupIP         = net.LookupIP
	primaryIPv4Detector = detectAdvertisedIPv4
	primaryIPv4Once     sync.Once
	primaryIPv4Value    string
	metadataHTTPClient  = &http.Client{Timeout: 250 * time.Millisecond}
	nonPublicIPv4Nets   = mustParseCIDRs(
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
	)
)

func registerRoutes(
	r *gin.RouterGroup,
	db *store.DB,
	reg *registry.Registry,
	engine *deployment.Engine,
	ops *operations.Manager,
	backupMgr *backup.Manager,
	version string,
) {
	// Public setup/auth endpoints
	r.GET("/setup", setupStatus(db))
	r.POST("/setup", setupAdmin(db))
	r.POST("/login", login(db))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": version})
	})

	r.Use(authRequired(db))
	r.Use(requirePermission())
	r.GET("/me", me(version))
	r.POST("/logout", logout(db))
	r.GET("/operations", listOperations(db))
	r.GET("/operations/:id", getOperation(db))

	// Vessel app users (login accounts)
	r.GET("/users", listUsers(db))
	r.POST("/users", createUser(db))
	r.PUT("/users/:id", updateUser(db))
	r.DELETE("/users/:id", deleteUser(db))

	// OS-level user management (Linux accounts)
	r.GET("/system/users", listOSUsers())
	r.GET("/system/users/:username", getOSUser())
	r.POST("/system/users", createOSUser())
	r.PUT("/system/users/:username", updateOSUser())
	r.DELETE("/system/users/:username", deleteOSUser())
	r.GET("/system/groups", listOSGroups())

	// Apps (templates)
	r.GET("/apps", listApps(reg))
	r.GET("/apps/:id", getApp(reg))

	// Deployments
	r.GET("/deployments", listDeployments(db, reg))
	r.POST("/deployments", createDeployment(engine, reg, ops))
	r.GET("/deployments/:id", getDeployment(db, reg))
	r.DELETE("/deployments/:id", removeDeployment(db, engine, ops))

	// Deployment actions
	r.POST("/deployments/:id/start", startDeployment(db, engine, ops))
	r.POST("/deployments/:id/stop", stopDeployment(db, engine, ops))
	r.POST("/deployments/:id/restart", restartDeployment(db, engine, ops))
	r.POST("/deployments/:id/update", updateDeployment(db, engine, ops))

	// Logs (SSE streaming)
	r.GET("/deployments/:id/logs", streamLogs(engine))
	r.GET("/deployments/:id/compose", getComposeDetail(engine))

	// Multi-service compose stack deployment
	r.POST("/deployments/compose", createComposeDeployment(engine, reg, ops))

	// Docker discovery
	r.GET("/docker/containers", listDockerContainers())
	r.GET("/docker/compose/stacks", listComposeStacks())
	r.DELETE("/docker/compose/stacks/:name", removeComposeStack())
	r.POST("/docker/import", importContainer(db, ops))
	r.POST("/docker/deploy", deployCustomContainer(engine, ops))
	r.GET("/docker/search", dockerHubSearch())
	r.GET("/docker/containers/:id/logs", streamContainerLogs())
	r.POST("/docker/containers/:id/stop", stopContainer())
	r.POST("/docker/containers/:id/start", startContainer())
	r.POST("/docker/containers/:id/restart", restartContainer())

	// Settings
	r.GET("/settings", getSettings(db))
	r.PUT("/settings", updateSettings(db))

	// Self-update
	r.GET("/system/update", selfUpdate(ops))
	r.GET("/system/backups", listBackups(backupMgr))
	r.POST("/system/backups", createBackup(backupMgr, ops))

	// System info
	r.GET("/system/ip", systemIP())
	r.GET("/system/dns", systemDNS())

	// Nginx management
	ngx := nginx.NewManager()
	r.GET("/nginx/status", nginxStatus(ngx))
	r.POST("/nginx/reload", nginxReload(ngx, ops))
	r.POST("/nginx/restart", nginxRestart(ngx, ops))
	r.POST("/nginx/stop", nginxStop(ngx, ops))
	r.POST("/nginx/start", nginxStart(ngx, ops))
	r.GET("/nginx/test", nginxTest(ngx))
	r.GET("/nginx/config", nginxGetMainConfig(ngx))
	r.PUT("/nginx/config", nginxSaveMainConfig(ngx))
	r.GET("/nginx/sites", nginxListSites(ngx))
	r.GET("/nginx/sites/:name", nginxGetSite(ngx))
	r.PUT("/nginx/sites/:name", nginxSaveSite(ngx))
	r.POST("/nginx/sites", nginxCreateSite(ngx))
	r.POST("/nginx/sites/:name/enable", nginxEnableSite(ngx))
	r.POST("/nginx/sites/:name/disable", nginxDisableSite(ngx))
	r.DELETE("/nginx/sites/:name", nginxDeleteSite(ngx))
	r.GET("/nginx/logs/:type", nginxGetLogs(ngx))
	r.GET("/nginx/logs/:type/stream", nginxStreamLogs(ngx))
	r.GET("/nginx/stats", nginxGetStats(ngx))

}

func listOperations(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		if user == nil || !roleAtLeast(user.Role, roleOperator) {
			c.JSON(403, gin.H{"error": "insufficient permissions"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		ops, err := db.ListOperations(limit, strings.TrimSpace(c.Query("resource_type")), strings.TrimSpace(c.Query("resource_id")))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if ops == nil {
			ops = []*store.Operation{}
		}
		ops = filterVisibleOperations(user, ops)
		c.JSON(200, gin.H{"operations": ops})
	}
}

func getOperation(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		if user == nil || !roleAtLeast(user.Role, roleOperator) {
			c.JSON(403, gin.H{"error": "insufficient permissions"})
			return
		}
		op, err := db.GetOperation(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if op == nil {
			c.JSON(404, gin.H{"error": "operation not found"})
			return
		}
		if !canViewOperation(user, op) {
			c.JSON(403, gin.H{"error": "insufficient permissions"})
			return
		}
		c.JSON(200, gin.H{"operation": op})
	}
}

func filterVisibleOperations(user *store.User, ops []*store.Operation) []*store.Operation {
	if user == nil {
		return []*store.Operation{}
	}
	if roleAtLeast(user.Role, roleAdmin) {
		return ops
	}
	out := make([]*store.Operation, 0, len(ops))
	for _, op := range ops {
		if canViewOperation(user, op) {
			out = append(out, op)
		}
	}
	return out
}

func canViewOperation(user *store.User, op *store.Operation) bool {
	if user == nil || op == nil {
		return false
	}
	if roleAtLeast(user.Role, roleAdmin) {
		return true
	}
	if !roleAtLeast(user.Role, roleOperator) {
		return false
	}
	return op.ResourceType == "deployment"
}

func listBackups(backupMgr *backup.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		archives, err := backupMgr.ListArchives()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"backups": archives})
	}
}

func createBackup(backupMgr *backup.Manager, ops *operations.Manager) gin.HandlerFunc {
	type request struct {
		OutputPath string `json:"output_path"`
	}
	return func(c *gin.Context) {
		var req request
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
		}
		outputPath := strings.TrimSpace(req.OutputPath)
		if outputPath == "" {
			outputPath = backupMgr.DefaultArchivePath()
		}
		op, err := ops.Start(operationSpec(c, "create_backup", "backup", filepath.Base(outputPath), map[string]any{
			"output_path": outputPath,
		}), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "archive_runtime", "Archive runtime state", func(ctx context.Context, step *operations.Step) error {
				_ = ctx
				step.Logf("Writing backup archive to %s", outputPath)
				if _, err := backupMgr.CreateArchive(outputPath); err != nil {
					return err
				}
				run.BindResource("backup", outputPath, filepath.Base(outputPath))
				run.SetSummary(fmt.Sprintf("Backup created at %s", outputPath))
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

// --- Apps ---

func listApps(reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"apps": reg.List()})
	}
}

func getApp(reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		tmpl, ok := reg.Get(c.Param("id"))
		if !ok {
			c.JSON(404, gin.H{"error": "app not found"})
			return
		}
		c.JSON(200, tmpl)
	}
}

// --- Deployments ---

func listDeployments(db *store.DB, reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		deployments, err := db.ListDeployments()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if deployments == nil {
			deployments = []*store.Deployment{}
		}
		c.JSON(200, gin.H{"deployments": deploymentListResponse(reg, deployments)})
	}
}

func getDeployment(db *store.DB, reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if d == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		c.JSON(200, deploymentResponse(reg, d))
	}
}

func systemDNS() gin.HandlerFunc {
	type dnsResponse struct {
		Domain          string   `json:"domain"`
		IPs             []string `json:"ips"`
		ExpectedIP      string   `json:"expected_ip"`
		Resolved        bool     `json:"resolved"`
		MatchesExpected bool     `json:"matches_expected"`
		Error           string   `json:"error,omitempty"`
	}
	return func(c *gin.Context) {
		domain := strings.TrimSpace(strings.ToLower(c.Query("domain")))
		resp := dnsResponse{Domain: domain, IPs: []string{}, ExpectedIP: getPrimaryIPv4()}
		if domain == "" {
			resp.Error = "domain is required"
			c.JSON(400, resp)
			return
		}
		if err := validateDomain(domain); err != nil {
			resp.Error = err.Error()
			c.JSON(400, resp)
			return
		}
		status := domainDNSStatus(domain)
		resp.IPs = status.IPs
		resp.ExpectedIP = status.ExpectedIP
		resp.Resolved = status.Resolved
		resp.MatchesExpected = status.MatchesExpected
		resp.Error = status.Error
		c.JSON(200, resp)
	}
}

func validateDomainDNSReady(domain string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil
	}
	status := domainDNSStatus(domain)
	if status.Resolved {
		return nil
	}
	if status.Error != "" {
		return fmt.Errorf("custom domain DNS is not ready yet: %s", status.Error)
	}
	return fmt.Errorf("custom domain DNS is not ready yet: add an A record for %s and wait for propagation", domain)
}

type createDeploymentRequest struct {
	AppID  string            `json:"app_id" binding:"required"`
	Name   string            `json:"name" binding:"required"`
	Domain string            `json:"domain"`
	Env    map[string]string `json:"env"`
	// SkipServices lists optional sidecar service names to omit.
	// Use this when you want to provide your own external database/cache/etc.
	// Example: ["n8n-db"] to skip the managed Postgres and supply DATABASE_URL yourself.
	SkipServices []string `json:"skip_services"`
}

func createDeployment(engine *deployment.Engine, reg *registry.Registry, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createDeploymentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDeploymentName(req.Name); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomain(req.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomainDNSReady(req.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateEnv(req.Env); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Validate that any skipped services are actually optional in the template
		if len(req.SkipServices) > 0 {
			tmpl, ok := reg.Get(req.AppID)
			if !ok {
				c.JSON(400, gin.H{"error": "unknown app: " + req.AppID})
				return
			}
			optionalNames := map[string]bool{}
			for _, svc := range tmpl.ExtraServices {
				if svc.Optional {
					optionalNames[svc.Name] = true
				}
			}
			for _, name := range req.SkipServices {
				if !optionalNames[name] {
					c.JSON(400, gin.H{"error": "service '" + name + "' is not optional and cannot be skipped"})
					return
				}
			}
		}

		skipSet := make(map[string]bool, len(req.SkipServices))
		for _, s := range req.SkipServices {
			skipSet[s] = true
		}

		op, err := ops.Start(operationSpec(c, "deploy", "deployment", req.Name, map[string]any{
			"app_id":        req.AppID,
			"domain":        req.Domain,
			"skip_services": req.SkipServices,
		}), func(ctx context.Context, run *operations.Run) error {
			_, runErr := engine.DeployWithRun(ctx, deployment.DeployRequest{
				AppID:        req.AppID,
				Name:         req.Name,
				Domain:       req.Domain,
				Env:          req.Env,
				SkipServices: skipSet,
			}, run)
			return runErr
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func removeDeployment(db *store.DB, engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		depl, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if depl == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		op, err := ops.Start(operationSpec(c, "remove_deployment", "deployment", depl.Name, map[string]any{
			"deployment_id": depl.ID,
		}), func(ctx context.Context, run *operations.Run) error {
			return engine.RemoveWithRun(ctx, depl.ID, run)
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func getComposeDetail(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		detail, err := engine.GetComposeDetail(ctx, c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		detail.ComposeYAML = redactComposeYAML(detail.ComposeYAML)
		c.JSON(200, detail)
	}
}

// --- Actions ---

func startDeployment(db *store.DB, engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		depl, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if depl == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		op, err := ops.Start(operationSpec(c, "start_deployment", "deployment", depl.Name, map[string]any{
			"deployment_id": depl.ID,
		}), func(ctx context.Context, run *operations.Run) error {
			return engine.StartWithRun(ctx, depl.ID, run)
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func stopDeployment(db *store.DB, engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		depl, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if depl == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		op, err := ops.Start(operationSpec(c, "stop_deployment", "deployment", depl.Name, map[string]any{
			"deployment_id": depl.ID,
		}), func(ctx context.Context, run *operations.Run) error {
			return engine.StopWithRun(ctx, depl.ID, run)
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func restartDeployment(db *store.DB, engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		depl, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if depl == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		op, err := ops.Start(operationSpec(c, "restart_deployment", "deployment", depl.Name, map[string]any{
			"deployment_id": depl.ID,
		}), func(ctx context.Context, run *operations.Run) error {
			return engine.RestartWithRun(ctx, depl.ID, run)
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func updateDeployment(db *store.DB, engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		depl, err := db.GetDeployment(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if depl == nil {
			c.JSON(404, gin.H{"error": "deployment not found"})
			return
		}
		op, err := ops.Start(operationSpec(c, "update_deployment", "deployment", depl.Name, map[string]any{
			"deployment_id": depl.ID,
		}), func(ctx context.Context, run *operations.Run) error {
			return engine.UpdateWithRun(ctx, depl.ID, run)
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

// --- Logs (Server-Sent Events) ---

func streamLogs(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		lines := make(chan string, 100)
		go func() {
			defer close(lines)
			_ = engine.StreamLogs(ctx, id, lines)
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-lines:
				if !ok {
					return false
				}
				fmt.Fprintf(w, "data: %s\n\n", line)
				return true
			case <-ctx.Done():
				return false
			}
		})
	}
}

// --- Settings ---

func getSettings(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Return non-sensitive settings
		c.JSON(200, gin.H{
			"status": "ok",
		})
	}
}

type updateSettingsRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
}

func updateSettings(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := db.SetSetting(req.Key, req.Value); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "updated"})
	}
}

// serveUI returns a handler that serves the embedded web UI.
func serveUI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/" || c.Request.URL.Path == "" {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, uiHTML)
			return
		}
		c.Status(404)
	}
}

// --- Docker discovery ---

func listDockerContainers() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		containers, err := docker.ListContainers(ctx)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if containers == nil {
			containers = []docker.Container{}
		}
		c.JSON(200, gin.H{"containers": containers})
	}
}

type importContainerRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
	Name        string `json:"name"`
}

func importContainer(db *store.DB, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req importContainerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if req.Name != "" {
			if err := validateDeploymentName(req.Name); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
		}

		op, err := ops.Start(operationSpec(c, "import_container", "deployment", req.Name, map[string]any{
			"container_id": req.ContainerID,
			"name":         req.Name,
		}), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "discover_container", "Inspect Docker container", func(ctx context.Context, step *operations.Step) error {
				containers, err := docker.ListContainers(ctx)
				if err != nil {
					return err
				}
				var found *docker.Container
				for i := range containers {
					if containers[i].ID == req.ContainerID ||
						strings.HasPrefix(containers[i].ID, req.ContainerID) ||
						containers[i].Name == req.ContainerID {
						found = &containers[i]
						break
					}
				}
				if found == nil {
					return fmt.Errorf("container not found")
				}

				name := req.Name
				if name == "" {
					name = found.Name
				}
				existing, _ := db.GetDeploymentByName(name)
				if existing != nil {
					return fmt.Errorf("already imported as: %s", name)
				}

				status := "running"
				if found.State != "running" {
					status = "stopped"
				}
				d := &store.Deployment{
					ID:          uuid.New().String(),
					Name:        name,
					AppID:       guessAppID(found.Image, found.Name),
					Status:      status,
					ComposeDir:  "",
					Imported:    true,
					ContainerID: found.ID,
					Image:       found.Image,
					Ports:       strings.Join(found.Ports, ", "),
				}
				if err := db.CreateDeployment(d); err != nil {
					return err
				}
				run.BindResource("deployment", d.ID, d.Name)
				run.SetSummary(fmt.Sprintf("Imported container %s", d.Name))
				step.Logf("Imported Docker container %s (%s)", d.Name, d.ContainerID)
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func streamContainerLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		lines := make(chan string, 100)
		go func() {
			defer close(lines)
			_ = docker.ContainerLogs(ctx, id, lines)
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-lines:
				if !ok {
					return false
				}
				fmt.Fprintf(w, "data: %s\n\n", line)
				return true
			case <-ctx.Done():
				return false
			}
		})
	}
}

func stopContainer() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if isVesselContainer(id) {
			c.JSON(400, gin.H{"error": "cannot stop the Vessel container itself"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err := docker.StopContainer(ctx, id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "stopped"})
	}
}

func startContainer() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err := docker.StartContainer(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "started"})
	}
}

func restartContainer() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if isVesselContainer(id) {
			c.JSON(400, gin.H{"error": "cannot restart the Vessel container itself"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err := docker.RestartContainer(ctx, id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "restarted"})
	}
}

// deployCustomContainer deploys an arbitrary Docker image as a Vessel-managed deployment.
func deployCustomContainer(engine *deployment.Engine, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Image  string `json:"image"  binding:"required"`
			Name   string `json:"name"   binding:"required"`
			Domain string `json:"domain"`
			Ports  []struct {
				Internal int    `json:"internal"`
				External int    `json:"external"`
				Protocol string `json:"protocol"`
			} `json:"ports"`
			Volumes []struct {
				Name      string `json:"name"`
				MountPath string `json:"mount_path"`
			} `json:"volumes"`
			Env map[string]string `json:"env"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateImageRef(req.Image); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDeploymentName(req.Name); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomain(req.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomainDNSReady(req.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateEnv(req.Env); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		for _, p := range req.Ports {
			if err := validatePort(p.Internal, "container port"); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			if p.External != 0 {
				if err := validatePort(p.External, "host port"); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
			}
			if p.Protocol != "" && p.Protocol != "tcp" && p.Protocol != "udp" {
				c.JSON(400, gin.H{"error": "protocol must be tcp or udp"})
				return
			}
		}

		// Build a synthetic AppTemplate from the request
		tmpl := &registry.AppTemplate{
			ID:    req.Name,
			Name:  req.Name,
			Image: req.Image,
		}
		for _, p := range req.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			tmpl.Ports = append(tmpl.Ports, registry.Port{
				Internal: p.Internal,
				External: p.External,
				Protocol: proto,
			})
			if tmpl.ProxyPort == 0 {
				tmpl.ProxyPort = p.Internal
			}
		}
		for _, v := range req.Volumes {
			tmpl.Volumes = append(tmpl.Volumes, registry.Volume{
				Name:      v.Name,
				MountPath: v.MountPath,
			})
		}

		op, err := ops.Start(operationSpec(c, "deploy_custom_container", "deployment", req.Name, map[string]any{
			"image":  req.Image,
			"domain": req.Domain,
		}), func(ctx context.Context, run *operations.Run) error {
			_, runErr := engine.DeployTemplateWithRun(ctx, tmpl, deployment.DeployRequest{
				AppID:  req.Name,
				Name:   req.Name,
				Domain: req.Domain,
				Env:    req.Env,
			}, run)
			return runErr
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

// dockerHubSearch proxies a Docker Hub search query.
func dockerHubSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(400, gin.H{"error": "q is required"})
			return
		}
		searchURL := "https://hub.docker.com/v2/search/repositories/?query=" + url.QueryEscape(q) + "&page_size=10"
		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()
		httpReq, _ := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		httpReq.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			c.JSON(502, gin.H{"error": "docker hub unreachable"})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", body)
	}
}

// listComposeStacks runs `docker compose ls` and returns all stacks on the host.
func listComposeStacks() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// --format json gives a JSON array
		cmd := exec.CommandContext(ctx, "docker", "compose", "ls", "--all", "--format", "json")
		out, err := cmd.Output()
		if err != nil {
			// fallback: return empty list rather than error
			c.JSON(200, gin.H{"stacks": []interface{}{}})
			return
		}

		// docker compose ls --format json outputs a JSON array directly
		var raw []map[string]interface{}
		if jsonErr := json.Unmarshal(out, &raw); jsonErr != nil {
			c.JSON(200, gin.H{"stacks": []interface{}{}})
			return
		}

		// Normalise field names (docker uses Title case: Name, Status, ConfigFiles)
		type Stack struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			ConfigFiles string `json:"config_files"`
		}
		stacks := make([]Stack, 0, len(raw))
		for _, r := range raw {
			s := Stack{}
			if v, ok := r["Name"].(string); ok {
				s.Name = v
			}
			if v, ok := r["Status"].(string); ok {
				s.Status = v
			}
			if v, ok := r["ConfigFiles"].(string); ok {
				s.ConfigFiles = v
			}
			stacks = append(stacks, s)
		}
		c.JSON(200, gin.H{"stacks": stacks})
	}
}

// isVesselContainer returns true if the given container ID/name belongs to Vessel itself.
func isVesselContainer(nameOrID string) bool {
	lower := strings.ToLower(nameOrID)
	return strings.Contains(lower, "vessel")
}

// removeComposeStack tears down an external (non-Vessel-managed) compose stack by name.
// It uses `docker compose --project-name <name> down` which works even when the
// original compose file no longer exists, because Docker tracks the stack by project name.
func removeComposeStack() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			c.JSON(400, gin.H{"error": "stack name is required"})
			return
		}
		// Safety: don't allow removing Vessel-managed stacks via this endpoint
		if isVesselContainer(name) {
			c.JSON(400, gin.H{"error": "cannot remove a Vessel-managed stack via this endpoint"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
		defer cancel()

		// --remove-orphans cleans up containers whose service definition was removed
		cmd := exec.CommandContext(ctx, "docker", "compose",
			"--project-name", name,
			"down", "--remove-orphans")
		out, err := cmd.CombinedOutput()
		if err != nil {
			c.JSON(500, gin.H{"error": "docker compose down failed: " + strings.TrimSpace(string(out))})
			return
		}
		c.JSON(200, gin.H{"status": "removed", "name": name})
	}
}

// selfUpdate streams the self-update progress as SSE.
// Strategy: run `vessel update --no-restart` to download and replace the binary,
// stream all output, send __DONE__, flush, then restart the service in a
// goroutine with a short delay so the response has time to reach the client.
func selfUpdate(ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		flusher, _ := c.Writer.(http.Flusher)
		writeSSE := func(line string) {
			fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			if flusher != nil {
				flusher.Flush()
			}
		}

		spec := operationSpec(c, "self_update", "system", "vessel", nil)
		op, run, err := ops.Begin(spec)
		if err != nil {
			writeSSE("ERROR: " + err.Error())
			return
		}
		run.BindResource("system", "vessel", "vessel")
		writeSSE("Tracking operation " + op.ID)

		exe, err := os.Executable()
		if err != nil {
			exe = "/usr/local/bin/vessel"
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()

		// --no-restart: download and replace binary only; we handle the restart
		// ourselves after flushing __DONE__ to the client.
		var args []string
		if logger.IsDebug() {
			args = []string{"update", "--no-restart", "--debug"}
		} else {
			args = []string{"update", "--no-restart"}
		}
		updateErr := run.Step(ctx, "update_binary", "Download and replace binary", func(ctx context.Context, step *operations.Step) error {
			cmd := exec.CommandContext(ctx, exe, args...)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {
				return err
			}

			lines := make(chan string, 64)
			var scanWG sync.WaitGroup
			scan := func(r io.Reader) {
				defer scanWG.Done()
				buf := bufio.NewScanner(r)
				for buf.Scan() {
					lines <- buf.Text()
				}
			}
			scanWG.Add(2)
			go scan(stdout)
			go scan(stderr)

			done := make(chan error, 1)
			go func() {
				waitErr := cmd.Wait()
				scanWG.Wait()
				close(lines)
				done <- waitErr
			}()

			for {
				select {
				case line, ok := <-lines:
					if !ok {
						lines = nil
						continue
					}
					step.Logf("%s", line)
					writeSSE(line)
				case err := <-done:
					return err
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
		if finishErr := ops.Finish(run, spec, updateErr); finishErr != nil && updateErr == nil {
			updateErr = finishErr
		}
		if updateErr != nil {
			writeSSE("ERROR: " + updateErr.Error())
			return
		}

		writeSSE("Restarting service…")
		writeSSE("__DONE__")

		// Restart the service after a short delay so the SSE response has time
		// to be flushed and received by the client before the server dies.
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = exec.Command("systemctl", "restart", "vessel").Run()
		}()
	}
}

// The caller provides a primary service plus zero or more sidecar services,
// and Vessel generates, writes, and starts the compose file.
func createComposeDeployment(engine *deployment.Engine, reg *registry.Registry, ops *operations.Manager) gin.HandlerFunc {
	type svcReq struct {
		Name  string `json:"name"         binding:"required"`
		Image string `json:"image"        binding:"required"`
		Ports []struct {
			Internal int    `json:"internal"`
			External int    `json:"external"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
		Volumes []struct {
			Name      string `json:"name"`
			MountPath string `json:"mount_path"`
		} `json:"volumes"`
		Environment map[string]string `json:"environment"`
		HealthCheck *struct {
			Test     []string `json:"test"`
			Interval string   `json:"interval"`
			Timeout  string   `json:"timeout"`
			Retries  int      `json:"retries"`
		} `json:"health_check"`
	}
	type req struct {
		Name     string            `json:"name"     binding:"required"`
		Domain   string            `json:"domain"`
		Env      map[string]string `json:"env"`
		Primary  svcReq            `json:"primary"  binding:"required"`
		Sidecars []svcReq          `json:"sidecars"`
	}
	return func(c *gin.Context) {
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDeploymentName(r.Name); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomain(r.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomainDNSReady(r.Domain); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateImageRef(r.Primary.Image); err != nil {
			c.JSON(400, gin.H{"error": "primary image: " + err.Error()})
			return
		}

		// Build a synthetic AppTemplate from the request
		tmpl := &registry.AppTemplate{
			ID:    r.Name,
			Name:  r.Name,
			Image: r.Primary.Image,
		}
		for _, p := range r.Primary.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			tmpl.Ports = append(tmpl.Ports, registry.Port{
				Internal: p.Internal,
				External: p.External,
				Protocol: proto,
			})
			if tmpl.ProxyPort == 0 {
				tmpl.ProxyPort = p.Internal
			}
		}
		for _, v := range r.Primary.Volumes {
			tmpl.Volumes = append(tmpl.Volumes, registry.Volume{
				Name:      v.Name,
				MountPath: v.MountPath,
			})
		}
		if r.Primary.HealthCheck != nil && len(r.Primary.HealthCheck.Test) > 0 {
			tmpl.HealthCheck = registry.HealthCheck{
				Test:     r.Primary.HealthCheck.Test,
				Interval: r.Primary.HealthCheck.Interval,
				Timeout:  r.Primary.HealthCheck.Timeout,
				Retries:  r.Primary.HealthCheck.Retries,
			}
		}

		// Build sidecar service definitions
		for _, s := range r.Sidecars {
			if err := validateImageRef(s.Image); err != nil {
				c.JSON(400, gin.H{"error": "sidecar " + s.Name + " image: " + err.Error()})
				return
			}
			sdef := registry.ServiceDef{
				Name:        s.Name,
				Image:       s.Image,
				Environment: s.Environment,
				Optional:    false,
				Role:        "sidecar",
			}
			for _, v := range s.Volumes {
				sdef.Volumes = append(sdef.Volumes, registry.Volume{
					Name:      v.Name,
					MountPath: v.MountPath,
				})
			}
			for _, p := range s.Ports {
				proto := p.Protocol
				if proto == "" {
					proto = "tcp"
				}
				sdef.Ports = append(sdef.Ports, registry.Port{
					Internal: p.Internal,
					External: p.External,
					Protocol: proto,
				})
			}
			if s.HealthCheck != nil && len(s.HealthCheck.Test) > 0 {
				sdef.HealthCheck = registry.HealthCheck{
					Test:     s.HealthCheck.Test,
					Interval: s.HealthCheck.Interval,
					Timeout:  s.HealthCheck.Timeout,
					Retries:  s.HealthCheck.Retries,
				}
			}
			tmpl.ExtraServices = append(tmpl.ExtraServices, sdef)
		}

		// Merge primary env vars into the env map
		env := r.Env
		if env == nil {
			env = make(map[string]string)
		}
		for k, v := range r.Primary.Environment {
			if _, exists := env[k]; !exists {
				env[k] = v
			}
		}

		op, err := ops.Start(operationSpec(c, "deploy_compose_stack", "deployment", r.Name, map[string]any{
			"domain":   r.Domain,
			"sidecars": len(r.Sidecars),
		}), func(ctx context.Context, run *operations.Run) error {
			_, runErr := engine.DeployTemplateWithRun(ctx, tmpl, deployment.DeployRequest{
				AppID:  r.Name,
				Name:   r.Name,
				Domain: r.Domain,
				Env:    env,
			}, run)
			return runErr
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func operationSpec(c *gin.Context, kind, resourceType, resourceName string, metadata map[string]any) operations.Spec {
	spec := operations.Spec{
		Kind:         kind,
		ResourceType: resourceType,
		ResourceName: resourceName,
		Metadata:     metadata,
	}
	if user := currentUser(c); user != nil {
		spec.ActorUserID = user.ID
		spec.ActorUsername = user.Username
	}
	switch kind {
	case "deploy", "deploy_custom_container", "deploy_compose_stack", "update_deployment":
		spec.Timeout = 15 * time.Minute
	case "remove_deployment":
		spec.Timeout = 5 * time.Minute
	default:
		spec.Timeout = 3 * time.Minute
	}
	return spec
}
func guessAppID(image, name string) string {
	s := strings.ToLower(image + " " + name)
	switch {
	case strings.Contains(s, "metabase"):
		return "metabase"
	case strings.Contains(s, "n8n"):
		return "n8n"
	case strings.Contains(s, "umami"):
		return "umami"
	case strings.Contains(s, "plausible"):
		return "plausible"
	case strings.Contains(s, "open-webui") || strings.Contains(s, "openwebui"):
		return "open-webui"
	case strings.Contains(s, "plane"):
		return "plane"
	case strings.Contains(s, "mysql") || strings.Contains(s, "mariadb"):
		return "mysql"
	case strings.Contains(s, "postgres"):
		return "postgres"
	case strings.Contains(s, "redis"):
		return "redis"
	case strings.Contains(s, "mongo"):
		return "mongodb"
	case strings.Contains(s, "nginx"):
		return "nginx"
	default:
		return "custom"
	}
}

// --- Nginx handlers ---

func nginxStatus(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, ngx.GetStatus())
	}
}

func nginxReload(ngx *nginx.Manager, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		op, err := ops.Start(operationSpec(c, "nginx_reload", "system", "nginx", nil), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "reload_nginx", "Reload nginx", func(_ context.Context, step *operations.Step) error {
				if out, ok := ngx.TestConfig(); !ok {
					step.Logf("nginx config test failed: %s", strings.TrimSpace(out))
					return fmt.Errorf("config test failed: %s", strings.TrimSpace(out))
				}
				if err := ngx.Reload(); err != nil {
					return err
				}
				run.SetSummary("Nginx configuration reloaded")
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func nginxRestart(ngx *nginx.Manager, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		op, err := ops.Start(operationSpec(c, "nginx_restart", "system", "nginx", nil), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "restart_nginx", "Restart nginx", func(_ context.Context, step *operations.Step) error {
				_ = step
				if err := ngx.Restart(); err != nil {
					return err
				}
				run.SetSummary("Nginx restarted")
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func nginxStop(ngx *nginx.Manager, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		op, err := ops.Start(operationSpec(c, "nginx_stop", "system", "nginx", nil), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "stop_nginx", "Stop nginx", func(_ context.Context, step *operations.Step) error {
				_ = step
				if err := ngx.Stop(); err != nil {
					return err
				}
				run.SetSummary("Nginx stopped")
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func nginxStart(ngx *nginx.Manager, ops *operations.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		op, err := ops.Start(operationSpec(c, "nginx_start", "system", "nginx", nil), func(ctx context.Context, run *operations.Run) error {
			return run.Step(ctx, "start_nginx", "Start nginx", func(_ context.Context, step *operations.Step) error {
				_ = step
				if err := ngx.Start(); err != nil {
					return err
				}
				run.SetSummary("Nginx started")
				return nil
			})
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(202, gin.H{"operation": op})
	}
}

func nginxTest(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		out, ok := ngx.TestConfig()
		c.JSON(200, gin.H{"ok": ok, "output": out})
	}
}

func nginxGetMainConfig(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		content, err := ngx.GetMainConfig()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"content": content})
	}
}

func nginxSaveMainConfig(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.SaveMainConfig(req.Content); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "saved"})
	}
}

func nginxListSites(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sites, err := ngx.ListSites()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if sites == nil {
			sites = []nginx.SiteFile{}
		}
		c.JSON(200, gin.H{"sites": sites})
	}
}

func nginxGetSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := validateSiteFileName(c.Param("name")); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		site, err := ngx.GetSite(c.Param("name"))
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, site)
	}
}

func nginxSaveSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := validateSiteFileName(c.Param("name")); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var req struct {
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.SaveSite(c.Param("name"), req.Content); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "saved"})
	}
}

func nginxCreateSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name       string `json:"name"        binding:"required"`
			ServerName string `json:"server_name" binding:"required"`
			Port       int    `json:"port"`
			Upstream   string `json:"upstream"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if req.Port == 0 {
			req.Port = 80
		}
		if err := validateSiteFileName(req.Name); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateDomain(req.ServerName); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validatePort(req.Port, "listen port"); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateUpstream(req.Upstream); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.CreateSite(req.Name, req.ServerName, req.Port, req.Upstream); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, gin.H{"status": "created", "name": req.Name})
	}
}

func nginxEnableSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := validateSiteFileName(c.Param("name")); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.EnableSite(c.Param("name")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "enabled"})
	}
}

func nginxDisableSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := validateSiteFileName(c.Param("name")); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.DisableSite(c.Param("name")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "disabled"})
	}
}

func nginxDeleteSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := validateSiteFileName(c.Param("name")); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := ngx.DeleteSite(c.Param("name")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "deleted"})
	}
}

func nginxGetStats(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		lines := 5000
		c.JSON(200, ngx.GetStats(lines))
	}
}

func nginxGetLogs(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		n := 200
		lines, err := ngx.TailLog(c.Param("type"), n)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"lines": lines})
	}
}

func nginxStreamLogs(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		logType := c.Param("type")
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		lines := make(chan string, 100)
		go func() {
			defer close(lines)
			_ = ngx.StreamLog(ctx, logType, lines)
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-lines:
				if !ok {
					return false
				}
				fmt.Fprintf(w, "data: %s\n\n", line)
				return true
			case <-ctx.Done():
				return false
			}
		})
	}
}

// --- System info ---

func systemIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getPrimaryIPv4()
		c.JSON(200, gin.H{"ip": ip})
	}
}

type dnsStatus struct {
	IPs             []string
	ExpectedIP      string
	Resolved        bool
	MatchesExpected bool
	Error           string
}

func domainDNSStatus(domain string) dnsStatus {
	status := dnsStatus{
		IPs:        []string{},
		ExpectedIP: getPrimaryIPv4(),
	}
	ips, err := dnsLookupIP(domain)
	if err != nil {
		status.Error = "DNS record not found yet"
		return status
	}
	seen := map[string]bool{}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			s := v4.String()
			if !seen[s] {
				status.IPs = append(status.IPs, s)
				seen[s] = true
			}
		}
	}
	if len(status.IPs) == 0 {
		status.Error = fmt.Sprintf("%s has no A record yet", domain)
		return status
	}
	if status.ExpectedIP == "" {
		status.Error = "unable to determine the server public IPv4 address automatically; set VESSEL_PUBLIC_IP on the server and try again"
		return status
	}
	status.MatchesExpected = seen[status.ExpectedIP]
	if !status.MatchesExpected {
		status.Error = fmt.Sprintf("DNS resolves to %s, expected %s", strings.Join(status.IPs, ", "), status.ExpectedIP)
		return status
	}
	status.Resolved = true
	return status
}

// getPrimaryIPv4 returns the public IPv4 address that Vessel should advertise
// for custom-domain DNS configuration, or an empty string if it cannot be
// determined.
func getPrimaryIPv4() string {
	primaryIPv4Once.Do(func() {
		primaryIPv4Value = primaryIPv4Detector()
	})
	return primaryIPv4Value
}

func detectAdvertisedIPv4() string {
	if ip := normalizeIPv4(os.Getenv("VESSEL_PUBLIC_IP")); ip != "" {
		return ip
	}
	if ip := firstPublicInterfaceIPv4(); ip != "" {
		return ip
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, detector := range []func(context.Context) string{
		detectAWSPublicIPv4,
		detectGCPPublicIPv4,
		detectAzurePublicIPv4,
		detectDigitalOceanPublicIPv4,
	} {
		if ip := detector(ctx); ip != "" {
			return ip
		}
	}
	return ""
}

func firstPublicInterfaceIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				if !isPublicIPv4(ip4) {
					continue
				}
				return ip4.String()
			}
		}
	}
	return ""
}

func isPublicIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil || !ip.IsGlobalUnicast() {
		return false
	}
	for _, block := range nonPublicIPv4Nets {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func normalizeIPv4(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
		return ip4.String()
	}
	return ""
}

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		out = append(out, network)
	}
	return out
}

func metadataText(ctx context.Context, method, endpoint string, headers map[string]string) string {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return ""
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := metadataHTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return ""
	}
	return normalizeIPv4(string(body))
}

func detectAWSPublicIPv4(ctx context.Context) string {
	const tokenURL = "http://169.254.169.254/latest/api/token"
	const metadataURL = "http://169.254.169.254/latest/meta-data/public-ipv4"
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPut, tokenURL, nil)
	if err == nil {
		tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
		if resp, err := metadataHTTPClient.Do(tokenReq); err == nil {
			tokenBody, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if ip := metadataText(ctx, http.MethodGet, metadataURL, map[string]string{
					"X-aws-ec2-metadata-token": strings.TrimSpace(string(tokenBody)),
				}); ip != "" {
					return ip
				}
			}
		}
	}
	return metadataText(ctx, http.MethodGet, metadataURL, nil)
}

func detectGCPPublicIPv4(ctx context.Context) string {
	return metadataText(ctx, http.MethodGet, "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip", map[string]string{
		"Metadata-Flavor": "Google",
	})
}

func detectAzurePublicIPv4(ctx context.Context) string {
	return metadataText(ctx, http.MethodGet, "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text", map[string]string{
		"Metadata": "true",
	})
}

func detectDigitalOceanPublicIPv4(ctx context.Context) string {
	return metadataText(ctx, http.MethodGet, "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address", nil)
}
