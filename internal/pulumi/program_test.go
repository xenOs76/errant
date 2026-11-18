package pulumi

import (
	"context"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/xenos76/errant/internal/config"
)

func TestMain(m *testing.M) {
	origResolve := ResolveImageFunc
	ResolveImageFunc = func(_ context.Context, img string, _ map[string]config.ImageBuildConfig) (string, error) {
		return img, nil
	}

	defer func() { ResolveImageFunc = origResolve }()

	m.Run()
}

type clusterMocks struct {
	mu        sync.Mutex
	resources []string
}

func (*clusterMocks) Call(_ pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func (m *clusterMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resources = append(m.resources, args.TypeToken+":"+args.Name)

	outputs := args.Inputs.Copy()

	return args.Name + "-id", outputs, nil
}

func newTestCluster() *config.ResolvedCluster {
	return &config.ResolvedCluster{
		Project:  "test-proj",
		Topology: "standard",
		Network: config.ResolvedNetwork{
			Mode:   "nat",
			Subnet: "192.168.56.0/24",
			DHCP:   true,
		},
		Nodes: []config.ResolvedNode{
			{
				Name:         "cp-1",
				Role:         "control-plane",
				MemoryMB:     4096,
				CPUs:         2,
				RootDiskSize: "25G",
				Image:        "https://example.com/test.qcow2",
				IP:           "192.168.56.10",
				Disks: []config.DiskConfig{
					{
						Device: "vdb",
						Size:   "10G",
						Format: "qcow2",
						Pool:   "default",
					},
				},
				SSH: config.SSHConfig{
					User: "vagrant",
				},
			},
			{
				Name:         "worker-1",
				Role:         "worker",
				MemoryMB:     2048,
				CPUs:         2,
				RootDiskSize: "20G",
				Image:        "https://example.com/test.qcow2",
				IP:           "192.168.56.21",
				SSH: config.SSHConfig{
					User: "vagrant",
				},
			},
		},
	}
}

func TestNewClusterProgram(t *testing.T) {
	cluster := newTestCluster()
	mocks := &clusterMocks{}

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		program := NewClusterProgram(cluster)

		return program(ctx)
	}, pulumi.WithMocks("test-project", "test-stack", mocks))
	if err != nil {
		t.Fatalf("program execution failed: %v", err)
	}

	mocks.mu.Lock()
	defer mocks.mu.Unlock()

	expectedTypes := []string{
		"libvirt:index/network:Network",
		"libvirt:index/volume:Volume",
		"libvirt:index/cloudInitDisk:CloudInitDisk",
		"libvirt:index/domain:Domain",
	}

	for _, expected := range expectedTypes {
		found := false

		for _, r := range mocks.resources {
			if len(r) >= len(expected) && r[:len(expected)] == expected {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("expected resource of type %s in mock resources, got: %v", expected, mocks.resources)
		}
	}
}

func TestRunnerInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewRunner(tmpDir, "test-proj")

	if runner.ProjectName != "test-proj" {
		t.Errorf("expected ProjectName 'test-proj', got: %s", runner.ProjectName)
	}

	if runner.BackendURL != "file://"+tmpDir+"/.errant/state" {
		t.Errorf("expected BackendURL 'file://%s/.errant/state', got: %s", tmpDir, runner.BackendURL)
	}
}
