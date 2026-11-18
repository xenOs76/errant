package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/generator"
)

func newSSHConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh-config",
		Short: "Display or regenerate the OpenSSH configuration for all nodes",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			destFile := filepath.Join(filepath.Dir(cfgPath), ".errant", "ssh_config")
			if err := generator.GenerateSSHConfig(cluster, destFile); err != nil {
				return err
			}

			content, err := os.ReadFile(destFile)
			if err != nil {
				return err
			}

			fmt.Print(string(content))

			return nil
		},
	}
}
