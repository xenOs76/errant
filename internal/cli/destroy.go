package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
	"github.com/xenos76/errant/internal/snapshot"
)

func newDestroyCmd() *cobra.Command {
	var (
		forceDestroy bool
		autoYes      bool
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy all virtual machines and infrastructure for the topology",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			projectDir := filepath.Dir(cfgPath)
			runner := runnerFactory(projectDir, cluster.Project)

			printDestructionPlan(ctx, cmd.OutOrStdout(), cluster, runner)

			if !forceDestroy && !autoYes {
				confirmed, err := confirmDestruction(cmd.InOrStdin(), cmd.OutOrStdout(), cluster)
				if err != nil {
					return err
				}

				if !confirmed {
					cancelledMsg := lipgloss.NewStyle().
						Foreground(colorAmber).
						Render("Destruction cancelled. No resources were modified.")
					fmt.Fprintln(cmd.OutOrStdout(), formatStep("Errant", cancelledMsg))

					return nil
				}
			}

			destroyingMsg := fmt.Sprintf("Destroying topology infrastructure: %s...", cluster.Topology)
			fmt.Fprintln(cmd.OutOrStdout(), formatStep("Errant", destroyingMsg))

			if err := cleanupClusterSnapshots(ctx, projectDir, cluster); err != nil {
				return err
			}

			if err := runner.Destroy(ctx, cluster); err != nil {
				return err
			}

			successMsg := lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true).
				Render("Topology successfully destroyed.")
			fmt.Fprintln(cmd.OutOrStdout(), formatStep("Errant", successMsg))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&forceDestroy, "force", "f", false, "Force destroy without interactive confirmation")
	cmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Automatic yes to confirmation prompt")

	return cmd
}

func cleanupClusterSnapshots(ctx context.Context, projectDir string, cluster *config.ResolvedCluster) error {
	if !snapshot.StateExists(projectDir) && len(cluster.Steps) == 0 {
		return nil
	}

	mgr := snapshotManagerFactory()

	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		if err := mgr.CleanupAll(ctx, domName, snapshot.SnapshotPrefix); err != nil {
			return err
		}

		if err := mgr.CleanupAll(ctx, domName, snapshot.UserSnapshotPrefix); err != nil {
			return err
		}
	}

	return snapshot.RemoveState(projectDir)
}

func printDestructionPlan(
	ctx context.Context,
	out io.Writer,
	cluster *config.ResolvedCluster,
	runner EngineRunner,
) {
	styledTopo := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cluster.Topology)
	styledProject := lipgloss.NewStyle().Foreground(colorCyan).Render(cluster.Project)
	targetMsg := fmt.Sprintf("Target topology: %s (project: %s)", styledTopo, styledProject)

	fmt.Fprintf(out, "%s\n\n", formatStep("Errant", targetMsg))
	fmt.Fprintf(out, "%s\n\n", formatWarningHeader("The following resources are targeted for destruction:"))

	vmHeader := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("  Virtual Machines (%d):", len(cluster.Nodes)))
	fmt.Fprintln(out, vmHeader)

	for _, node := range cluster.Nodes {
		printNodeDetails(ctx, out, cluster, node, runner)
	}

	netHeader := lipgloss.NewStyle().Bold(true).Render("\n  Network:")
	fmt.Fprintln(out, netHeader)

	styledBridge := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cluster.Network.Bridge)
	fmt.Fprintf(out, "    - Bridge: %s (subnet: %s, mode: %s)\n\n",
		styledBridge, cluster.Network.Subnet, cluster.Network.Mode)
}

func printNodeDetails(
	ctx context.Context,
	out io.Writer,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	runner EngineRunner,
) {
	domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
	rawStatus := runner.GetDomainStatus(ctx, domName)
	status := formatDomainStatus(rawStatus)
	styledName := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(node.Name)

	fmt.Fprintf(out, "    - %s (status: %s, ip: %s, role: %s, %dc/%dMB)\n",
		styledName, status, node.IP, node.Role, node.CPUs, node.MemoryMB)
	fmt.Fprintf(out, "      • Domain: %s\n", domName)
	fmt.Fprintf(out, "      • Root Disk: %s (pool: default)\n", node.RootDiskSize)
	fmt.Fprintf(out, "      • Cloud-Init ISO: %s-cloudinit.iso\n", domName)

	for _, disk := range node.Disks {
		fmt.Fprintf(out, "      • Extra Disk (%s): %s (format: %s)\n",
			disk.Device, disk.Size, disk.Format)
	}
}

func confirmDestruction(in io.Reader, out io.Writer, cluster *config.ResolvedCluster) (bool, error) {
	promptMsg := fmt.Sprintf("Are you sure you want to destroy topology %q (%d node(s))?",
		cluster.Topology, len(cluster.Nodes))
	fmt.Fprint(out, formatPrompt(promptMsg, "[y/N]"))

	reader := bufio.NewReader(in)

	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading confirmation input: %w", err)
	}

	response := strings.ToLower(strings.TrimSpace(line))

	return response == "y" || response == "yes", nil
}
