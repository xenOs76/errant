package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const sampleErrantYAML = `
version: "1"
project: k8s-test

defaults:
  image: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
  image_alias: "ubuntu"
  memory_mb: 2048
  cpus: 2
  root_disk_size: "25G"
  ssh:
    user: vagrant
    public_key: "/tmp/fake.pub"
    port: 22
  synced_folders:
    - source: "."
      target: "/errant"
      type: virtiofs

network:
  subnet: "192.168.56.0/24"
  bridge: "virbr56"

profiles:
  control-plane:
    role: control-plane
    memory_mb: 4096
    cpus: 2
    disks:
      - device: vdb
        size: "20G"
    synced_folders:
      - source: "./data"
        target: "/data"
    provision:
      - type: shell
        inline: "echo CP ready"

  worker:
    role: worker
    memory_mb: 2048
    cpus: 2
    provision:
      - type: shell
        inline: "echo Worker ready"

topologies:
  standard:
    nodes:
      - name: kube-cp
        profile: control-plane
        ip: 192.168.56.10

      - name: "kube-worker-${index}"
        profile: worker
        count: 2
        ip_start: 192.168.56.21

  ha:
    nodes:
      - name: "kube-cp-${index}"
        profile: control-plane
        count: 3
        ip_start: 192.168.56.10
        memory_mb: 2560
        disks:
          - condition: "${index} == 1"
            device: vdb
            size: "20G"
          - condition: "${index} > 1"
            device: vdc
            size: "10G"
          - condition: "${index} < 3"
            device: vdd
            size: "5G"
          - condition: "${index} >= 2"
            device: vde
            size: "5G"
          - condition: "${index} <= 2"
            device: vdf
            size: "5G"
          - condition: "${index} != 2"
            device: vdg
            size: "5G"
          - condition: "unsupported syntax"
            device: vdh
            size: "5G"

global_provision:
  - type: shell
    inline: "echo Global ready"
`

func TestValidateAndResolve(t *testing.T) {
	if err := ValidateYAML([]byte(sampleErrantYAML)); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(sampleErrantYAML), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	std, err := cfg.ResolveCluster("standard")
	if err != nil {
		t.Fatalf("resolve standard failed: %v", err)
	}

	testStandardTopologyCP(t, std)
	testStandardTopologyWorkers(t, std)
}

func testStandardTopologyCP(t *testing.T, std *ResolvedCluster) {
	t.Helper()

	if len(std.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(std.Nodes))
	}

	cp := std.Nodes[0]
	if cp.Name != "kube-cp" || cp.Role != "control-plane" {
		t.Errorf("unexpected CP identification: %+v", cp)
	}

	if cp.MemoryMB != 4096 || cp.IP != "192.168.56.10" {
		t.Errorf("unexpected CP sizing/IP: %+v", cp)
	}

	if len(cp.Disks) != 1 || cp.Disks[0].Device != "vdb" {
		t.Errorf("expected 1 secondary disk on CP, got: %+v", cp.Disks)
	}

	if len(cp.SyncedFolders) != 2 {
		t.Errorf("expected 2 synced folders (default + profile), got: %+v", cp.SyncedFolders)
	}
}

func testStandardTopologyWorkers(t *testing.T, std *ResolvedCluster) {
	t.Helper()

	w1 := std.Nodes[1]
	if w1.Name != "kube-worker-1" || w1.IP != "192.168.56.21" || w1.Role != "worker" {
		t.Errorf("unexpected worker-1 values: %+v", w1)
	}

	if len(w1.SyncedFolders) != 1 || w1.SyncedFolders[0].Target != "/errant" {
		t.Errorf("expected default synced folders on worker, got: %+v", w1.SyncedFolders)
	}

	w2 := std.Nodes[2]
	if w2.Name != "kube-worker-2" || w2.IP != "192.168.56.22" || w2.Role != "worker" {
		t.Errorf("unexpected worker-2 values: %+v", w2)
	}
}

func TestResolveHA(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(sampleErrantYAML), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	ha, err := cfg.ResolveCluster("ha")
	if err != nil {
		t.Fatalf("resolve ha failed: %v", err)
	}

	if len(ha.Nodes) != 3 {
		t.Fatalf("expected 3 ha nodes, got %d", len(ha.Nodes))
	}

	ha1 := ha.Nodes[0]
	if ha1.Name != "kube-cp-1" || ha1.MemoryMB != 2560 || ha1.IP != "192.168.56.10" {
		t.Errorf("unexpected ha1 values: %+v", ha1)
	}

	if len(ha1.Disks) < 3 {
		t.Errorf("expected at least 3 disks matching condition on ha1, got %d", len(ha1.Disks))
	}
}

func TestTopologySelection(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(sampleErrantYAML), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	resolved, err := cfg.ResolveCluster("")
	if err != nil {
		t.Fatalf("unexpected error resolving empty topology: %v", err)
	}

	if resolved.Topology != "standard" {
		t.Errorf("expected default topology 'standard', got %q", resolved.Topology)
	}

	_, err = cfg.ResolveCluster("non-existent")
	if err == nil {
		t.Fatal("expected error resolving non-existent topology, got nil")
	}
}

func TestResolveInvalidIP(t *testing.T) {
	badCfg := &Config{
		Version: "1",
		Project: "bad-ip",
		Topologies: map[string]TopologyConfig{
			"test": {
				Nodes: []NodeConfig{
					{
						Name:    "node-${index}",
						Count:   2,
						IPStart: "not-an-ip",
					},
				},
			},
		},
	}

	_, err := badCfg.ResolveCluster("test")
	if err == nil {
		t.Fatal("expected error with invalid IPStart, got nil")
	}
}

func TestResolveMissingIP(t *testing.T) {
	cfg := &Config{
		Version: "1",
		Project: "missing-ip",
		Topologies: map[string]TopologyConfig{
			"test": {
				Nodes: []NodeConfig{
					{
						Name:  "node-1",
						Count: 1,
					},
				},
			},
		},
	}

	cluster, err := cfg.ResolveCluster("test")
	if err != nil {
		t.Fatalf("unexpected error resolving node without explicit IP: %v", err)
	}

	if cluster.Nodes[0].IP != "" {
		t.Errorf("expected empty IP, got %q", cluster.Nodes[0].IP)
	}
}

func TestResolveDirectOverrides(t *testing.T) {
	cfg := &Config{
		Version: "1",
		Project: "direct-overrides",
		Topologies: map[string]TopologyConfig{
			"test": {
				Nodes: []NodeConfig{
					{
						Name:         "custom-node",
						Role:         "database",
						Image:        "https://example.com/db.img",
						MemoryMB:     8192,
						CPUs:         4,
						RootDiskSize: "100G",
						IP:           "192.168.56.50",
						Disks: []DiskConfig{
							{Device: "vdb", Size: "50G"},
						},
					},
				},
			},
		},
	}

	cluster, err := cfg.ResolveCluster("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node := cluster.Nodes[0]
	if node.Role != "database" || node.MemoryMB != 8192 || node.CPUs != 4 || node.RootDiskSize != "100G" {
		t.Errorf("overrides not respected: %+v", node)
	}
}

func TestResolveNetworkOptions(t *testing.T) {
	dhcp := false
	cfg := &Config{
		Version: "1",
		Project: "net-test",
		Network: NetworkConfig{
			Mode:    "private",
			Subnet:  "10.0.0.0/24",
			Bridge:  "virbr10",
			DHCP:    &dhcp,
			Gateway: "10.0.0.1",
		},
		Topologies: map[string]TopologyConfig{
			"test": {
				Nodes: []NodeConfig{
					{Name: "n1", IP: "10.0.0.10"},
				},
			},
		},
	}

	cluster, err := cfg.ResolveCluster("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Network.DHCP || cluster.Network.Gateway != "10.0.0.1" {
		t.Errorf("network fields not resolved correctly: %+v", cluster.Network)
	}
}

func TestFindConfigFileExplicit(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := FindConfigFile(filepath.Join(tmpDir, "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error for non-existent explicit path")
	}

	target := filepath.Join(tmpDir, "CustomErrantfile.yaml")
	if err := os.WriteFile(target, []byte(sampleErrantYAML), 0o600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	found, err := FindConfigFile(target)
	if err != nil {
		t.Fatalf("unexpected error finding explicit file: %v", err)
	}

	if found != target {
		t.Errorf("expected %s, got %s", target, found)
	}
}

func TestFindConfigFileSearch(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subproject", "deep")

	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	cfgFile := filepath.Join(tmpDir, "subproject", "Errantfile.yaml")
	if err := os.WriteFile(cfgFile, []byte(sampleErrantYAML), 0o600); err != nil {
		t.Fatalf("writing Errantfile: %v", err)
	}

	origDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(origDir)
	}()

	_ = os.Chdir(subDir)

	found, err := FindConfigFile("")
	if err != nil {
		t.Fatalf("failed finding candidate in parent dir: %v", err)
	}

	if found != cfgFile {
		t.Errorf("expected %s, got %s", cfgFile, found)
	}
}

func TestLoadFileValid(t *testing.T) {
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.yaml")

	if err := os.WriteFile(validPath, []byte(sampleErrantYAML), 0o600); err != nil {
		t.Fatalf("writing valid: %v", err)
	}

	cfg, err := LoadFile(validPath)
	if err != nil {
		t.Fatalf("unexpected error loading valid: %v", err)
	}

	if cfg.Project != "k8s-test" {
		t.Errorf("expected project k8s-test, got %s", cfg.Project)
	}
}

func TestLoadFileErrors(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadFile(filepath.Join(tmpDir, "ghost.yaml"))
	if err == nil {
		t.Fatal("expected error loading non-existent file")
	}

	badYAML := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(badYAML, []byte("version: [unclosed"), 0o600); err != nil {
		t.Fatalf("writing bad yaml: %v", err)
	}

	_, err = LoadFile(badYAML)
	if err == nil {
		t.Fatal("expected error loading invalid yaml")
	}

	invalidSchema := filepath.Join(tmpDir, "invalid_schema.yaml")
	if err := os.WriteFile(invalidSchema, []byte("project: test\nversion: 12345"), 0o600); err != nil {
		t.Fatalf("writing invalid schema: %v", err)
	}

	_, err = LoadFile(invalidSchema)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestBoxResolutionAndSchemaV2(t *testing.T) {
	v2YAML := `
version: "2"
project: box-test

defaults:
  box: ubuntu/noble64
  memory_mb: 2048
  cpus: 2

network:
  subnet: "192.168.56.0/24"
  bridge: "virbr56"

profiles:
  debian-profile:
    box: debian/bookworm64
    memory_mb: 1024

topologies:
  standard:
    nodes:
      - name: default-node
        ip: 192.168.56.10
      - name: profile-node
        profile: debian-profile
        ip: 192.168.56.11
      - name: override-node
        box: nixos/24.11
        ip: 192.168.56.12
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "v2.yaml")

	if err := os.WriteFile(filePath, []byte(v2YAML), 0o600); err != nil {
		t.Fatalf("writing v2 yaml: %v", err)
	}

	cfg, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error validating and loading v2 yaml: %v", err)
	}

	resolved, err := cfg.ResolveCluster("standard")
	if err != nil {
		t.Fatalf("unexpected error resolving cluster: %v", err)
	}

	if len(resolved.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(resolved.Nodes))
	}

	// default-node should inherit ubuntu/noble64
	if resolved.Nodes[0].Image != "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img" {
		t.Errorf("expected ubuntu noble URL, got %s", resolved.Nodes[0].Image)
	}

	// profile-node should inherit debian/bookworm64
	expectedDebian := "https://cloud.debian.org/images/cloud/bookworm/latest/" +
		"debian-12-genericcloud-amd64.qcow2"
	if resolved.Nodes[1].Image != expectedDebian {
		t.Errorf("expected debian bookworm URL, got %s", resolved.Nodes[1].Image)
	}

	// override-node should have nixos/24.11
	expectedNixOS := "nix://channel:nixos-24.11?format=qcow"
	if resolved.Nodes[2].Image != expectedNixOS {
		t.Errorf("expected nixos 24.11 URL, got %s", resolved.Nodes[2].Image)
	}
}

func TestDefaultSSHUser_Errant(t *testing.T) {
	yamlContent := `
version: "1"
project: test-default-user
defaults:
  image: "ubuntu/noble64"
network:
  subnet: "192.168.56.0/24"
topologies:
  standard:
    nodes:
      - name: test-vm
        ip: 192.168.56.10
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Errantfile.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("writing test yaml: %v", err)
	}

	cfg, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error validating and loading yaml: %v", err)
	}

	resolved, err := cfg.ResolveCluster("standard")
	if err != nil {
		t.Fatalf("unexpected error resolving cluster: %v", err)
	}

	if len(resolved.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resolved.Nodes))
	}

	if resolved.Nodes[0].SSH.User != "errant" {
		t.Errorf("expected default SSH user 'errant', got %q", resolved.Nodes[0].SSH.User)
	}
}

func TestImagesBlockValidationAndResolution(t *testing.T) {
	yamlContent := `
version: "2"
project: test-images-block
network:
  subnet: "192.168.56.0/24"
images:
  custom-nixos:
    builder: "nixos-generators"
    channel: "nixos-26.05"
    format: "qcow"
    packages:
      - "curl"
      - "git"
    inline: |
      services.openssh.enable = true;
topologies:
  standard:
    nodes:
      - name: nix-vm
        image: custom-nixos
        ip: 192.168.56.10
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Errantfile.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("writing test yaml: %v", err)
	}

	cfg, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error validating and loading yaml: %v", err)
	}

	if len(cfg.Images) != 1 {
		t.Fatalf("expected 1 image in config, got %d", len(cfg.Images))
	}

	img, ok := cfg.Images["custom-nixos"]
	if !ok {
		t.Fatal("custom-nixos not found in cfg.Images")
	}

	if img.Channel != "nixos-26.05" || img.Format != "qcow" || len(img.Packages) != 2 {
		t.Errorf("unexpected image config parsed: %+v", img)
	}

	resolved, err := cfg.ResolveCluster("standard")
	if err != nil {
		t.Fatalf("unexpected error resolving cluster: %v", err)
	}

	if len(resolved.Images) != 1 {
		t.Fatalf("expected 1 image in resolved cluster, got %d", len(resolved.Images))
	}
}

func TestStepsSchemaAndResolution(t *testing.T) {
	yamlContent := `
version: "1"
project: steps-test
defaults:
  image: debian/trixie64
  snapshot_mode: live
  shutdown_timeout: "3m"
topologies:
  dev:
    nodes:
      - name: web-1
        role: web
        ip: 192.168.57.10
steps:
  - name: webserver
    description: Install nginx
    roles: [web]
    provision:
      - type: shell
        inline: apt-get install -y nginx
  - name: app
    description: Deploy app
    nodes: [web-1]
    provision:
      - type: shell
        inline: echo hello > /var/www/html/index.html
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Errantfile.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("writing test yaml: %v", err)
	}

	cfg, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error loading yaml with steps: %v", err)
	}

	resolved, err := cfg.ResolveCluster("dev")
	if err != nil {
		t.Fatalf("unexpected error resolving cluster: %v", err)
	}

	if resolved.SnapshotMode != "live" {
		t.Errorf("expected snapshot_mode 'live', got %q", resolved.SnapshotMode)
	}

	if resolved.ShutdownTimeout != 3*time.Minute {
		t.Errorf("expected shutdown_timeout 3m, got %v", resolved.ShutdownTimeout)
	}

	if len(resolved.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(resolved.Steps))
	}

	if resolved.Steps[0].Index != 1 || resolved.Steps[0].Name != "webserver" {
		t.Errorf("unexpected step 0: %+v", resolved.Steps[0])
	}

	if resolved.Steps[1].Index != 2 || resolved.Steps[1].Name != "app" {
		t.Errorf("unexpected step 1: %+v", resolved.Steps[1])
	}
}

func TestStepsValidationErrors(t *testing.T) {
	cases := []struct {
		name  string
		steps []StepConfig
	}{
		{
			name: "ReservedBase",
			steps: []StepConfig{
				{Name: "base", Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
			},
		},
		{
			name: "ReservedZero",
			steps: []StepConfig{
				{Name: "0", Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
			},
		},
		{
			name: "DuplicateName",
			steps: []StepConfig{
				{Name: "web", Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
				{Name: "web", Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
			},
		},
		{
			name: "UnknownNode",
			steps: []StepConfig{
				{Name: "web", Nodes: []string{"ghost"}, Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
			},
		},
		{
			name: "UnknownRole",
			steps: []StepConfig{
				{Name: "web", Roles: []string{"db"}, Provision: []ProvisionConfig{{Type: "shell", Inline: "true"}}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Version: "1",
				Project: "steps-err",
				Topologies: map[string]TopologyConfig{
					"dev": {
						Nodes: []NodeConfig{{Name: "n1", Role: "web", IP: "192.168.57.10"}},
					},
				},
				Steps: tc.steps,
			}

			if _, err := cfg.ResolveCluster("dev"); err == nil {
				t.Fatalf("expected error for case %s, got nil", tc.name)
			}
		})
	}
}

func TestStoragePoolResolution(t *testing.T) {
	origDetect := ResolveStoragePoolFunc
	ResolveStoragePoolFunc = func(pool string) string {
		if pool == "auto" {
			return "vmstore"
		}

		return pool
	}

	t.Cleanup(func() { ResolveStoragePoolFunc = origDetect })

	cfg := &Config{
		Version: "1",
		Project: "pool-test",
		Defaults: DefaultsConfig{
			Image:       "debian/trixie64",
			StoragePool: "auto",
		},
		Profiles: map[string]ProfileConfig{
			"cp": {
				Role: "control-plane",
				Disks: []DiskConfig{
					{Device: "vdb", Size: "20G"},
				},
			},
			"worker": {
				Role:        "worker",
				StoragePool: "fast-nvme",
			},
		},
		Topologies: map[string]TopologyConfig{
			"dev": {
				Nodes: []NodeConfig{
					{Name: "cp-1", Profile: "cp", IP: "192.168.56.10"},
					{Name: "w-1", Profile: "worker", IP: "192.168.56.21"},
					{Name: "w-2", Profile: "worker", StoragePool: "default", IP: "192.168.56.22"},
				},
			},
		},
	}

	cluster, err := cfg.ResolveCluster("dev")
	if err != nil {
		t.Fatalf("ResolveCluster failed: %v", err)
	}

	if cluster.Nodes[0].StoragePool != "vmstore" {
		t.Errorf("expected cp-1 pool 'vmstore', got %q", cluster.Nodes[0].StoragePool)
	}

	if cluster.Nodes[0].Disks[0].Pool != "vmstore" {
		t.Errorf("expected cp-1 secondary disk pool 'vmstore', got %q", cluster.Nodes[0].Disks[0].Pool)
	}

	if cluster.Nodes[1].StoragePool != "fast-nvme" {
		t.Errorf("expected w-1 pool 'fast-nvme', got %q", cluster.Nodes[1].StoragePool)
	}

	if cluster.Nodes[2].StoragePool != "default" {
		t.Errorf("expected w-2 pool 'default', got %q", cluster.Nodes[2].StoragePool)
	}
}

func TestLocalProvisionerSchemaValidation(t *testing.T) {
	yamlData := []byte(`
version: "1"
project: local-prov-test
defaults:
  box: ubuntu/noble64
topologies:
  dev:
    nodes:
      - name: node-1
        ip: 192.168.56.10
steps:
  - name: setup
    provision:
      - type: local
        dir: ../other-repo
        inline: ./scripts/run-playbook.sh -e preprod -p nfs.yml
        env:
          FOO: bar
`)

	if err := ValidateYAML(yamlData); err != nil {
		t.Fatalf("expected schema validation to pass for type: local, got: %v", err)
	}
}
