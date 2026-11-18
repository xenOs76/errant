package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenos76/errant/internal/config"
)

func TestIsNixURI(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"nix://channel:nixos-26.05?format=qcow", true},
		{"nix://flake:nixpkgs#nixos-26.05", true},
		{"https://cloud-images.ubuntu.com/noble.img", false},
		{"/path/to/image.qcow2", false},
		{"ubuntu/noble64", false},
	}

	for _, tt := range tests {
		got := IsNixURI(tt.input)
		if got != tt.want {
			t.Errorf("IsNixURI(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseNixURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantChannel string
		wantFlake   string
		wantFormat  string
	}{
		{
			name:        "ChannelWithFormat",
			uri:         "nix://channel:nixos-26.05?format=qcow",
			wantChannel: "nixos-26.05",
			wantFormat:  "qcow",
		},
		{
			name:       "FlakeURI",
			uri:        "nix://flake:nixpkgs#nixos-26.05?format=openstack",
			wantFlake:  "nixpkgs#nixos-26.05",
			wantFormat: "openstack",
		},
		{
			name:        "BareChannel",
			uri:         "nix://nixos-24.11",
			wantChannel: "nixos-24.11",
			wantFormat:  "qcow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseNixURI(tc.uri)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.Channel != tc.wantChannel {
				t.Errorf("got channel %q, want %q", cfg.Channel, tc.wantChannel)
			}

			if cfg.Flake != tc.wantFlake {
				t.Errorf("got flake %q, want %q", cfg.Flake, tc.wantFlake)
			}

			if cfg.Format != tc.wantFormat {
				t.Errorf("got format %q, want %q", cfg.Format, tc.wantFormat)
			}
		})
	}
}

func TestCacheKey(t *testing.T) {
	cfg1 := config.ImageBuildConfig{
		Channel:  "nixos-26.05",
		Format:   "qcow",
		Packages: []string{"vim", "git"},
	}

	cfg2 := config.ImageBuildConfig{
		Channel:  "nixos-26.05",
		Format:   "qcow",
		Packages: []string{"vim", "git"},
	}

	cfg3 := config.ImageBuildConfig{
		Channel:  "nixos-26.05",
		Format:   "openstack",
		Packages: []string{"vim", "git"},
	}

	key1 := CacheKey("custom-node", cfg1)
	key2 := CacheKey("custom-node", cfg2)
	key3 := CacheKey("custom-node", cfg3)

	if key1 != key2 {
		t.Errorf("identical configs should produce identical cache keys: %s vs %s", key1, key2)
	}

	if key1 == key3 {
		t.Errorf("different formats should produce different cache keys: %s vs %s", key1, key3)
	}

	if !strings.HasSuffix(key1, ".qcow2") {
		t.Errorf("cache key should end with .qcow2, got %s", key1)
	}
}

func TestGenerateNixConfig(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "configuration.nix")

	cfg := config.ImageBuildConfig{
		Packages: []string{"vim", "socat"},
		Inline:   "networking.firewall.enable = false;",
	}

	if err := generateNixConfig(targetPath, cfg); err != nil {
		t.Fatalf("generateNixConfig failed: %v", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("reading generated config: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, "errant-base.nix") {
		t.Error("generated config missing base profile import")
	}

	if !strings.Contains(str, "vim") || !strings.Contains(str, "socat") {
		t.Error("generated config missing packages")
	}

	if !strings.Contains(str, "networking.firewall.enable = false;") {
		t.Error("generated config missing inline configuration")
	}
}
