package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/deployment"
	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/proxy"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
	"github.com/gin-gonic/gin"
)

// Start initializes and runs the Vessel HTTP server.
func Start(cfg *config.Config, version string) error {
	logger.Infof("Starting Vessel v%s...", version)
	logger.Infof("Opening SQLite database at: %s", cfg.DBPath)

	// Open database
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Errorf("failed to open SQLite database: %v", err)
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	logger.Infof("Running database schema migrations...")
	if err := db.Migrate(); err != nil {
		logger.Errorf("database migrations failed: %v", err)
		return fmt.Errorf("migrate database: %w", err)
	}
	logger.Infof("Database schema is fully up-to-date")

	// Initialize registry
	logger.Infof("Loading app templates catalog...")
	reg := registry.New()
	if os.Getenv("VESSEL_TEMPLATE_CATALOG_DISABLED") != "1" {
		catalogURL := os.Getenv("VESSEL_TEMPLATE_CATALOG_URL")
		logger.Infof("Loading remote templates catalog from URL: %s", catalogURL)
		if err := reg.LoadFromRemote(catalogURL); err != nil {
			logger.Errorf("failed to load remote templates catalog: %v", err)
			fmt.Printf("warning: failed to load remote templates: %v\n", err)
		}
	}
	logger.Infof("Loading custom templates from data templates directory: %s", cfg.TemplatesDir)
	if err := reg.LoadFromDir(cfg.TemplatesDir); err != nil {
		logger.Errorf("failed to load custom templates from directory: %v", err)
		fmt.Printf("warning: failed to load custom templates: %v\n", err)
	}

	// Initialize proxy manager
	logger.Infof("Setting up reverse proxy (Caddy)...")
	prx := proxy.NewManager(cfg.CaddyDir)
	if err := prx.EnsureMainConfig(); err != nil {
		logger.Errorf("failed to configure Caddy reverse proxy: %v", err)
		fmt.Printf("warning: caddy config setup failed: %v\n", err)
	}

	// Initialize deployment engine
	logger.Infof("Initializing docker compose deployment engine...")
	engine := deployment.NewEngine(cfg, db, reg, prx)

	// Start background status sync
	logger.Infof("Starting periodic background container sync status...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go engine.PeriodicSync(ctx, 30*time.Second)

	// Set up router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if logger.IsDebug() {
		logger.Infof("Configuring Gin request logger middleware")
		r.Use(gin.LoggerWithWriter(logger.GetWriter()))
	}
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Register routes
	logger.Infof("Registering HTTP endpoints under /api/v1")
	api := r.Group("/api/v1")
	registerRoutes(api, db, reg, engine, version)

	// Serve embedded UI
	r.NoRoute(serveUI())

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // 0 = no timeout (needed for log streaming)
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Infof("Termination signal received. Gracefully shutting down Vessel...")
		fmt.Println("\nShutting down Vessel...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
		logger.Infof("Server shutdown completed cleanly")
	}()

	fmt.Printf("🚢 Vessel running at http://localhost%s\n", addr)
	logger.Infof("Vessel HTTP server successfully listening at http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("ListenAndServe server error: %v", err)
		return err
	}
	return nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if sameOrigin(origin, c.Request.Host) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			} else {
				c.AbortWithStatus(403)
				return
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func sameOrigin(origin, host string) bool {
	return origin == "http://"+host || origin == "https://"+host
}
