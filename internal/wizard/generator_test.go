package wizard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllArchetypesProduceValidYAML(t *testing.T) {
	archetypes := []string{"k8s-cluster", "k8s-ha", "single-vm", "custom"}

	for _, arch := range archetypes {
		t.Run(arch, func(t *testing.T) {
			ans := NewDefaultAnswers("test-proj")
			ans.ArchetypeID = arch
			ans.ControlNodes = 2
			ans.WorkerNodes = 3

			yamlBytes, err := RenderYAML(ans)
			if err != nil {
				t.Fatalf("failed to render YAML for archetype %s: %v", arch, err)
			}

			if len(yamlBytes) == 0 {
				t.Fatalf("rendered YAML was empty for archetype %s", arch)
			}
		})
	}
}

func TestWriteErrantfile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "Errantfile.yaml")

	ans := NewDefaultAnswers("temp-cluster")
	ans.OutputPath = target

	written, err := WriteErrantfile(ans)
	if err != nil {
		t.Fatalf("unexpected error writing Errantfile: %v", err)
	}

	if written != target {
		t.Errorf("expected written path %s, got %s", target, written)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed reading written file: %v", err)
	}

	if len(content) == 0 {
		t.Error("written Errantfile is empty")
	}
}
