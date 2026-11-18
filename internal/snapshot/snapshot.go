package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xenos76/errant/internal/config"
)

const (
	// SnapshotPrefix is the naming prefix for declarative errant-managed VM snapshots.
	SnapshotPrefix = "errant-step-"
	// UserSnapshotPrefix is the naming prefix for ad-hoc user-saved VM snapshots.
	UserSnapshotPrefix = "errant-user-"
	// StepKindDeclarative indicates the active checkpoint is a declarative Errantfile step.
	StepKindDeclarative = "step"
	// StepKindUser indicates the active checkpoint is a user-saved ad-hoc step.
	StepKindUser  = "user"
	stateFileName = "steps.json"
)

// Manager defines lifecycle snapshot operations decoupled from Pulumi/Terraform.
type Manager interface {
	Create(ctx context.Context, domainName, snapName, description, mode string) error
	Revert(ctx context.Context, domainName, snapName string) error
	Delete(ctx context.Context, domainName, snapName string) error
	List(ctx context.Context, domainName string) ([]string, error)
	CleanupAll(ctx context.Context, domainName, prefix string) error
}

// StepRecord captures metadata about a frozen lifecycle step snapshot.
type StepRecord struct {
	Index     int       `json:"index"`
	Name      string    `json:"name"`
	Snapshot  string    `json:"snapshot"`
	Hash      string    `json:"hash,omitempty"`
	Nodes     []string  `json:"nodes"`
	CreatedAt time.Time `json:"created_at"`
}

// CustomStepRecord captures metadata about a user-saved ad-hoc step snapshot.
type CustomStepRecord struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Snapshot    string    `json:"snapshot"`
	ParentStep  int       `json:"parent_step"`
	ParentName  string    `json:"parent_name,omitempty"`
	Nodes       []string  `json:"nodes"`
	CreatedAt   time.Time `json:"created_at"`
}

// State persists active step and snapshot metadata in .errant/steps.json.
type State struct {
	Topology        string                      `json:"topology"`
	CurrentKind     string                      `json:"current_kind,omitempty"`
	CurrentStep     int                         `json:"current_step"`
	CurrentStepName string                      `json:"current_step_name"`
	Steps           map[int]StepRecord          `json:"steps"`
	CustomSteps     map[string]CustomStepRecord `json:"custom_steps,omitempty"`
}

// StepSnapshotName returns the canonical snapshot identifier for a step index.
func StepSnapshotName(index int) string {
	return fmt.Sprintf("%s%d", SnapshotPrefix, index)
}

// UserSnapshotName returns the canonical snapshot identifier for a user-saved step.
func UserSnapshotName(name string) string {
	return UserSnapshotPrefix + name
}

// StepHash computes a deterministic SHA-256 digest of a step's definition.
func StepHash(step config.ResolvedStep) string {
	payload, err := json.Marshal(struct {
		Nodes     []string                 `json:"nodes"`
		Roles     []string                 `json:"roles"`
		Provision []config.ProvisionConfig `json:"provision"`
	}{
		Nodes:     step.Nodes,
		Roles:     step.Roles,
		Provision: step.Provision,
	})
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:8])
}

// StatePath returns the path to .errant/steps.json for a project directory.
func StatePath(projectDir string) string {
	return filepath.Join(projectDir, ".errant", stateFileName)
}

// StateExists reports whether .errant/steps.json exists for the project.
func StateExists(projectDir string) bool {
	_, err := os.Stat(StatePath(projectDir))

	return err == nil
}

// LoadState reads .errant/steps.json, returning an empty initialized State if missing.
func LoadState(projectDir string) (*State, error) {
	path := StatePath(projectDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{
				CurrentKind: StepKindDeclarative,
				Steps:       make(map[int]StepRecord),
				CustomSteps: make(map[string]CustomStepRecord),
			}, nil
		}

		return nil, fmt.Errorf("reading step state %s: %w", path, err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing step state %s: %w", path, err)
	}

	if st.CurrentKind == "" {
		st.CurrentKind = StepKindDeclarative
	}

	if st.Steps == nil {
		st.Steps = make(map[int]StepRecord)
	}

	if st.CustomSteps == nil {
		st.CustomSteps = make(map[string]CustomStepRecord)
	}

	return &st, nil
}

// SaveState writes step metadata to .errant/steps.json.
func SaveState(projectDir string, st *State) error {
	path := StatePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating state directory for %s: %w", path, err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding step state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing step state %s: %w", path, err)
	}

	return nil
}

// RemoveState deletes .errant/steps.json if present.
func RemoveState(projectDir string) error {
	path := StatePath(projectDir)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing step state %s: %w", path, err)
	}

	return nil
}
