package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.qcow2", true},
		{"disk.img", true},
		{"installer.iso", true},
		{"download.part", false},
		{"README.md", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isImageFile(tt.filename)
		if got != tt.expected {
			t.Errorf("isImageFile(%q) = %v, want %v", tt.filename, got, tt.expected)
		}
	}
}

func TestImageListCmd(t *testing.T) {
	tmpCache := t.TempDir()

	origCache := os.Getenv("ERRANT_CACHE_DIR")
	defer func() { _ = os.Setenv("ERRANT_CACHE_DIR", origCache) }()

	_ = os.Setenv("ERRANT_CACHE_DIR", tmpCache)

	// Create fake images
	f1 := filepath.Join(tmpCache, "fake-ubuntu.qcow2")
	if err := os.WriteFile(f1, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("writing fake image: %v", err)
	}

	f2 := filepath.Join(tmpCache, "fake-debian.img")
	if err := os.WriteFile(f2, make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("writing fake image: %v", err)
	}

	rootCmd := NewRootCmd()

	var buf bytes.Buffer

	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"image", "list"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("executing image list failed: %v", err)
	}
}

func TestImagePruneCmd(t *testing.T) {
	tmpCache := t.TempDir()

	origCache := os.Getenv("ERRANT_CACHE_DIR")
	defer func() { _ = os.Setenv("ERRANT_CACHE_DIR", origCache) }()

	_ = os.Setenv("ERRANT_CACHE_DIR", tmpCache)

	partFile := filepath.Join(tmpCache, "download.qcow2.part")
	if err := os.WriteFile(partFile, []byte("temp"), 0o644); err != nil {
		t.Fatalf("writing part file: %v", err)
	}

	imgFile := filepath.Join(tmpCache, "complete.qcow2")
	if err := os.WriteFile(imgFile, []byte("ready"), 0o644); err != nil {
		t.Fatalf("writing img file: %v", err)
	}

	// First prune without --all should only delete .part
	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"image", "prune"})

	if err := cmd1.Execute(); err != nil {
		t.Fatalf("executing image prune: %v", err)
	}

	if _, err := os.Stat(partFile); !os.IsNotExist(err) {
		t.Error("expected part file to be removed")
	}

	if _, err := os.Stat(imgFile); err != nil {
		t.Errorf("expected complete image to remain, got err: %v", err)
	}

	// Prune with --all should remove complete image
	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"image", "prune", "--all"})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("executing image prune --all: %v", err)
	}

	if _, err := os.Stat(imgFile); !os.IsNotExist(err) {
		t.Error("expected complete image to be removed with --all")
	}
}

func TestImageBuildCmd_MissingRecipe(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "Errantfile.yaml")

	content := `
version: "2"
project: test-image-build
network:
  subnet: "192.168.56.0/24"
topologies:
  standard:
    nodes:
      - name: vm-1
        ip: 192.168.56.10
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing Errantfile: %v", err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"image", "build", "nonexistent-recipe", "-c", cfgPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent recipe, got nil")
	}
}
