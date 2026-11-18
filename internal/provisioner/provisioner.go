package provisioner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xenos76/errant/internal/config"
)

var (
	execAnsibleCommand           = exec.CommandContext
	execSSHCommand               = exec.CommandContext
	defaultStdout      io.Writer = os.Stdout
	defaultStderr      io.Writer = os.Stderr
)

// WaitForSSH attempts to connect to a node via SSH until successful or context/timeout expires.
func WaitForSSH(ctx context.Context, sshConfigFile, nodeName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		cmd := execSSHCommand(
			ctx, "ssh", "-F", sshConfigFile,
			"-o", "BatchMode=yes",
			"-o", "StrictHostKeyChecking=no",
			"-o", "ConnectTimeout=3",
			nodeName, "true",
		)

		if err := cmd.Run(); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for SSH on node %s: %w", nodeName, ctx.Err())
		case <-ticker.C:
		}
	}
}

// MountSyncedFolders ensures all declared synced folders are mounted in the guest.
func MountSyncedFolders(ctx context.Context, sshConfigFile string, node config.ResolvedNode) error {
	for _, sf := range node.SyncedFolders {
		fmt.Println(FormatStep(node.Name, fmt.Sprintf("Mounting synced folder: %s -> %s", sf.Source, sf.Target)))

		mountCmd := fmt.Sprintf(
			"sudo mkdir -p %s && (mountpoint -q %s || sudo mount -t 9p -o trans=virtio,version=9p2000.L %s %s || true)",
			sf.Target, sf.Target, sf.Target, sf.Target,
		)

		cmd := execSSHCommand(ctx, "ssh", "-F", sshConfigFile, node.Name, mountCmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed mounting %s on %s: %s: %w", sf.Target, node.Name, string(out), err)
		}
	}

	return nil
}

// RunFileProvisioner transfers a host file to a guest destination and sets permissions.
func RunFileProvisioner(
	ctx context.Context,
	sshConfigFile, nodeName string,
	prov config.ProvisionConfig,
	projectDir string,
) error {
	srcPath := prov.Source
	if !filepath.IsAbs(srcPath) && projectDir != "" {
		srcPath = filepath.Join(projectDir, srcPath)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading provision source %s: %w", srcPath, err)
	}

	mode := prov.Mode
	if mode == "" {
		mode = "0644"
	}

	fmt.Println(FormatStep(nodeName, fmt.Sprintf("Provisioning file: %s -> %s", prov.Source, prov.Target)))

	targetDir := filepath.Dir(prov.Target)
	remoteCmd := fmt.Sprintf(
		"sudo mkdir -p %s && sudo tee %s > /dev/null && sudo chmod %s %s",
		targetDir, prov.Target, mode, prov.Target,
	)

	cmd := execSSHCommand(ctx, "ssh", "-F", sshConfigFile, nodeName, remoteCmd)
	cmd.Stdin = bytes.NewReader(data)

	stdoutWriter := NewNodeLineWriter(defaultStdout, nodeName, false)
	stderrWriter := NewNodeLineWriter(defaultStderr, nodeName, true)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	err = cmd.Run()
	_ = stdoutWriter.Flush()
	_ = stderrWriter.Flush()

	return err
}

// RunShellProvisioner executes a shell script or inline command over SSH.
func RunShellProvisioner(
	ctx context.Context,
	sshConfigFile, nodeName string,
	prov config.ProvisionConfig,
	projectDir string,
) error {
	var scriptContent string

	var label string

	switch {
	case prov.Script != "":
		scriptPath := prov.Script
		if !filepath.IsAbs(scriptPath) && projectDir != "" {
			scriptPath = filepath.Join(projectDir, scriptPath)
		}

		data, err := os.ReadFile(scriptPath)
		if err != nil {
			return fmt.Errorf("reading provision script %s: %w", scriptPath, err)
		}

		scriptContent = string(data)
		label = prov.Script
	case prov.Inline != "":
		scriptContent = prov.Inline
		label = "inline command"
	default:
		return nil
	}

	fmt.Println(FormatStep(nodeName, "Running shell provisioner: "+label))

	var envPairs []string
	for k, v := range prov.Env {
		envPairs = append(envPairs, fmt.Sprintf("%s=%q", k, v))
	}

	remoteCmd := "sudo -E"
	if len(envPairs) > 0 {
		remoteCmd += " " + strings.Join(envPairs, " ")
	}

	remoteCmd += " bash -s"

	cmd := execSSHCommand(ctx, "ssh", "-F", sshConfigFile, nodeName, remoteCmd)
	cmd.Stdin = strings.NewReader(scriptContent)

	stdoutWriter := NewNodeLineWriter(defaultStdout, nodeName, false)
	stderrWriter := NewNodeLineWriter(defaultStderr, nodeName, true)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	err := cmd.Run()
	_ = stdoutWriter.Flush()
	_ = stderrWriter.Flush()

	return err
}

// ProvisionNode runs global and node-specific provisioners for a single node.
func ProvisionNode(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	node config.ResolvedNode,
) error {
	fmt.Println(FormatStep(node.Name, "Waiting for SSH availability..."))

	if err := WaitForSSH(ctx, sshConfigFile, node.Name, 90*time.Second); err != nil {
		return err
	}

	if err := MountSyncedFolders(ctx, sshConfigFile, node); err != nil {
		fmt.Printf("    * Warning mounting synced folders on %s: %v\n", node.Name, err)
	}

	for _, gp := range cluster.GlobalProvision {
		if err := runSingleProvisioner(ctx, sshConfigFile, node.Name, gp, cluster.ProjectDir); err != nil {
			return fmt.Errorf("global provision failed on %s: %w", node.Name, err)
		}
	}

	for _, p := range node.Provision {
		if err := runSingleProvisioner(ctx, sshConfigFile, node.Name, p, cluster.ProjectDir); err != nil {
			return fmt.Errorf("provisioning step failed on %s: %w", node.Name, err)
		}
	}

	return nil
}

// RunAnsibleProvisioner executes an Ansible playbook against a targeted node or inventory.
func RunAnsibleProvisioner(
	ctx context.Context,
	sshConfigFile, nodeName string,
	prov config.ProvisionConfig,
	projectDir string,
) error {
	if prov.Playbook == "" {
		return errors.New("ansible provisioner requires 'playbook' parameter")
	}

	playbookPath := prov.Playbook
	if !filepath.IsAbs(playbookPath) && projectDir != "" {
		playbookPath = filepath.Join(projectDir, playbookPath)
	}

	if _, err := os.Stat(playbookPath); err != nil {
		return fmt.Errorf("reading playbook %s: %w", playbookPath, err)
	}

	inventoryPath := prov.Inventory
	if inventoryPath == "" && projectDir != "" {
		inventoryPath = filepath.Join(projectDir, ".errant", "inventory.ini")
	} else if !filepath.IsAbs(inventoryPath) && projectDir != "" {
		inventoryPath = filepath.Join(projectDir, inventoryPath)
	}

	fmt.Println(FormatStep(nodeName, "Running ansible provisioner: "+prov.Playbook))

	args := []string{
		"-i", inventoryPath,
		"--ssh-common-args=-F " + sshConfigFile,
	}

	if nodeName != "" {
		args = append(args, "--limit", nodeName)
	}

	for k, v := range prov.ExtraVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, playbookPath)

	cmd := execAnsibleCommand(ctx, "ansible-playbook", args...)
	cmd.Dir = projectDir

	stdoutWriter := NewNodeLineWriter(defaultStdout, nodeName, false)
	stderrWriter := NewNodeLineWriter(defaultStderr, nodeName, true)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	err := cmd.Run()
	_ = stdoutWriter.Flush()
	_ = stderrWriter.Flush()

	if err != nil {
		return fmt.Errorf("ansible-playbook failed: %w", err)
	}

	return nil
}

func runSingleProvisioner(
	ctx context.Context,
	sshConfigFile, nodeName string,
	prov config.ProvisionConfig,
	projectDir string,
) error {
	switch prov.Type {
	case "file":
		return RunFileProvisioner(ctx, sshConfigFile, nodeName, prov, projectDir)
	case "shell":
		return RunShellProvisioner(ctx, sshConfigFile, nodeName, prov, projectDir)
	case "ansible":
		return RunAnsibleProvisioner(ctx, sshConfigFile, nodeName, prov, projectDir)
	default:
		return fmt.Errorf("unsupported provisioner type: %s", prov.Type)
	}
}

// ProvisionCluster runs provisioners across all nodes or a targeted node.
func ProvisionCluster(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	targetNodeName string,
) error {
	for _, node := range cluster.Nodes {
		if targetNodeName != "" && node.Name != targetNodeName {
			continue
		}

		if err := ProvisionNode(ctx, cluster, sshConfigFile, node); err != nil {
			return err
		}
	}

	return nil
}

// StepTargetNodes returns the subset of cluster nodes targeted by a lifecycle step.
func StepTargetNodes(cluster *config.ResolvedCluster, step config.ResolvedStep) []config.ResolvedNode {
	if len(step.Nodes) == 0 && len(step.Roles) == 0 {
		return cluster.Nodes
	}

	nodeSet := make(map[string]struct{}, len(step.Nodes))
	for _, n := range step.Nodes {
		nodeSet[n] = struct{}{}
	}

	roleSet := make(map[string]struct{}, len(step.Roles))
	for _, r := range step.Roles {
		roleSet[r] = struct{}{}
	}

	var targets []config.ResolvedNode

	for _, node := range cluster.Nodes {
		_, matchNode := nodeSet[node.Name]
		_, matchRole := roleSet[node.Role]

		if matchNode || matchRole {
			targets = append(targets, node)
		}
	}

	return targets
}

// ProvisionStep executes a single lifecycle step's provisioners on all targeted nodes.
func ProvisionStep(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	step config.ResolvedStep,
) error {
	targets := StepTargetNodes(cluster, step)
	for _, node := range targets {
		fmt.Println(FormatStep(node.Name, fmt.Sprintf("Waiting for SSH for step %d (%s)...", step.Index, step.Name)))

		if err := WaitForSSH(ctx, sshConfigFile, node.Name, 90*time.Second); err != nil {
			return err
		}

		for _, p := range step.Provision {
			if err := runSingleProvisioner(ctx, sshConfigFile, node.Name, p, cluster.ProjectDir); err != nil {
				return fmt.Errorf("step %d (%s) failed on %s: %w", step.Index, step.Name, node.Name, err)
			}
		}
	}

	return nil
}
