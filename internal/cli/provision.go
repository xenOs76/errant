package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newProvisionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "provision [<node-name>]",
		Short: "Execute provisioners on running virtual machines",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(
			_ *cobra.Command, args []string, toComplete string,
		) ([]string, cobra.ShellCompDirective) {
			return completeNodeNames(args, toComplete)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			targetNode := ""
			if len(args) > 0 {
				targetNode = args[0]
				if !hasNode(cluster, targetNode) {
					return fmt.Errorf("node %q not found in current topology %q", targetNode, cluster.Topology)
				}
			}

			sshConfigFile := filepath.Join(filepath.Dir(cfgPath), ".errant", "ssh_config")
			if _, err := os.Stat(sshConfigFile); err != nil {
				return fmt.Errorf("ssh_config not found at %s; please run 'errant up' first", sshConfigFile)
			}

			return provisionClusterFunc(ctx, cluster, sshConfigFile, targetNode)
		},
	}
}
