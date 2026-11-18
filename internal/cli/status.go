package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/snapshot"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Display status of all virtual machines in the active topology",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			projectDir := filepath.Dir(cfgPath)
			runner := runnerFactory(projectDir, cluster.Project)

			headerCluster := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cluster.Project)
			headerTopo := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cluster.Topology)

			stepSuffix := ""

			if snapshot.StateExists(projectDir) {
				if st, err := snapshot.LoadState(projectDir); err == nil {
					if st.CurrentKind == snapshot.StepKindUser {
						stepSuffix = " | Step: user:" + st.CurrentStepName
					} else {
						stepSuffix = fmt.Sprintf(" | Step: %d (%s)", st.CurrentStep, st.CurrentStepName)
					}
				}
			}

			fmt.Printf("Cluster: %s | Topology: %s%s\n\n", headerCluster, headerTopo, stepSuffix)
			fmt.Printf("%-22s %-16s %-14s %-12s %s\n", "NAME", "IP", "ROLE", "STATUS", "CPUS/RAM")
			fmt.Printf("%-22s %-16s %-14s %-12s %s\n", "----", "--", "----", "------", "--------")

			for _, node := range cluster.Nodes {
				domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
				rawState := runner.GetDomainStatus(ctx, domName)
				nameCol := padStyled(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(node.Name), 22)
				statusCol := padStyled(formatDomainStatus(rawState), 12)
				sizing := fmt.Sprintf("%dc/%dMB", node.CPUs, node.MemoryMB)

				fmt.Printf("%s %-16s %-14s %s %s\n", nameCol, node.IP, node.Role, statusCol, sizing)
			}

			fmt.Println()

			return nil
		},
	}
}
