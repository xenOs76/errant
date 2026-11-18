package cli

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newHaltCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "halt",
		Short: "Gracefully shut down all running virtual machines in the topology",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			projectDir := filepath.Dir(cfgPath)
			runner := runnerFactory(projectDir, cluster.Project)

			return runner.Halt(ctx, cluster)
		},
	}
}
