package snapshot

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/xenos76/errant/internal/config"
)

func TestStatePersistence(t *testing.T) {
	dir := t.TempDir()

	if StateExists(dir) {
		t.Fatal("expected StateExists to be false initially")
	}

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState on missing file failed: %v", err)
	}

	if len(st.Steps) != 0 {
		t.Fatalf("expected empty steps map, got %d", len(st.Steps))
	}

	st.Topology = "dev"
	st.CurrentKind = StepKindDeclarative
	st.CurrentStep = 1
	st.CurrentStepName = "webserver"
	st.Steps[1] = StepRecord{
		Index:    1,
		Name:     "webserver",
		Snapshot: StepSnapshotName(1),
		Hash:     StepHash(config.ResolvedStep{Name: "webserver"}),
		Nodes:    []string{"debian-vm"},
	}
	st.CustomSteps["exp-1"] = CustomStepRecord{
		Name:        "exp-1",
		Description: "manual experiment",
		Snapshot:    UserSnapshotName("exp-1"),
		ParentStep:  1,
		ParentName:  "webserver",
		Nodes:       []string{"debian-vm"},
	}

	if err := SaveState(dir, st); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	if !StateExists(dir) {
		t.Fatal("expected StateExists to be true after SaveState")
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.CurrentStep != 1 || loaded.CurrentStepName != "webserver" {
		t.Errorf("unexpected loaded state: %+v", loaded)
	}

	if rec, ok := loaded.CustomSteps["exp-1"]; !ok || rec.Snapshot != "errant-user-exp-1" {
		t.Errorf("unexpected custom steps loaded: %+v", loaded.CustomSteps)
	}

	if err := RemoveState(dir); err != nil {
		t.Fatalf("RemoveState failed: %v", err)
	}

	if StateExists(dir) {
		t.Fatal("expected StateExists to be false after RemoveState")
	}
}

func withMockedVirsh(t *testing.T, recorded *[][]string) func() {
	t.Helper()

	origExec := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		*recorded = append(*recorded, append([]string{name}, args...))
		joined := strings.Join(args, " ")

		switch {
		case strings.Contains(joined, "domstate"):
			return exec.CommandContext(ctx, "printf", "shut off\n")
		case strings.Contains(joined, "domblklist"):
			out := "Type Device Target Source\n----\nfile disk vda /tmp/root.qcow2\nfile cdrom hda /tmp/ci.iso\n"

			return exec.CommandContext(ctx, "printf", "%s", out)
		case strings.Contains(joined, "snapshot-list"):
			return exec.CommandContext(ctx, "printf", "errant-step-0\nerrant-step-1\nmanual-snap\n")
		default:
			return exec.CommandContext(ctx, "true")
		}
	}

	return func() { execCommand = origExec }
}

func TestLibvirtManagerCreateAndRevert(t *testing.T) {
	var recorded [][]string

	cleanup := withMockedVirsh(t, &recorded)
	defer cleanup()

	mgr := NewLibvirtManager()
	ctx := context.Background()

	if err := mgr.Create(ctx, "proj-vm", "errant-step-0", "base", "disk"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if !hasCommandArg(recorded, "snapshot-create-as", "--diskspec hda,snapshot=no") {
		t.Errorf("expected --diskspec hda,snapshot=no for cdrom in snapshot-create-as, got: %v", recorded)
	}

	if err := mgr.Revert(ctx, "proj-vm", "errant-step-0"); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}
}

func TestLibvirtManagerListAndCleanup(t *testing.T) {
	var recorded [][]string

	cleanup := withMockedVirsh(t, &recorded)
	defer cleanup()

	mgr := NewLibvirtManager()
	ctx := context.Background()

	snaps, err := mgr.List(ctx, "proj-vm")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots, got %v", snaps)
	}

	recorded = nil

	if err := mgr.CleanupAll(ctx, "proj-vm", SnapshotPrefix); err != nil {
		t.Fatalf("CleanupAll failed: %v", err)
	}

	if got := countCommands(recorded, "snapshot-delete"); got != 2 {
		t.Errorf("expected 2 prefixed snapshots deleted, got %d", got)
	}
}

func hasCommandArg(recorded [][]string, substrA, substrB string) bool {
	for _, cmd := range recorded {
		joined := strings.Join(cmd, " ")
		if strings.Contains(joined, substrA) && strings.Contains(joined, substrB) {
			return true
		}
	}

	return false
}

func countCommands(recorded [][]string, substr string) int {
	count := 0

	for _, cmd := range recorded {
		if strings.Contains(strings.Join(cmd, " "), substr) {
			count++
		}
	}

	return count
}
