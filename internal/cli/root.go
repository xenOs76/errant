package cli

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
	"github.com/xenos76/errant/internal/provisioner"
	"github.com/xenos76/errant/internal/pulumi"
	"github.com/xenos76/errant/internal/snapshot"
)

// EngineRunner defines the hypervisor execution engine interface.
type EngineRunner interface {
	Apply(ctx context.Context, cluster *config.ResolvedCluster) error
	Destroy(ctx context.Context, cluster *config.ResolvedCluster) error
	Halt(ctx context.Context, cluster *config.ResolvedCluster) error
	Start(ctx context.Context, cluster *config.ResolvedCluster) error
	GetDomainStatus(ctx context.Context, domainName string) string
}

var (
	configFile   string
	topologyName string

	runnerFactory = func(workDir, projectName string) EngineRunner {
		return pulumi.NewRunner(workDir, projectName)
	}
	snapshotManagerFactory = func() snapshot.Manager {
		return snapshot.NewLibvirtManager()
	}
	execSSHCommand         = exec.Command
	provisionClusterFunc   = provisioner.ProvisionCluster
	provisionStepFunc      = provisioner.ProvisionStep
	waitForSSHFunc         = provisioner.WaitForSSH
	mountSyncedFoldersFunc = provisioner.MountSyncedFolders
)

// NewRootCmd constructs the primary errant command line interface.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "errant",
		Short: "Errant: modern, free-software local multi-node VM orchestrator",
		Long: `Errant is a lightweight, declarative local virtual machine orchestrator
designed as a modern alternative to HashiCorp Vagrant. It leverages Pulumi, Libvirt/KVM,
and Cloud-Init for fast, reproducible multi-node topologies.`,
	}

	rootCmd.PersistentFlags().StringVarP(
		&configFile, "config", "c", "", "Path to Errantfile.yaml",
	)
	rootCmd.PersistentFlags().StringVarP(
		&topologyName, "topology", "t", "", "Target cluster topology (e.g. standard, ha)",
	)

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newUpCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newSSHCmd())
	rootCmd.AddCommand(newProvisionCmd())
	rootCmd.AddCommand(newHaltCmd())
	rootCmd.AddCommand(newDestroyCmd())
	rootCmd.AddCommand(newSSHConfigCmd())
	rootCmd.AddCommand(newInventoryCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newImageCmd())
	rootCmd.AddCommand(newStepsCmd())

	return rootCmd
}

// Execute runs the root CLI command.
func Execute() error {
	return NewRootCmd().Execute()
}

func loadResolvedCluster() (*config.ResolvedCluster, string, error) {
	path, err := config.FindConfigFile(configFile)
	if err != nil {
		return nil, "", err
	}

	cfg, err := config.LoadFile(path)
	if err != nil {
		return nil, "", err
	}

	cluster, err := cfg.ResolveCluster(topologyName)
	if err != nil {
		return nil, "", err
	}

	cluster.ProjectDir = filepath.Dir(path)

	return cluster, path, nil
}
