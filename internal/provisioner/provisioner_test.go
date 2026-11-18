package provisioner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/xenos76/errant/internal/config"
)

func TestWaitForSSHSuccess(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = orig }()

	ctx := context.Background()
	if err := WaitForSSH(ctx, "/fake/ssh_config", "test-node", 5*time.Second); err != nil {
		t.Fatalf("WaitForSSH failed: %v", err)
	}
}

func TestWaitForSSHTimeout(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("false")
	}

	defer func() { execSSHCommand = orig }()

	ctx := context.Background()

	err := WaitForSSH(ctx, "/fake/ssh_config", "test-node", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestMountSyncedFolders(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = orig }()

	node := config.ResolvedNode{
		Name: "test-node",
		SyncedFolders: []config.SyncedFolderConfig{
			{Source: ".", Target: "/errant", Type: "virtiofs"},
		},
	}

	ctx := context.Background()
	if err := MountSyncedFolders(ctx, "/fake/ssh_config", node); err != nil {
		t.Fatalf("MountSyncedFolders failed: %v", err)
	}
}

func TestRunFileProvisioner(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = orig }()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.sh")
	_ = os.WriteFile(srcFile, []byte("echo hello"), 0o755)

	prov := config.ProvisionConfig{
		Type:   "file",
		Source: "test.sh",
		Target: "/tmp/test.sh",
		Mode:   "0755",
	}

	ctx := context.Background()
	if err := RunFileProvisioner(ctx, "/fake/ssh_config", "test-node", prov, tmpDir); err != nil {
		t.Fatalf("RunFileProvisioner failed: %v", err)
	}

	// Test missing file
	prov.Source = "missing.sh"
	if err := RunFileProvisioner(ctx, "/fake/ssh_config", "test-node", prov, tmpDir); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestRunShellProvisioner(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = orig }()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "script.sh")
	_ = os.WriteFile(srcFile, []byte("echo hello"), 0o755)

	ctx := context.Background()

	// Script
	provScript := config.ProvisionConfig{
		Type:   "shell",
		Script: "script.sh",
		Env:    map[string]string{"FOO": "bar"},
	}
	if err := RunShellProvisioner(ctx, "/fake/ssh_config", "test-node", provScript, tmpDir); err != nil {
		t.Fatalf("RunShellProvisioner script failed: %v", err)
	}

	// Inline
	provInline := config.ProvisionConfig{
		Type:   "shell",
		Inline: "echo inline",
	}
	if err := RunShellProvisioner(ctx, "/fake/ssh_config", "test-node", provInline, tmpDir); err != nil {
		t.Fatalf("RunShellProvisioner inline failed: %v", err)
	}

	// Empty
	emptyProv := config.ProvisionConfig{Type: "shell"}
	if err := RunShellProvisioner(ctx, "/fake/ssh_config", "test-node", emptyProv, tmpDir); err != nil {
		t.Fatalf("RunShellProvisioner empty failed: %v", err)
	}

	// Missing script file
	provMissing := config.ProvisionConfig{
		Type:   "shell",
		Script: "missing.sh",
	}
	if err := RunShellProvisioner(ctx, "/fake/ssh_config", "test-node", provMissing, tmpDir); err == nil {
		t.Fatal("expected error for missing script, got nil")
	}
}

func TestProvisionCluster(t *testing.T) {
	orig := execSSHCommand
	execSSHCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = orig }()

	tmpDir := t.TempDir()
	cluster := &config.ResolvedCluster{
		Project:    "test-proj",
		ProjectDir: tmpDir,
		GlobalProvision: []config.ProvisionConfig{
			{Type: "shell", Inline: "echo global"},
		},
		Nodes: []config.ResolvedNode{
			{
				Name: "node-1",
				Provision: []config.ProvisionConfig{
					{Type: "shell", Inline: "echo node"},
				},
			},
		},
	}

	ctx := context.Background()
	if err := ProvisionCluster(ctx, cluster, "/fake/ssh_config", ""); err != nil {
		t.Fatalf("ProvisionCluster failed: %v", err)
	}

	// Target single node
	if err := ProvisionCluster(ctx, cluster, "/fake/ssh_config", "node-1"); err != nil {
		t.Fatalf("ProvisionCluster targeted node failed: %v", err)
	}

	// Unsupported type
	cluster.Nodes[0].Provision = []config.ProvisionConfig{{Type: "invalid"}}
	if err := ProvisionCluster(ctx, cluster, "/fake/ssh_config", ""); err == nil {
		t.Fatal("expected error for unsupported provisioner type, got nil")
	}
}

func TestRunAnsibleProvisioner(t *testing.T) {
	orig := execAnsibleCommand
	execAnsibleCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execAnsibleCommand = orig }()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook.yaml")

	if err := os.WriteFile(playbookFile, []byte("---\n- hosts: all\n"), 0o600); err != nil {
		t.Fatalf("failed creating test playbook: %v", err)
	}

	ctx := context.Background()

	// 1. Success with custom inventory and extra vars
	prov := config.ProvisionConfig{
		Type:      "ansible",
		Playbook:  "playbook.yaml",
		Inventory: "custom-inventory.ini",
		ExtraVars: map[string]string{"env": "staging"},
	}

	if err := RunAnsibleProvisioner(ctx, "/fake/ssh_config", "node-1", prov, tmpDir); err != nil {
		t.Fatalf("RunAnsibleProvisioner failed: %v", err)
	}

	// 2. Missing playbook error
	provMissingPlaybook := config.ProvisionConfig{Type: "ansible"}
	if err := RunAnsibleProvisioner(ctx, "/fake/ssh_config", "node-1", provMissingPlaybook, tmpDir); err == nil {
		t.Fatal("expected error for missing playbook parameter, got nil")
	}

	// 3. Nonexistent playbook file error
	provNonExistent := config.ProvisionConfig{Type: "ansible", Playbook: "nonexistent.yaml"}
	if err := RunAnsibleProvisioner(ctx, "/fake/ssh_config", "node-1", provNonExistent, tmpDir); err == nil {
		t.Fatal("expected error for nonexistent playbook file, got nil")
	}

	// 4. Execution failure
	execAnsibleCommand = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("false")
	}

	if err := RunAnsibleProvisioner(ctx, "/fake/ssh_config", "node-1", prov, tmpDir); err == nil {
		t.Fatal("expected error when ansible-playbook fails, got nil")
	}
}
