package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestProvisionStepClusterScopeAnsible(t *testing.T) {
	tmpDir := t.TempDir()
	pbPath := filepath.Join(tmpDir, "site.yml")

	if err := os.WriteFile(pbPath, []byte("- hosts: all\n"), 0o600); err != nil {
		t.Fatalf("writing playbook: %v", err)
	}

	var calls [][]string

	origSSH := execSSHCommand
	execSSHCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	t.Cleanup(func() { execSSHCommand = origSSH })

	origAnsible := execAnsibleCommand
	execAnsibleCommand = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		calls = append(calls, args)

		return exec.CommandContext(ctx, "true")
	}

	t.Cleanup(func() { execAnsibleCommand = origAnsible })

	cluster := &config.ResolvedCluster{
		Project:    "k8s-homelab",
		ProjectDir: tmpDir,
		Nodes: []config.ResolvedNode{
			{Name: "cp-1", Role: "control-plane"},
			{Name: "w-1", Role: "worker"},
		},
	}

	step := config.ResolvedStep{
		Index: 1,
		Name:  "preprod-deploy",
		Provision: []config.ProvisionConfig{
			{Type: "ansible", Scope: "cluster", Playbook: "site.yml"},
		},
	}

	if err := ProvisionStep(context.Background(), cluster, "/fake/ssh_config", step); err != nil {
		t.Fatalf("ProvisionStep failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 cluster-wide ansible-playbook call, got %d (%v)", len(calls), calls)
	}

	for _, arg := range calls[0] {
		if arg == "--limit" {
			t.Errorf("did not expect --limit in cluster-scoped ansible call: %v", calls[0])
		}
	}
}

func TestRunLocalProvisioner(t *testing.T) {
	projectDir := t.TempDir()
	subDir := filepath.Join(projectDir, "external-repo")

	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	outFile := filepath.Join(projectDir, "out.txt")
	cmd := fmt.Sprintf(
		"pwd > %q && echo \"$MY_VAR\" >> %q && echo \"$ERRANT_PROJECT_DIR\" >> %q",
		outFile, outFile, outFile,
	)
	prov := config.ProvisionConfig{
		Type:   "local",
		Dir:    "external-repo",
		Inline: cmd,
		Env:    map[string]string{"MY_VAR": "hello-local"},
	}

	if err := RunLocalProvisioner(context.Background(), "/fake/ssh_config", "", prov, projectDir); err != nil {
		t.Fatalf("RunLocalProvisioner failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[0] != subDir || lines[1] != "hello-local" || lines[2] != projectDir {
		t.Fatalf("unexpected local provisioner output: %q", string(data))
	}

	emptyProv := config.ProvisionConfig{Type: "local"}
	if err := RunLocalProvisioner(context.Background(), "", "", emptyProv, projectDir); err == nil {
		t.Error("expected error when neither inline nor script is set")
	}

	badDir := config.ProvisionConfig{Type: "local", Dir: "does-not-exist", Inline: "true"}
	if err := RunLocalProvisioner(context.Background(), "", "", badDir, projectDir); err == nil {
		t.Error("expected error when dir does not exist")
	}
}

func TestProvisionStepLocalRunsOnce(t *testing.T) {
	tmpDir := t.TempDir()

	origSSH := execSSHCommand
	execSSHCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	t.Cleanup(func() { execSSHCommand = origSSH })

	var calls int

	origLocal := execLocalCommand
	execLocalCommand = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		calls++

		return exec.CommandContext(ctx, "true")
	}

	t.Cleanup(func() { execLocalCommand = origLocal })

	cluster := &config.ResolvedCluster{
		Project:    "k8s-homelab",
		ProjectDir: tmpDir,
		Nodes: []config.ResolvedNode{
			{Name: "cp-1", Role: "control-plane"},
			{Name: "w-1", Role: "worker"},
		},
	}

	step := config.ResolvedStep{
		Index: 1,
		Name:  "preprod-up",
		Provision: []config.ProvisionConfig{
			{Type: "local", Inline: "echo ok"},
		},
	}

	if err := ProvisionStep(context.Background(), cluster, "/fake/ssh_config", step); err != nil {
		t.Fatalf("ProvisionStep failed: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 local provisioner call for step, got %d", calls)
	}
}
