package pulumi

import (
	"context"
	"testing"

	"github.com/xenos76/errant/internal/config"
)

func TestRunnerGetDomainStatus(t *testing.T) {
	runner := NewRunner(t.TempDir(), "test-proj")
	ctx := context.Background()

	// An uncreated or destroyed domain should report "not created"
	status := runner.GetDomainStatus(ctx, "nonexistent-domain-test-xyz")
	if status != "not created" {
		t.Errorf("expected 'not created', got: %q", status)
	}
}

func TestRunnerStartNoOpOnUncreated(t *testing.T) {
	runner := NewRunner(t.TempDir(), "test-proj")
	ctx := context.Background()

	cluster := &config.ResolvedCluster{
		Project: "test-proj",
		Nodes: []config.ResolvedNode{
			{Name: "nonexistent-node-1"},
		},
	}

	// For a domain that is "not created", Start should not attempt to start it and should return nil.
	if err := runner.Start(ctx, cluster); err != nil {
		t.Fatalf("expected nil error on uncreated domains, got: %v", err)
	}
}
