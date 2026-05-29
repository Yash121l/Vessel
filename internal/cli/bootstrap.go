package cli

import (
	"fmt"

	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/system"
)

func runBootstrap(cfg *config.Config) error {
	fmt.Println("🚀 Vessel Bootstrap")
	fmt.Println("===================")

	b := system.NewBootstrapper(cfg)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Detecting Linux distribution", b.DetectDistro},
		{"Checking system dependencies", b.CheckDependencies},
		{"Installing Docker", b.InstallDocker},
		{"Installing Docker Compose", b.InstallDockerCompose},
		{"Installing Nginx", b.InstallNginx},
		{"Installing Certbot Nginx plugin", b.InstallCertbotNginx},
		{"Configuring firewall", b.ConfigureFirewall},
		{"Setting up Vessel directories", b.SetupDirectories},
		{"Initializing database", b.InitDatabase},
	}

	for _, step := range steps {
		fmt.Printf("  → %s... ", step.name)
		if err := step.fn(); err != nil {
			fmt.Printf("✗\n    Error: %v\n", err)
			return fmt.Errorf("bootstrap failed at '%s': %w", step.name, err)
		}
		fmt.Println("✓")
	}

	fmt.Println()
	fmt.Println("✅ Bootstrap complete!")
	fmt.Printf("   Run 'vessel serve' to start the management UI\n")
	fmt.Printf("   Default address: http://localhost:%d\n", cfg.Port)
	return nil
}
