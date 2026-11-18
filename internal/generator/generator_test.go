package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenos76/errant/internal/config"
)

func TestGenerateSSHConfig(t *testing.T) {
	cluster := &config.ResolvedCluster{
		Project: "test-proj",
		Nodes: []config.ResolvedNode{
			{
				Name: "control-plane-1",
				Role: "control-plane",
				IP:   "192.168.56.10",
				SSH: config.SSHConfig{
					User:       "errant",
					Port:       22,
					PrivateKey: "/home/user/.ssh/id_rsa",
				},
			},
			{
				Name: "worker-1",
				Role: "worker",
				IP:   "192.168.56.21",
				SSH: config.SSHConfig{
					User: "", // Test default fallback
					Port: 0,  // Test default fallback
				},
			},
		},
	}

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "ssh_config")

	err := GenerateSSHConfig(cluster, destFile)
	if err != nil {
		t.Fatalf("GenerateSSHConfig failed: %v", err)
	}

	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	output := string(content)

	if !strings.Contains(output, "Host control-plane-1") {
		t.Errorf("expected Host control-plane-1, got:\n%s", output)
	}

	if !strings.Contains(output, "HostName 192.168.56.10") {
		t.Errorf("expected HostName 192.168.56.10, got:\n%s", output)
	}

	if !strings.Contains(output, "IdentityFile /home/user/.ssh/id_rsa") {
		t.Errorf("expected IdentityFile, got:\n%s", output)
	}

	if !strings.Contains(output, "Host worker-1") {
		t.Errorf("expected Host worker-1, got:\n%s", output)
	}

	if !strings.Contains(output, "User errant") {
		t.Errorf("expected default User errant, got:\n%s", output)
	}

	if !strings.Contains(output, "Port 22") {
		t.Errorf("expected default Port 22, got:\n%s", output)
	}
}

func TestGenerateSSHConfigInvalidPath(t *testing.T) {
	cluster := &config.ResolvedCluster{
		Project: "test-proj",
	}

	err := GenerateSSHConfig(cluster, "/dev/null/invalid/ssh_config")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestGenerateAnsibleInventory(t *testing.T) {
	cluster := &config.ResolvedCluster{
		Project: "test-proj",
		Nodes: []config.ResolvedNode{
			{
				Name: "control-plane-1",
				Role: "control-plane",
				IP:   "192.168.56.10",
				SSH: config.SSHConfig{
					User: "vagrant",
				},
			},
			{
				Name: "worker-1",
				Role: "worker",
				IP:   "192.168.56.21",
			},
			{
				Name: "standalone-node",
				Role: "", // Test ungrouped
				IP:   "192.168.56.30",
			},
		},
	}

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "inventory.ini")

	err := GenerateAnsibleInventory(cluster, destFile)
	if err != nil {
		t.Fatalf("GenerateAnsibleInventory failed: %v", err)
	}

	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	output := string(content)

	if !strings.Contains(output, "[control_plane]") {
		t.Errorf("expected [control_plane] section, got:\n%s", output)
	}

	if !strings.Contains(output, "control-plane-1 ansible_host=192.168.56.10 ansible_user=vagrant") {
		t.Errorf("expected control-plane-1 entry with vagrant user, got:\n%s", output)
	}

	if !strings.Contains(output, "[worker]") {
		t.Errorf("expected [worker] section, got:\n%s", output)
	}

	if !strings.Contains(output, "worker-1 ansible_host=192.168.56.21 ansible_user=errant") {
		t.Errorf("expected worker-1 entry with default errant user, got:\n%s", output)
	}

	if !strings.Contains(output, "[ungrouped]") {
		t.Errorf("expected [ungrouped] section, got:\n%s", output)
	}

	if !strings.Contains(output, "[k8s_cluster:children]") {
		t.Errorf("expected [k8s_cluster:children] section, got:\n%s", output)
	}
}

func TestGenerateAnsibleInventoryInvalidPath(t *testing.T) {
	cluster := &config.ResolvedCluster{
		Project: "test-proj",
	}

	err := GenerateAnsibleInventory(cluster, "/dev/null/invalid/inventory.ini")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}
