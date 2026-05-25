package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vessel-app/vessel/internal/config"
	"github.com/vessel-app/vessel/internal/deployment"
	"github.com/vessel-app/vessel/internal/registry"
	"github.com/vessel-app/vessel/internal/store"
)

func registerRoutes(
	r *gin.RouterGroup,
	db *store.DB,
	reg *registry.Registry,
	engine *deployment.Engine,
	cfg *config.Config,
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

	// Settings
	r.GET("/settings", getSettings(db))
	r.PUT("/settings", updateSettings(db))

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

type settingsResponse struct {
	Port    int    `json:"port"`
	DataDir string `json:"data_dir"`
}

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
		// In production, the UI is embedded via go:embed.
		// During development, this falls through to the API.
		if c.Request.URL.Path == "/" || c.Request.URL.Path == "" {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, uiHTML)
			return
		}
		c.Status(404)
	}
}
