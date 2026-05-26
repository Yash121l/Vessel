package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/deployment"
	"github.com/Yash121l/Vessel/internal/proxy"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
)

// Start initializes and runs the Vessel HTTP server.
func Start(cfg *config.Config) error {
	// Open database
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Initialize registry
	reg := registry.New()
	if err := reg.LoadFromDir(cfg.TemplatesDir); err != nil {
		fmt.Printf("warning: failed to load custom templates: %v\n", err)
	}

	// Initialize proxy manager
	prx := proxy.NewManager(cfg.CaddyDir)
	if err := prx.EnsureMainConfig(); err != nil {
		fmt.Printf("warning: caddy config setup failed: %v\n", err)
	}

	// Initialize deployment engine
	engine := deployment.NewEngine(cfg, db, reg, prx)

	// Start background status sync
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go engine.PeriodicSync(ctx, 30*time.Second)

	// Set up router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Register routes
	api := r.Group("/api/v1")
	registerRoutes(api, db, reg, engine)

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
		fmt.Println("\nShutting down Vessel...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("🚢 Vessel running at http://localhost%s\n", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
