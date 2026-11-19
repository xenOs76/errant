package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xenos76/errant/internal/config"
	"github.com/xenos76/errant/internal/pulumi"
	"github.com/xenos76/errant/internal/snapshot"
)

type mockEngineRunner struct {
	applyFunc           func(ctx context.Context, cluster *config.ResolvedCluster) error
	destroyFunc         func(ctx context.Context, cluster *config.ResolvedCluster) error
	haltFunc            func(ctx context.Context, cluster *config.ResolvedCluster) error
	startFunc           func(ctx context.Context, cluster *config.ResolvedCluster) error
	getDomainStatusFunc func(ctx context.Context, domainName string) string
}

func (m *mockEngineRunner) Apply(ctx context.Context, cluster *config.ResolvedCluster) error {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, cluster)
	}

	return nil
}

func (m *mockEngineRunner) Destroy(ctx context.Context, cluster *config.ResolvedCluster) error {
	if m.destroyFunc != nil {
		return m.destroyFunc(ctx, cluster)
	}

	return nil
}

func (m *mockEngineRunner) Halt(ctx context.Context, cluster *config.ResolvedCluster) error {
	if m.haltFunc != nil {
		return m.haltFunc(ctx, cluster)
	}

	return nil
}

func (m *mockEngineRunner) Start(ctx context.Context, cluster *config.ResolvedCluster) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, cluster)
	}

	return nil
}

func (m *mockEngineRunner) GetDomainStatus(ctx context.Context, domainName string) string {
	if m.getDomainStatusFunc != nil {
		return m.getDomainStatusFunc(ctx, domainName)
	}

	return "running"
}

const testErrantYAML = `
version: "1"
project: cli-test

defaults:
  image: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
  memory_mb: 2048
  cpus: 2
  root_disk_size: "20G"
  ssh:
    user: vagrant

network:
  subnet: "192.168.56.0/24"
  bridge: "virbr56"

profiles:
  cp:
    role: control-plane
    memory_mb: 2048
    cpus: 2

topologies:
  standard:
    nodes:
      - name: test-cp
        profile: cp
        ip: 192.168.56.10
`

func setupTestConfigFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "Errantfile.yaml")

	if err := os.WriteFile(cfgPath, []byte(testErrantYAML), 0o600); err != nil {
		t.Fatalf("failed to create test Errantfile: %v", err)
	}

	return cfgPath
}

func TestMain(m *testing.M) {
	origResolve := pulumi.ResolveImageFunc
	pulumi.ResolveImageFunc = func(
		_ context.Context,
		img string,
		_ map[string]config.ImageBuildConfig,
	) (string, error) {
		return img, nil
	}

	origSSH := waitForSSHFunc
	waitForSSHFunc = func(_ context.Context, _ string, _ string, _ time.Duration) error {
		return nil
	}

	origMount := mountSyncedFoldersFunc
	mountSyncedFoldersFunc = func(_ context.Context, _ string, _ config.ResolvedNode) error {
		return nil
	}

	defer func() {
		pulumi.ResolveImageFunc = origResolve
		waitForSSHFunc = origSSH
		mountSyncedFoldersFunc = origMount
	}()

	m.Run()
}

func executeCommand(args ...string) (string, error) {
	return executeCommandWithInput("", args...)
}

func executeCommandWithInput(input string, args ...string) (string, error) {
	cmd := NewRootCmd()
	cmd.SetArgs(args)

	if input != "" {
		cmd.SetIn(strings.NewReader(input))
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	var pipeBuf bytes.Buffer

	done := make(chan struct{})

	go func() {
		_, _ = io.Copy(&pipeBuf, r)

		close(done)
	}()

	err := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	<-done

	_ = r.Close()

	return buf.String() + pipeBuf.String(), err
}

func TestRootAndHelp(t *testing.T) {
	out, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("unexpected error running help: %v", err)
	}

	if !strings.Contains(out, "lightweight, declarative local virtual machine orchestrator") {
		t.Errorf("unexpected help output: %s", out)
	}
}

func TestValidateCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("ValidFile", func(t *testing.T) {
		out, err := executeCommand("validate", "--config", cfgPath)
		if err != nil {
			t.Fatalf("validation failed unexpectedly: %v", err)
		}

		if !strings.Contains(out, "Schema valid") {
			t.Errorf("expected 'Schema valid', got: %s", out)
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := executeCommand("validate", "--config", "/nonexistent/path/Errantfile.yaml")
		if err == nil {
			t.Fatal("expected error with nonexistent config, got nil")
		}
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		badFile := filepath.Join(t.TempDir(), "bad.yaml")
		_ = os.WriteFile(badFile, []byte("bad: ["), 0o600)

		_, err := executeCommand("validate", "--config", badFile)
		if err == nil {
			t.Fatal("expected error with bad yaml, got nil")
		}
	})
}

func TestInitCmd(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("NonInteractiveDefaults", func(t *testing.T) {
		target := filepath.Join(tmpDir, "Errantfile.yaml")

		out, err := executeCommand("init", "-y", "-o", target)
		if err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		if !strings.Contains(out, "Successfully created") {
			t.Errorf("expected success message, got: %s", out)
		}
	})

	t.Run("NonInteractiveSingleVM", func(t *testing.T) {
		target := filepath.Join(tmpDir, "SingleErrantfile.yaml")

		out, err := executeCommand("init", "-y", "-a", "single-vm", "-o", target)
		if err != nil {
			t.Fatalf("init single-vm failed: %v", err)
		}

		if !strings.Contains(out, "single-vm") {
			t.Errorf("expected single-vm in output, got: %s", out)
		}
	})
}

func TestInventoryCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("Success", func(t *testing.T) {
		out, err := executeCommand("inventory", "--config", cfgPath)
		if err != nil {
			t.Fatalf("inventory command failed: %v", err)
		}

		if !strings.Contains(out, "[control_plane]") {
			t.Errorf("expected [control_plane] in inventory output, got: %s", out)
		}
	})

	t.Run("MissingConfig", func(t *testing.T) {
		_, err := executeCommand("inventory", "--config", "/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSSHConfigCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("Success", func(t *testing.T) {
		out, err := executeCommand("ssh-config", "--config", cfgPath)
		if err != nil {
			t.Fatalf("ssh-config command failed: %v", err)
		}

		if !strings.Contains(out, "Host test-cp") {
			t.Errorf("expected 'Host test-cp' in output, got: %s", out)
		}
	})

	t.Run("MissingConfig", func(t *testing.T) {
		_, err := executeCommand("ssh-config", "--config", "/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestStatusCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			getDomainStatusFunc: func(_ context.Context, _ string) string {
				return "running"
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	t.Run("Success", func(t *testing.T) {
		out, err := executeCommand("status", "--config", cfgPath)
		if err != nil {
			t.Fatalf("status command failed: %v", err)
		}

		if !strings.Contains(out, "test-cp") {
			t.Errorf("expected test-cp in status output, got: %s", out)
		}
	})

	t.Run("MissingConfig", func(t *testing.T) {
		_, err := executeCommand("status", "--config", "/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("InvalidTopology", func(t *testing.T) {
		_, err := executeCommand("status", "--config", cfgPath, "--topology", "nonexistent")
		if err == nil {
			t.Fatal("expected error with nonexistent topology, got nil")
		}
	})
}

func TestHaltCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{}
	}

	defer func() { runnerFactory = origFactory }()

	t.Run("Success", func(t *testing.T) {
		_, err := executeCommand("halt", "--config", cfgPath)
		if err != nil {
			t.Fatalf("halt command failed: %v", err)
		}
	})

	t.Run("MissingConfig", func(t *testing.T) {
		_, err := executeCommand("halt", "--config", "/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSSHCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("NodeNotFound", func(t *testing.T) {
		_, err := executeCommand("ssh", "unknown-node", "--config", cfgPath)
		if err == nil {
			t.Fatal("expected error for unknown node, got nil")
		}
	})

	t.Run("MissingSSHConfigFile", func(t *testing.T) {
		_, err := executeCommand("ssh", "test-cp", "--config", cfgPath)
		if err == nil {
			t.Fatal("expected error for missing ssh_config file, got nil")
		}
	})
}

func TestDestroyCmd_MissingConfig(t *testing.T) {
	_, err := executeCommand("destroy", "--config", "/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDestroyCmd_Force(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{}
	}

	defer func() { runnerFactory = origFactory }()

	_ = os.MkdirAll(filepath.Join(filepath.Dir(cfgPath), ".errant", "state"), 0o750)

	out, err := executeCommand("destroy", "--config", cfgPath, "--force")
	if err != nil {
		t.Fatalf("destroy failed: %v", err)
	}

	if !strings.Contains(out, "targeted for destruction") {
		t.Errorf("expected resource listing, got: %s", out)
	}

	if !strings.Contains(out, "successfully destroyed") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestDestroyCmd_PromptDecline(t *testing.T) {
	cfgPath := setupTestConfigFile(t)
	destroyCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			destroyFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				destroyCalled = true

				return nil
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	out, err := executeCommandWithInput("n\n", "destroy", "--config", cfgPath)
	if err != nil {
		t.Fatalf("destroy command failed: %v", err)
	}

	if destroyCalled {
		t.Error("destroy runner should not have been called on prompt decline")
	}

	if !strings.Contains(out, "Destruction cancelled") {
		t.Errorf("expected cancellation message, got: %s", out)
	}
}

func TestDestroyCmd_PromptAccept(t *testing.T) {
	cfgPath := setupTestConfigFile(t)
	destroyCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			destroyFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				destroyCalled = true

				return nil
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	out, err := executeCommandWithInput("y\n", "destroy", "--config", cfgPath)
	if err != nil {
		t.Fatalf("destroy command failed: %v", err)
	}

	if !destroyCalled {
		t.Error("destroy runner should have been called on prompt accept")
	}

	if !strings.Contains(out, "successfully destroyed") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestDestroyCmd_AutoYes(t *testing.T) {
	cfgPath := setupTestConfigFile(t)
	destroyCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			destroyFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				destroyCalled = true

				return nil
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	out, err := executeCommand("destroy", "--config", cfgPath, "-y")
	if err != nil {
		t.Fatalf("destroy command failed: %v", err)
	}

	if !destroyCalled {
		t.Error("destroy runner should have been called with -y")
	}

	if !strings.Contains(out, "successfully destroyed") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestUpCmdAndPrintSummary(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("MissingConfig", func(t *testing.T) {
		_, err := executeCommand("up", "--config", "/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("PrintClusterSummary", func(t *testing.T) {
		configFile = cfgPath

		cluster, _, err := loadResolvedCluster()
		if err != nil {
			t.Fatalf("loading cluster: %v", err)
		}

		printClusterSummary(cluster, "/tmp/fake_ssh", "/tmp/fake_inv")
	})
}

func TestUpCmdSuccess(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{}
	}

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		return nil
	}

	defer func() {
		runnerFactory = origFactory
		provisionClusterFunc = origProv
	}()

	out, err := executeCommand("up", "--config", cfgPath)
	if err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if !strings.Contains(out, "Topology successfully deployed") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestUpCmdFailure(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			applyFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				return errors.New("simulated apply failure")
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	_, err := executeCommand("up", "--config", cfgPath)
	if err == nil {
		t.Fatal("expected error from up when runner fails, got nil")
	}
}

func TestUpCmdExistingVMsSkipsProvisioning(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	started := false
	provisionCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			getDomainStatusFunc: func(_ context.Context, _ string) string {
				return "shut off"
			},
			startFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				started = true

				return nil
			},
		}
	}

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		provisionCalled = true

		return nil
	}

	defer func() {
		runnerFactory = origFactory
		provisionClusterFunc = origProv
	}()

	out, err := executeCommand("up", "--config", cfgPath)
	if err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if !started {
		t.Error("expected runner.Start to be called for shut off VMs")
	}

	if provisionCalled {
		t.Error("expected provisioning to be skipped for existing VMs")
	}

	if !strings.Contains(out, "Skipping provisioning") {
		t.Errorf("expected skip provisioning message, got: %s", out)
	}
}

func TestUpCmdNewVMRunsProvisioning(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	provisionCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			getDomainStatusFunc: func(_ context.Context, _ string) string {
				return "not created"
			},
		}
	}

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		provisionCalled = true

		return nil
	}

	defer func() {
		runnerFactory = origFactory
		provisionClusterFunc = origProv
	}()

	_, err := executeCommand("up", "--config", cfgPath)
	if err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if !provisionCalled {
		t.Error("expected provisioning to run for newly created VMs")
	}
}

func TestUpCmdForceProvisioning(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	provisionCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			getDomainStatusFunc: func(_ context.Context, _ string) string {
				return "shut off"
			},
		}
	}

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		provisionCalled = true

		return nil
	}

	defer func() {
		runnerFactory = origFactory
		provisionClusterFunc = origProv
	}()

	_, err := executeCommand("up", "--config", cfgPath, "--provision")
	if err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if !provisionCalled {
		t.Error("expected provisioning to run when --provision is specified")
	}
}

func TestUpCmdNoProvisionFlag(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	provisionCalled := false

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			getDomainStatusFunc: func(_ context.Context, _ string) string {
				return "not created"
			},
		}
	}

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		provisionCalled = true

		return nil
	}

	defer func() {
		runnerFactory = origFactory
		provisionClusterFunc = origProv
	}()

	_, err := executeCommand("up", "--config", cfgPath, "--no-provision")
	if err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if provisionCalled {
		t.Error("expected provisioning to be skipped when --no-provision is specified")
	}
}

func TestUpCmdConflictingProvisionFlags(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{}
	}

	defer func() { runnerFactory = origFactory }()

	_, err := executeCommand("up", "--config", cfgPath, "--provision", "--no-provision")
	if err == nil {
		t.Fatal("expected error with conflicting flags, got nil")
	}
}

func TestUpCmdStartFailure(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	origFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner {
		return &mockEngineRunner{
			startFunc: func(_ context.Context, _ *config.ResolvedCluster) error {
				return errors.New("simulated start failure")
			},
		}
	}

	defer func() { runnerFactory = origFactory }()

	_, err := executeCommand("up", "--config", cfgPath)
	if err == nil {
		t.Fatal("expected error when start fails, got nil")
	}
}

func TestSSHCmdSuccess(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	// create fake ssh_config
	sshDir := filepath.Join(filepath.Dir(cfgPath), ".errant")
	_ = os.MkdirAll(sshDir, 0o750)
	_ = os.WriteFile(filepath.Join(sshDir, "ssh_config"), []byte("Host test-cp\n"), 0o600)

	origSSH := execSSHCommand
	execSSHCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("true")
	}

	defer func() { execSSHCommand = origSSH }()

	_, err := executeCommand("ssh", "test-cp", "--config", cfgPath)
	if err != nil {
		t.Fatalf("ssh command failed: %v", err)
	}
}

func TestExecuteFunc(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"errant", "--help"}

	if err := Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
}

func TestSSHCompletion(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	t.Run("ValidArgsCompletion", func(t *testing.T) {
		out, err := executeCommand("__complete", "--config", cfgPath, "ssh", "")
		if err != nil {
			t.Fatalf("completion failed: %v", err)
		}

		if !strings.Contains(out, "test-cp") {
			t.Errorf("expected test-cp in completion output, got: %s", out)
		}
	})

	t.Run("ValidArgsCompletionWithArgs", func(t *testing.T) {
		out, err := executeCommand("__complete", "--config", cfgPath, "ssh", "test-cp", "")
		if err != nil {
			t.Fatalf("completion failed: %v", err)
		}

		if strings.Contains(out, "test-cp") {
			t.Errorf("expected no completion for second argument, got: %s", out)
		}
	})

	t.Run("ValidArgsCompletionInvalidConfig", func(t *testing.T) {
		out, err := executeCommand("__complete", "ssh", "", "--config", "/nonexistent/config.yaml")
		if err != nil {
			t.Fatalf("completion failed: %v", err)
		}

		if strings.Contains(out, "test-cp") {
			t.Errorf("expected no completion on invalid config, got: %s", out)
		}
	})
}

func TestProvisionCmd(t *testing.T) {
	cfgPath := setupTestConfigFile(t)

	sshDir := filepath.Join(filepath.Dir(cfgPath), ".errant")
	_ = os.MkdirAll(sshDir, 0o750)
	_ = os.WriteFile(filepath.Join(sshDir, "ssh_config"), []byte("Host test-cp\n"), 0o600)

	origProv := provisionClusterFunc
	provisionClusterFunc = func(_ context.Context, _ *config.ResolvedCluster, _ string, _ string) error {
		return nil
	}

	defer func() { provisionClusterFunc = origProv }()

	t.Run("Success", func(t *testing.T) {
		_, err := executeCommand("provision", "--config", cfgPath)
		if err != nil {
			t.Fatalf("provision failed: %v", err)
		}
	})

	t.Run("TargetNode", func(t *testing.T) {
		_, err := executeCommand("provision", "test-cp", "--config", cfgPath)
		if err != nil {
			t.Fatalf("provision targeted node failed: %v", err)
		}
	})

	t.Run("UnknownNode", func(t *testing.T) {
		_, err := executeCommand("provision", "unknown-node", "--config", cfgPath)
		if err == nil {
			t.Fatal("expected error for unknown node, got nil")
		}
	})

	t.Run("MissingSSHConfig", func(t *testing.T) {
		_ = os.Remove(filepath.Join(sshDir, "ssh_config"))

		_, err := executeCommand("provision", "--config", cfgPath)
		if err == nil {
			t.Fatal("expected error for missing ssh_config, got nil")
		}
	})
}

type mockSnapshotManager struct {
	created    []string
	reverted   []string
	deleted    []string
	cleanedUp  []string
	listedSnap []string
}

func (m *mockSnapshotManager) Create(
	_ context.Context,
	domainName, snapName, _, _ string,
) error {
	m.created = append(m.created, domainName+":"+snapName)

	return nil
}

func (m *mockSnapshotManager) Revert(_ context.Context, domainName, snapName string) error {
	m.reverted = append(m.reverted, domainName+":"+snapName)

	return nil
}

func (m *mockSnapshotManager) Delete(_ context.Context, domainName, snapName string) error {
	m.deleted = append(m.deleted, domainName+":"+snapName)

	return nil
}

func (m *mockSnapshotManager) List(_ context.Context, _ string) ([]string, error) {
	return m.listedSnap, nil
}

func (m *mockSnapshotManager) CleanupAll(_ context.Context, domainName, prefix string) error {
	m.cleanedUp = append(m.cleanedUp, domainName+":"+prefix)

	return nil
}

const testErrantWithStepsYAML = `
version: "1"
project: step-cli-test
defaults:
  image: "https://cloud.debian.org/images/cloud/trixie/daily/latest/debian-13-genericcloud-amd64-daily.qcow2"
  ssh:
    user: errant
topologies:
  dev:
    nodes:
      - name: debian-vm
        ip: 192.168.57.10
steps:
  - name: webserver
    description: Install nginx
    provision:
      - type: shell
        inline: apt-get install -y nginx
  - name: app
    description: Deploy app
    provision:
      - type: shell
        inline: echo app
`

func setupStepLifecycleTest(t *testing.T) (string, *mockSnapshotManager, *[]string) {
	t.Helper()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "Errantfile.yaml")

	if err := os.WriteFile(cfgPath, []byte(testErrantWithStepsYAML), 0o600); err != nil {
		t.Fatalf("writing Errantfile: %v", err)
	}

	mockSnap := &mockSnapshotManager{
		listedSnap: []string{"errant-step-0", "errant-step-1", "errant-step-2"},
	}

	origSnapFactory := snapshotManagerFactory
	snapshotManagerFactory = func() snapshot.Manager { return mockSnap }

	t.Cleanup(func() { snapshotManagerFactory = origSnapFactory })

	origRunnerFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner { return &mockEngineRunner{} }

	t.Cleanup(func() { runnerFactory = origRunnerFactory })

	var provSteps []string

	origProvStep := provisionStepFunc
	provisionStepFunc = func(
		_ context.Context,
		_ *config.ResolvedCluster,
		_ string,
		step config.ResolvedStep,
	) error {
		provSteps = append(provSteps, step.Name)

		return nil
	}

	t.Cleanup(func() { provisionStepFunc = origProvStep })

	return cfgPath, mockSnap, &provSteps
}

func TestStepUpApplyAndList(t *testing.T) {
	cfgPath, mockSnap, provSteps := setupStepLifecycleTest(t)

	if _, err := executeCommand("up", "--config", cfgPath, "--step", "1"); err != nil {
		t.Fatalf("up --step 1 failed: %v", err)
	}

	if len(*provSteps) != 1 || (*provSteps)[0] != "webserver" {
		t.Fatalf("expected only webserver provisioned, got %v", *provSteps)
	}

	if len(mockSnap.created) != 2 {
		t.Fatalf("expected 2 snapshots created (step 0 & 1), got %v", mockSnap.created)
	}

	out, err := executeCommand("steps", "list", "--config", cfgPath)
	if err != nil {
		t.Fatalf("steps list failed: %v", err)
	}

	if !strings.Contains(out, "webserver") || !strings.Contains(out, "saved") || !strings.Contains(out, "pending") {
		t.Errorf("unexpected steps list output:\n%s", out)
	}

	if _, err := executeCommand("steps", "apply", "--config", cfgPath); err != nil {
		t.Fatalf("steps apply failed: %v", err)
	}

	if len(*provSteps) != 2 || (*provSteps)[1] != "app" {
		t.Fatalf("expected app provisioned on steps apply, got %v", *provSteps)
	}
}

func TestStepRevertAndDestroyCleanup(t *testing.T) {
	cfgPath, mockSnap, _ := setupStepLifecycleTest(t)

	if _, err := executeCommand("up", "--config", cfgPath); err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if _, err := executeCommand("steps", "revert", "1", "--prune", "--config", cfgPath); err != nil {
		t.Fatalf("steps revert failed: %v", err)
	}

	if len(mockSnap.reverted) != 1 || mockSnap.reverted[0] != "step-cli-test-debian-vm:errant-step-1" {
		t.Errorf("unexpected revert calls: %v", mockSnap.reverted)
	}

	if len(mockSnap.deleted) != 1 || mockSnap.deleted[0] != "step-cli-test-debian-vm:errant-step-2" {
		t.Errorf("expected errant-step-2 pruned, got %v", mockSnap.deleted)
	}

	if _, err := executeCommand("destroy", "-y", "--config", cfgPath); err != nil {
		t.Fatalf("destroy failed: %v", err)
	}

	if len(mockSnap.cleanedUp) != 2 ||
		mockSnap.cleanedUp[0] != "step-cli-test-debian-vm:errant-step-" ||
		mockSnap.cleanedUp[1] != "step-cli-test-debian-vm:errant-user-" {
		t.Errorf("expected CleanupAll for both prefixes on destroy, got %v", mockSnap.cleanedUp)
	}

	if snapshot.StateExists(filepath.Dir(cfgPath)) {
		t.Error("expected steps.json to be removed after destroy")
	}
}

func TestUpWithoutStepsDoesNotTouchSnapshots(t *testing.T) {
	cfgPath := setupTestConfigFile(t)
	mockSnap := &mockSnapshotManager{}

	origSnapFactory := snapshotManagerFactory
	snapshotManagerFactory = func() snapshot.Manager { return mockSnap }

	defer func() { snapshotManagerFactory = origSnapFactory }()

	origRunnerFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner { return &mockEngineRunner{} }

	defer func() { runnerFactory = origRunnerFactory }()

	if _, err := executeCommand("up", "--config", cfgPath); err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if len(mockSnap.created) != 0 {
		t.Errorf("expected zero snapshots when steps are not configured, got %v", mockSnap.created)
	}

	if snapshot.StateExists(filepath.Dir(cfgPath)) {
		t.Error("expected no steps.json when steps are not configured")
	}
}

func TestAdhocStepSaveRevertDeleteWithoutConfigSteps(t *testing.T) {
	cfgPath := setupTestConfigFile(t)
	mockSnap := &mockSnapshotManager{}

	origSnapFactory := snapshotManagerFactory
	snapshotManagerFactory = func() snapshot.Manager { return mockSnap }

	t.Cleanup(func() { snapshotManagerFactory = origSnapFactory })

	origRunnerFactory := runnerFactory
	runnerFactory = func(_, _ string) EngineRunner { return &mockEngineRunner{} }

	t.Cleanup(func() { runnerFactory = origRunnerFactory })

	if _, err := executeCommand("steps", "save", "my-exp", "-m", "debug", "--config", cfgPath); err != nil {
		t.Fatalf("steps save failed: %v", err)
	}

	if len(mockSnap.created) != 1 || mockSnap.created[0] != "cli-test-test-cp:errant-user-my-exp" {
		t.Fatalf("unexpected snapshot created: %v", mockSnap.created)
	}

	out, err := executeCommand("steps", "list", "--config", cfgPath)
	if err != nil {
		t.Fatalf("steps list failed: %v", err)
	}

	if !strings.Contains(out, "user:my-exp") || !strings.Contains(out, "debug") {
		t.Errorf("expected user:my-exp in steps list, got:\n%s", out)
	}

	if _, err := executeCommand("steps", "revert", "my-exp", "--config", cfgPath); err != nil {
		t.Fatalf("steps revert unprefixed custom failed: %v", err)
	}

	if len(mockSnap.reverted) != 1 || mockSnap.reverted[0] != "cli-test-test-cp:errant-user-my-exp" {
		t.Errorf("unexpected revert: %v", mockSnap.reverted)
	}

	if _, err := executeCommand("steps", "delete", "user:my-exp", "--config", cfgPath); err != nil {
		t.Fatalf("steps delete failed: %v", err)
	}

	if len(mockSnap.deleted) != 1 || mockSnap.deleted[0] != "cli-test-test-cp:errant-user-my-exp" {
		t.Errorf("unexpected delete: %v", mockSnap.deleted)
	}
}

func TestAdhocStepDisambiguationWithDeclarativeStep(t *testing.T) {
	cfgPath, mockSnap, _ := setupStepLifecycleTest(t)

	if _, err := executeCommand("up", "--config", cfgPath, "--step", "1"); err != nil {
		t.Fatalf("up --step 1 failed: %v", err)
	}

	// Save an ad-hoc step whose name matches a declarative step ("app") in Errantfile.yaml.
	if _, err := executeCommand("steps", "save", "app", "-m", "manual app", "--config", cfgPath); err != nil {
		t.Fatalf("steps save 'app' failed: %v", err)
	}

	// Applying declarative step 2 while on user:app should require revert or --force.
	if _, err := executeCommand("steps", "apply", "2", "--config", cfgPath); err == nil {
		t.Fatal("expected error applying declarative step while on user:app without --force")
	}

	// Reverting with explicit user:app targets errant-user-app.
	if _, err := executeCommand("steps", "revert", "user:app", "--config", cfgPath); err != nil {
		t.Fatalf("revert user:app failed: %v", err)
	}

	if mockSnap.reverted[0] != "step-cli-test-debian-vm:errant-user-app" {
		t.Errorf("expected errant-user-app reverted, got %v", mockSnap.reverted)
	}

	// Reverting with unprefixed "app" targets declarative step 2 (errant-step-2) and prints ambiguity note.
	out, err := executeCommand("steps", "revert", "app", "--config", cfgPath)
	if err != nil {
		t.Fatalf("revert app failed: %v", err)
	}

	if mockSnap.reverted[1] != "step-cli-test-debian-vm:errant-step-2" {
		t.Errorf("expected errant-step-2 reverted, got %v", mockSnap.reverted)
	}

	if !strings.Contains(out, "user:app") {
		t.Errorf("expected ambiguity note mentioning user:app, got:\n%s", out)
	}
}
