package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Yash121l/Vessel/internal/deployment"
	"github.com/Yash121l/Vessel/internal/docker"
	"github.com/Yash121l/Vessel/internal/nginx"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
)

func registerRoutes(
	r *gin.RouterGroup,
	db *store.DB,
	reg *registry.Registry,
	engine *deployment.Engine,
) {
	// Apps (templates)
	r.GET("/apps", listApps(reg))
	r.GET("/apps/:id", getApp(reg))

	// Deployments
	r.GET("/deployments", listDeployments(db))
	r.POST("/deployments", createDeployment(engine))
	r.GET("/deployments/:id", getDeployment(db))
	r.DELETE("/deployments/:id", removeDeployment(engine))

	// Deployment actions
	r.POST("/deployments/:id/start", startDeployment(engine))
	r.POST("/deployments/:id/stop", stopDeployment(engine))
	r.POST("/deployments/:id/restart", restartDeployment(engine))
	r.POST("/deployments/:id/update", updateDeployment(engine))

	// Logs (SSE streaming)
	r.GET("/deployments/:id/logs", streamLogs(engine))
	r.GET("/deployments/:id/compose", getComposeDetail(engine))

	// Docker discovery
	r.GET("/docker/containers", listDockerContainers())
	r.POST("/docker/import", importContainer(db))
	r.POST("/docker/deploy", deployCustomContainer(engine))
	r.GET("/docker/search", dockerHubSearch())
	r.GET("/docker/containers/:id/logs", streamContainerLogs())
	r.POST("/docker/containers/:id/stop", stopContainer())
	r.POST("/docker/containers/:id/start", startContainer())
	r.POST("/docker/containers/:id/restart", restartContainer())

	// Settings
	r.GET("/settings", getSettings(db))
	r.PUT("/settings", updateSettings(db))

	// Nginx management
	ngx := nginx.NewManager()
	r.GET("/nginx/status", nginxStatus(ngx))
	r.POST("/nginx/reload", nginxReload(ngx))
	r.POST("/nginx/restart", nginxRestart(ngx))
	r.POST("/nginx/stop", nginxStop(ngx))
	r.POST("/nginx/start", nginxStart(ngx))
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

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": "0.1.0"})
	})
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

func listDeployments(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deployments, err := db.ListDeployments()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if deployments == nil {
			deployments = []*store.Deployment{}
		}
		c.JSON(200, gin.H{"deployments": deployments})
	}
}

func getDeployment(db *store.DB) gin.HandlerFunc {
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
		c.JSON(200, d)
	}
}

type createDeploymentRequest struct {
	AppID  string            `json:"app_id" binding:"required"`
	Name   string            `json:"name" binding:"required"`
	Domain string            `json:"domain"`
	Env    map[string]string `json:"env"`
}

func createDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createDeploymentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()

		d, err := engine.Deploy(ctx, deployment.DeployRequest{
			AppID:  req.AppID,
			Name:   req.Name,
			Domain: req.Domain,
			Env:    req.Env,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, d)
	}
}

func removeDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()
		if err := engine.Remove(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "removed"})
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
		c.JSON(200, detail)
	}
}

// --- Actions ---

func startDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()
		if err := engine.Start(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "started"})
	}
}

func stopDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()
		if err := engine.Stop(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "stopped"})
	}
}

func restartDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()
		if err := engine.Restart(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "restarted"})
	}
}

func updateDeployment(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()
		if err := engine.Update(ctx, c.Param("id")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "updated"})
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

func importContainer(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req importContainerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		containers, err := docker.ListContainers(ctx)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
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
			c.JSON(404, gin.H{"error": "container not found"})
			return
		}

		name := req.Name
		if name == "" {
			name = found.Name
		}

		// Check not already imported
		existing, _ := db.GetDeploymentByName(name)
		if existing != nil {
			c.JSON(409, gin.H{"error": "already imported as: " + name})
			return
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
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, d)
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
func deployCustomContainer(engine *deployment.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Image  string            `json:"image"  binding:"required"`
			Name   string            `json:"name"   binding:"required"`
			Domain string            `json:"domain"`
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

		// Register the synthetic template temporarily
		engine.RegisterTemp(tmpl)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()

		d, err := engine.Deploy(ctx, deployment.DeployRequest{
			AppID:  req.Name,
			Name:   req.Name,
			Domain: req.Domain,
			Env:    req.Env,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, d)
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
		url := "https://hub.docker.com/v2/search/repositories/?query=" + q + "&page_size=10"
		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()
		httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
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

// isVesselContainer returns true if the given container ID/name belongs to Vessel itself.
func isVesselContainer(nameOrID string) bool {
	lower := strings.ToLower(nameOrID)
	return strings.Contains(lower, "vessel")
}

// guessAppID maps a container image/name to a known Vessel app template ID.
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

func nginxReload(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if out, ok := ngx.TestConfig(); !ok {
			c.JSON(400, gin.H{"error": "config test failed", "output": out})
			return
		}
		if err := ngx.Reload(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "reloaded"})
	}
}

func nginxRestart(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ngx.Restart(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "restarted"})
	}
}

func nginxStop(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ngx.Stop(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "stopped"})
	}
}

func nginxStart(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ngx.Start(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "started"})
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
		if err := ngx.CreateSite(req.Name, req.ServerName, req.Port, req.Upstream); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, gin.H{"status": "created", "name": req.Name})
	}
}

func nginxEnableSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ngx.EnableSite(c.Param("name")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "enabled"})
	}
}

func nginxDisableSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ngx.DisableSite(c.Param("name")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "disabled"})
	}
}

func nginxDeleteSite(ngx *nginx.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
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

func nginxGetLogs(ngx *nginx.Manager) gin.HandlerFunc {	return func(c *gin.Context) {
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
