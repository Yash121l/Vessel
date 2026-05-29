package cli

import (
	"fmt"

	"github.com/Yash121l/Vessel/internal/backup"
	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/nginx"
	"github.com/spf13/cobra"
)

var restoreForce bool

var restoreCmd = &cobra.Command{
	Use:   "restore <archive>",
	Short: "Restore a full Vessel backup archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		mgr := backup.NewManager(cfg, nginx.NewManager().ConfigRoot())
		if _, err := mgr.RestoreArchive(args[0], restoreForce); err != nil {
			return err
		}
		fmt.Printf("Restore completed from %s\n", args[0])
		return nil
	},
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "Overwrite existing files during restore")
}
