package cli

import (
	"fmt"

	"github.com/Yash121l/Vessel/internal/backup"
	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/nginx"
	"github.com/spf13/cobra"
)

var backupOutput string

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a full Vessel backup archive",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		mgr := backup.NewManager(cfg, nginx.NewManager().ConfigRoot())
		dest := backupOutput
		if dest == "" {
			dest = mgr.DefaultArchivePath()
		}
		if _, err := mgr.CreateArchive(dest); err != nil {
			return err
		}
		fmt.Printf("Backup written to %s\n", dest)
		return nil
	},
}

func init() {
	backupCmd.Flags().StringVar(&backupOutput, "output", "", "Output archive path (.tar.gz)")
}
