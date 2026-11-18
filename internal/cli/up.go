package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
	"github.com/xenos76/errant/internal/generator"
)

var (
	provisionFlag   bool
	noProvisionFlag bool
	upStepFlag      string
	noStepsFlag     bool
)

func newUpCmd() *cobra.Command {
	provisionFlag = false
	noProvisionFlag = false
	upStepFlag = ""
	noStepsFlag = false

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and boot virtual machines for the active topology",
		RunE: func(_ *cobra.Command, _ []string) error {
			defer func() {
				provisionFlag = false
				noProvisionFlag = false
				upStepFlag = ""
				noStepsFlag = false
			}()

			if provisionFlag && noProvisionFlag {
				return errors.New("cannot specify both --provision and --no-provision")
			}

			if upStepFlag != "" && noStepsFlag {
				return errors.New("cannot specify both --step and --no-steps")
			}

			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			return executeUpWithOpts(ctx, cluster, cfgPath, upOptions{
				ForceProvision: provisionFlag,
				SkipProvision:  noProvisionFlag,
				TargetStep:     upStepFlag,
				SkipSteps:      noStepsFlag,
			})
		},
	}

	cmd.Flags().BoolVar(
		&provisionFlag,
		"provision",
		false,
		"Force provisioning execution even if VMs were already created",
	)
	cmd.Flags().BoolVar(&noProvisionFlag, "no-provision", false, "Skip provisioning after booting VMs")
	cmd.Flags().StringVar(&upStepFlag, "step", "", "Advance lifecycle steps up to the given step index or name")
	cmd.Flags().BoolVar(&noStepsFlag, "no-steps", false, "Skip declared lifecycle steps and snapshot creation")

	return cmd
}

type upProvisionPlan struct {
	Force           bool
	Skip            bool
	AllNodesExisted bool
}

func (p upProvisionPlan) ShouldProvision() bool {
	if p.Force {
		return true
	}

	if p.Skip {
		return false
	}

	return !p.AllNodesExisted
}

type upOptions struct {
	ForceProvision bool
	SkipProvision  bool
	TargetStep     string
	SkipSteps      bool
}

func executeUp(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	cfgPath string,
	forceProvision, skipProvision bool,
) error {
	return executeUpWithOpts(ctx, cluster, cfgPath, upOptions{
		ForceProvision: forceProvision,
		SkipProvision:  skipProvision,
	})
}

func executeUpWithOpts(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	cfgPath string,
	opts upOptions,
) error {
	projectDir := filepath.Dir(cfgPath)
	sshConfigFile := filepath.Join(projectDir, ".errant", "ssh_config")
	inventoryFile := filepath.Join(projectDir, ".errant", "inventory.ini")

	styledTopo := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cluster.Topology)
	styledProject := lipgloss.NewStyle().Foreground(colorCyan).Render(cluster.Project)
	fmt.Println(formatStep("Errant", fmt.Sprintf("Deploying topology: %s (project: %s)", styledTopo, styledProject)))

	if err := generator.GenerateSSHConfig(cluster, sshConfigFile); err != nil {
		return fmt.Errorf("failed generating SSH config: %w", err)
	}

	if err := generator.GenerateAnsibleInventory(cluster, inventoryFile); err != nil {
		return fmt.Errorf("failed generating Ansible inventory: %w", err)
	}

	runner := runnerFactory(projectDir, cluster.Project)
	allNodesExisted := checkAllNodesExisted(ctx, runner, cluster)

	fmt.Println(formatStep("Errant", "Applying topology infrastructure (Pulumi)..."))

	if err := runner.Apply(ctx, cluster); err != nil {
		return err
	}

	if err := runner.Start(ctx, cluster); err != nil {
		return err
	}

	plan := upProvisionPlan{
		Force:           opts.ForceProvision,
		Skip:            opts.SkipProvision,
		AllNodesExisted: allNodesExisted,
	}

	if err := handleProvisioning(ctx, cluster, sshConfigFile, plan); err != nil {
		return err
	}

	if !opts.SkipSteps {
		if err := handleUpSteps(ctx, cluster, sshConfigFile, opts.TargetStep); err != nil {
			return err
		}
	}

	printClusterSummary(cluster, sshConfigFile, inventoryFile)

	return nil
}

func handleUpSteps(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	targetStep string,
) error {
	if len(cluster.Steps) == 0 && targetStep == "" {
		return nil
	}

	targetIdx := len(cluster.Steps)
	if targetStep != "" {
		idx, _, err := resolveDeclarativeStepRef(cluster, targetStep)
		if err != nil {
			return err
		}

		targetIdx = idx
	}

	mgr := snapshotManagerFactory()

	return applyStepsThrough(ctx, os.Stdout, mgr, cluster, sshConfigFile, targetIdx)
}

func checkAllNodesExisted(ctx context.Context, runner EngineRunner, cluster *config.ResolvedCluster) bool {
	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		if runner.GetDomainStatus(ctx, domName) == "not created" {
			return false
		}
	}

	return true
}

func handleProvisioning(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	plan upProvisionPlan,
) error {
	if plan.ShouldProvision() {
		fmt.Println("\n" + formatStep("Errant", "Provisioning virtual machines..."))

		if err := provisionClusterFunc(ctx, cluster, sshConfigFile, ""); err != nil {
			return fmt.Errorf("provisioning failed: %w", err)
		}

		return nil
	}

	if plan.AllNodesExisted {
		skipMsg := lipgloss.NewStyle().Foreground(colorAmber).Render(
			"Skipping provisioning (VMs already created; use --provision or 'errant provision' to run provisioners)",
		)
		fmt.Println("\n" + formatStep("Errant", skipMsg))
	}

	return ensureNodesReady(ctx, cluster, sshConfigFile)
}

func ensureNodesReady(ctx context.Context, cluster *config.ResolvedCluster, sshConfigFile string) error {
	for _, node := range cluster.Nodes {
		fmt.Println(formatStep(node.Name, "Waiting for SSH availability..."))

		if err := waitForSSHFunc(ctx, sshConfigFile, node.Name, 90*time.Second); err != nil {
			return err
		}

		if err := mountSyncedFoldersFunc(ctx, sshConfigFile, node); err != nil {
			fmt.Printf("    * Warning mounting synced folders on %s: %v\n", node.Name, err)
		}
	}

	return nil
}

func printClusterSummary(cluster *config.ResolvedCluster, sshConfigFile, inventoryFile string) {
	successMsg := lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("Topology successfully deployed!")
	fmt.Println("\n" + formatStep("Errant", successMsg))
	fmt.Printf("    * SSH configuration : %s\n", sshConfigFile)
	fmt.Printf("    * Ansible inventory : %s\n\n", inventoryFile)

	sectionHeader := lipgloss.NewStyle().Bold(true).Render("Nodes deployed:")
	fmt.Println(sectionHeader)

	for _, node := range cluster.Nodes {
		styledNode := padStyled(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(node.Name), 20)
		fmt.Printf("  - %s (IP: %-15s, Role: %s)\n", styledNode, node.IP, node.Role)
	}

	if len(cluster.Nodes) > 0 {
		sshCmd := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("errant ssh " + cluster.Nodes[0].Name)
		fmt.Printf("\nConnect with: %s\n", sshCmd)
	}
}
