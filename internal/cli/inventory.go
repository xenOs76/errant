package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/generator"
)

func newInventoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inventory",
		Short: "Display or regenerate the Ansible hosts.ini inventory",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			destFile := filepath.Join(filepath.Dir(cfgPath), ".errant", "inventory.ini")
			if err := generator.GenerateAnsibleInventory(cluster, destFile); err != nil {
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
