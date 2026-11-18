package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	virshURI        = "qemu:///system"
	shutdownTimeout = 60 * time.Second
)

var execCommand = exec.CommandContext

// LibvirtManager manages VM snapshots via virsh without coupling to Pulumi.
type LibvirtManager struct {
	Stdout io.Writer
	Stderr io.Writer
}

// NewLibvirtManager creates a Libvirt snapshot manager writing command output to stdout/stderr.
func NewLibvirtManager() *LibvirtManager {
	return &LibvirtManager{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Create freezes the domain into a named snapshot.
// In "disk" mode, a running domain is gracefully shut down before snapshotting and restarted afterward.
func (m *LibvirtManager) Create(
	ctx context.Context,
	domainName, snapName, description, mode string,
) error {
	if mode == "" {
		mode = "disk"
	}

	wasRunning := false

	if mode == "disk" {
		state, err := m.domainState(ctx, domainName)
		if err == nil && state == "running" {
			wasRunning = true

			if err := m.shutdownAndWait(ctx, domainName); err != nil {
				return fmt.Errorf("shutting down %s before snapshot %s: %w", domainName, snapName, err)
			}
		}
	}

	args := []string{
		"-c", virshURI,
		"snapshot-create-as", domainName, snapName,
		"--description", description,
		"--atomic",
	}

	excludedDisks, err := m.nonSnapshotDisks(ctx, domainName)
	if err == nil {
		for _, dev := range excludedDisks {
			args = append(args, "--diskspec", dev+",snapshot=no")
		}
	}

	cmd := execCommand(ctx, "virsh", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"creating snapshot %s on %s: %s: %w",
			snapName, domainName, strings.TrimSpace(string(out)), err,
		)
	}

	if wasRunning {
		if err := m.startDomain(ctx, domainName); err != nil {
			return fmt.Errorf("restarting %s after snapshot %s: %w", domainName, snapName, err)
		}
	}

	return nil
}

// Revert restores a domain to the specified snapshot and ensures it is running.
func (m *LibvirtManager) Revert(ctx context.Context, domainName, snapName string) error {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "snapshot-revert", domainName, snapName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"reverting %s to snapshot %s: %s: %w",
			domainName, snapName, strings.TrimSpace(string(out)), err,
		)
	}

	state, err := m.domainState(ctx, domainName)
	if err == nil && state != "running" {
		if err := m.startDomain(ctx, domainName); err != nil {
			return fmt.Errorf("starting %s after revert to %s: %w", domainName, snapName, err)
		}
	}

	return nil
}

// Delete removes a named snapshot from a domain.
func (*LibvirtManager) Delete(ctx context.Context, domainName, snapName string) error {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "snapshot-delete", domainName, snapName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"deleting snapshot %s on %s: %s: %w",
			snapName, domainName, strings.TrimSpace(string(out)), err,
		)
	}

	return nil
}

// List returns all snapshot names currently present on a domain.
func (*LibvirtManager) List(ctx context.Context, domainName string) ([]string, error) {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "snapshot-list", domainName, "--name")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("listing snapshots on %s: %s: %w", domainName, strings.TrimSpace(string(out)), err)
	}

	var snaps []string

	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			snaps = append(snaps, trimmed)
		}
	}

	return snaps, nil
}

// CleanupAll deletes all snapshots on domainName whose names start with prefix.
// If the domain does not exist or has no snapshots, it returns nil.
func (m *LibvirtManager) CleanupAll(ctx context.Context, domainName, prefix string) error {
	snaps, err := m.List(ctx, domainName)
	if err != nil {
		return nil
	}

	for _, snap := range snaps {
		if prefix == "" || strings.HasPrefix(snap, prefix) {
			if err := m.Delete(ctx, domainName, snap); err != nil {
				return err
			}
		}
	}

	return nil
}

func (*LibvirtManager) domainState(ctx context.Context, domainName string) (string, error) {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "domstate", domainName)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (*LibvirtManager) startDomain(ctx context.Context, domainName string) error {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "start", domainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("virsh start %s: %s: %w", domainName, strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (m *LibvirtManager) shutdownAndWait(ctx context.Context, domainName string) error {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "shutdown", domainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("virsh shutdown %s: %s: %w", domainName, strings.TrimSpace(string(out)), err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		state, err := m.domainState(waitCtx, domainName)
		if err == nil && state == "shut off" {
			return nil
		}

		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for %s to shut off: %w", domainName, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func (*LibvirtManager) nonSnapshotDisks(ctx context.Context, domainName string) ([]string, error) {
	cmd := execCommand(ctx, "virsh", "-c", virshURI, "domblklist", domainName, "--details")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var targets []string

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Columns: Type | Device | Target | Source
		if fields[0] == "Type" || strings.HasPrefix(fields[0], "-") {
			continue
		}

		device := fields[1]
		target := fields[2]

		if device == "cdrom" || device == "floppy" {
			targets = append(targets, target)
		}
	}

	return targets, nil
}
