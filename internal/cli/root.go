package cli

import (
	"fmt"
	"os"

	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/server"
	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "vessel",
	Short: "Vessel — lightweight self-hosted app deployment manager",
	Long: `Vessel is a lightweight, self-hosted deployment manager for Linux VPS.
Deploy and manage popular self-hosted applications with minimal DevOps knowledge.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Vessel server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		logger.Init(debug, cfg.DataDir)
		defer logger.Close()
		return server.Start(cfg, Version)
	},
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap the system (install Docker, nginx, configure firewall)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		logger.Init(debug, cfg.DataDir)
		defer logger.Close()
		return runBootstrap(cfg)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Vessel %s\n", Version)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging to file")
}

func Execute() {
	rootCmd.AddCommand(serveCmd, bootstrapCmd, versionCmd, updateCmd, backupCmd, restoreCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
