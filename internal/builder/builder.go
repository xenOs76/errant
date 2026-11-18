package builder

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xenos76/errant/internal/config"
)

//go:embed embed/errant-base.nix
var defaultBaseProfile string

// IsNixURI checks if an image reference is a nix:// URI.
func IsNixURI(ref string) bool {
	return strings.HasPrefix(ref, "nix://")
}

// ParseNixURI extracts build parameters from a nix:// URI.
// Example: nix://channel:nixos-26.05?format=qcow
// Example: nix://flake:nixpkgs#nixos-26.05?format=qcow
func ParseNixURI(rawURI string) (config.ImageBuildConfig, error) {
	if !IsNixURI(rawURI) {
		return config.ImageBuildConfig{}, fmt.Errorf("not a valid nix URI: %q", rawURI)
	}

	cfg := config.ImageBuildConfig{
		Builder: "nixos-generators",
		Format:  "qcow",
	}

	trimmed := strings.TrimPrefix(rawURI, "nix://")

	pathPart, queryPart, found := strings.Cut(trimmed, "?")
	if !found {
		pathPart = trimmed
	}

	if queryPart != "" {
		values, err := url.ParseQuery(queryPart)
		if err != nil {
			return config.ImageBuildConfig{}, fmt.Errorf("parsing nix URI query %q: %w", queryPart, err)
		}

		if fmtParam := values.Get("format"); fmtParam != "" {
			cfg.Format = fmtParam
		}
	}

	switch {
	case strings.HasPrefix(pathPart, "channel:"):
		cfg.Channel = strings.TrimPrefix(pathPart, "channel:")
	case strings.HasPrefix(pathPart, "flake:"):
		cfg.Flake = strings.TrimPrefix(pathPart, "flake:")
	default:
		cfg.Channel = pathPart
	}

	return cfg, nil
}

// CacheKey computes a deterministic cache filename for an image build recipe.
func CacheKey(name string, cfg config.ImageBuildConfig) string {
	format := cfg.Format
	if format == "" {
		format = "qcow"
	}

	h := sha256.New()
	_, _ = h.Write([]byte(defaultBaseProfile))
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte(cfg.Builder))
	_, _ = h.Write([]byte(cfg.Channel))
	_, _ = h.Write([]byte(cfg.Flake))
	_, _ = h.Write([]byte(format))
	_, _ = h.Write([]byte(cfg.Inline))

	for _, p := range cfg.Packages {
		_, _ = h.Write([]byte(p))
	}

	for _, m := range cfg.Modules {
		_, _ = h.Write([]byte(m))
	}

	hash := hex.EncodeToString(h.Sum(nil))[:12]

	base := name
	if base == "" {
		base = "nixos"
		if cfg.Channel != "" {
			base = cfg.Channel
		}
	}

	base = strings.ReplaceAll(base, "/", "-")

	return fmt.Sprintf("%s-%s-%s.qcow2", base, format, hash)
}

// BuildImage builds a declarative NixOS image and caches it in cacheDir.
func BuildImage(ctx context.Context, name string, cfg config.ImageBuildConfig, cacheDir string) (string, error) {
	if _, err := exec.LookPath("nix"); err != nil {
		return "", fmt.Errorf(
			"nix command not found in PATH: installing Nix is required to generate image %q. "+
				"Install Nix or specify a prebuilt QCOW2 URL instead: %w", name, err,
		)
	}

	targetFilename := CacheKey(name, cfg)
	targetPath := filepath.Join(cacheDir, targetFilename)

	if fi, err := os.Stat(targetPath); err == nil && fi.Size() > 0 {
		return targetPath, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("creating image cache directory %s: %w", cacheDir, err)
	}

	tmpDir, err := os.MkdirTemp("", "errant-img-build-*")
	if err != nil {
		return "", fmt.Errorf("creating build temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configFile := filepath.Join(tmpDir, "configuration.nix")
	if err := generateNixConfig(configFile, cfg); err != nil {
		return "", fmt.Errorf("generating build config for %s: %w", name, err)
	}

	format := cfg.Format
	if format == "" {
		format = "qcow"
	}

	fmt.Printf("==> [Errant] Generating base image: %s (format: %s)\n", name, format)
	fmt.Printf("    Destination: %s\n", targetPath)

	outLink := filepath.Join(tmpDir, "result")
	args := []string{
		"run", "--extra-experimental-features", "nix-command flakes",
		"nixpkgs#nixos-generators", "--",
		"--format", format,
		"-c", configFile,
		"-o", outLink,
	}

	if cfg.Channel != "" {
		args = append(args, "-I", "nixpkgs=channel:"+cfg.Channel)
	}

	cmd := exec.CommandContext(ctx, "nix", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nixos-generators build failed for image %s: %w", name, err)
	}

	resolvedArtifact, err := resolveBuiltArtifact(outLink)
	if err != nil {
		return "", err
	}

	if err := copyFile(resolvedArtifact, targetPath); err != nil {
		return "", fmt.Errorf("caching built image %s to %s: %w", resolvedArtifact, targetPath, err)
	}

	fmt.Printf("✔ Generated and cached base image: %s\n", targetPath)

	return targetPath, nil
}

func generateNixConfig(targetPath string, cfg config.ImageBuildConfig) error {
	baseFile := filepath.Join(filepath.Dir(targetPath), "errant-base.nix")
	if err := os.WriteFile(baseFile, []byte(defaultBaseProfile), 0o644); err != nil {
		return fmt.Errorf("writing errant-base.nix: %w", err)
	}

	var sb strings.Builder

	sb.WriteString("{\n")
	sb.WriteString("  imports = [\n")
	sb.WriteString("    ./errant-base.nix\n")

	for _, mod := range cfg.Modules {
		abs, err := filepath.Abs(mod)
		if err == nil {
			fmt.Fprintf(&sb, "    %s\n", abs)
		} else {
			fmt.Fprintf(&sb, "    ./%s\n", mod)
		}
	}

	sb.WriteString("  ];\n\n")

	if len(cfg.Packages) > 0 {
		sb.WriteString("  environment.systemPackages = with import <nixpkgs> {}; [\n")

		for _, pkg := range cfg.Packages {
			fmt.Fprintf(&sb, "    %s\n", pkg)
		}

		sb.WriteString("  ];\n\n")
	}

	if cfg.Inline != "" {
		sb.WriteString("  # Inline configuration\n")
		sb.WriteString(cfg.Inline)
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")

	return os.WriteFile(targetPath, []byte(sb.String()), 0o644)
}

func resolveBuiltArtifact(outLink string) (string, error) {
	fi, err := os.Stat(outLink)
	if err != nil {
		return "", fmt.Errorf("accessing build outlink %s: %w", outLink, err)
	}

	if !fi.IsDir() {
		return outLink, nil
	}

	entries, err := os.ReadDir(outLink)
	if err != nil {
		return "", fmt.Errorf("reading build output directory %s: %w", outLink, err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".qcow2") || strings.HasSuffix(entry.Name(), ".img") {
			return filepath.Join(outLink, entry.Name()), nil
		}
	}

	if len(entries) > 0 {
		return filepath.Join(outLink, entries[0].Name()), nil
	}

	return "", fmt.Errorf("no image file found inside build output directory %s", outLink)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}
