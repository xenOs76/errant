package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
)

func completeNodeNames(args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cluster, _, err := loadResolvedCluster()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string

	for _, n := range cluster.Nodes {
		if strings.HasPrefix(n.Name, toComplete) {
			desc := n.Role
			if n.IP != "" {
				desc += fmt.Sprintf(" (%s)", n.IP)
			}

			completions = append(completions, fmt.Sprintf("%s\t%s", n.Name, desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

func hasNode(cluster *config.ResolvedCluster, nodeName string) bool {
	for _, n := range cluster.Nodes {
		if n.Name == nodeName {
			return true
		}
	}

	return false
}

func runSSH(args []string) error {
	nodeName := args[0]
	extraArgs := args[1:]

	cluster, cfgPath, err := loadResolvedCluster()
	if err != nil {
		return err
	}

	if !hasNode(cluster, nodeName) {
		return fmt.Errorf("node %q not found in current topology %q", nodeName, cluster.Topology)
	}

	sshConfigFile := filepath.Join(filepath.Dir(cfgPath), ".errant", "ssh_config")
	if _, err := os.Stat(sshConfigFile); err != nil {
		return fmt.Errorf("ssh_config not found at %s; please run 'errant up' first", sshConfigFile)
	}

	sshArgs := []string{"-F", sshConfigFile, nodeName}
	sshArgs = append(sshArgs, extraArgs...)

	sshCmd := execSSHCommand("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	return sshCmd.Run()
}

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <node-name> [-- <extra-ssh-args...>]",
		Short: "Connect interactively to a virtual machine via SSH",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(
			_ *cobra.Command, args []string, toComplete string,
		) ([]string, cobra.ShellCompDirective) {
			return completeNodeNames(args, toComplete)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSSH(args)
		},
	}
}
