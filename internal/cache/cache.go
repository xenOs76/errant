package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xenos76/errant/internal/builder"
	"github.com/xenos76/errant/internal/config"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Minute,
}

// GetCacheDir returns the configured image cache directory.
func GetCacheDir() string {
	if envDir := os.Getenv("ERRANT_CACHE_DIR"); envDir != "" {
		return envDir
	}

	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "errant", "images")
	}

	if fi, err := os.Stat("/data/downloads/errant-images"); err == nil && fi.IsDir() {
		return "/data/downloads/errant-images"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".errant", "cache")
	}

	return filepath.Join(home, ".cache", "errant", "images")
}

// ResolveClusterImage resolves a base image reference which can be:
// 1. A declared image recipe key in declaredImages (built via builder)
// 2. A nix:// URI (built via builder)
// 3. A remote HTTP/HTTPS URL (downloaded and cached locally)
// 4. A local file path (returned directly).
func ResolveClusterImage(
	ctx context.Context,
	rawImage string,
	declaredImages map[string]config.ImageBuildConfig,
) (string, error) {
	if rawImage == "" {
		return "", nil
	}

	cacheDir := GetCacheDir()

	if imgCfg, exists := declaredImages[rawImage]; exists {
		return builder.BuildImage(ctx, rawImage, imgCfg, cacheDir)
	}

	if builder.IsNixURI(rawImage) {
		imgCfg, err := builder.ParseNixURI(rawImage)
		if err != nil {
			return "", fmt.Errorf("parsing nix image URI %q: %w", rawImage, err)
		}

		return builder.BuildImage(ctx, "", imgCfg, cacheDir)
	}

	if strings.HasPrefix(rawImage, "http://") || strings.HasPrefix(rawImage, "https://") {
		return resolveRemoteImage(rawImage, cacheDir)
	}

	return rawImage, nil
}

func resolveRemoteImage(rawImage, cacheDir string) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("creating image cache directory %s: %w", cacheDir, err)
	}

	u, err := url.Parse(rawImage)
	if err != nil {
		return "", fmt.Errorf("parsing image URL %s: %w", rawImage, err)
	}

	filename := filepath.Base(u.Path)
	if filename == "" || filename == "/" || filename == "." {
		hash := sha256.Sum256([]byte(rawImage))
		filename = fmt.Sprintf("image-%x.img", hash[:8])
	}

	targetPath := filepath.Join(cacheDir, filename)
	if fi, err := os.Stat(targetPath); err == nil && fi.Size() > 0 {
		return targetPath, nil
	}

	return downloadImage(rawImage, targetPath)
}

// ResolveImage checks if rawImage is a remote URL or Nix URI and resolves it.
// If already a local file path, it returns it directly.
func ResolveImage(rawImage string) (string, error) {
	return ResolveClusterImage(context.Background(), rawImage, nil)
}

func downloadImage(rawImage, targetPath string) (string, error) {
	fmt.Printf("==> [Errant] Downloading base image: %s\n", rawImage)
	fmt.Printf("    Destination: %s\n", targetPath)

	partPath := targetPath + ".part"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawImage, nil)
	if err != nil {
		return "", fmt.Errorf("creating request for %s: %w", rawImage, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading image %s: %w", rawImage, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading image %s returned HTTP status %d", rawImage, resp.StatusCode)
	}

	outFile, err := os.Create(partPath)
	if err != nil {
		return "", fmt.Errorf("creating temporary image file %s: %w", partPath, err)
	}

	pr := newProgressReader(resp.Body, resp.ContentLength)

	if _, err := io.Copy(outFile, pr); err != nil {
		_ = outFile.Close()
		_ = os.Remove(partPath)

		return "", fmt.Errorf("saving image to %s: %w", partPath, err)
	}

	pr.finish()

	if err := outFile.Close(); err != nil {
		_ = os.Remove(partPath)

		return "", fmt.Errorf("closing image file %s: %w", partPath, err)
	}

	if err := os.Rename(partPath, targetPath); err != nil {
		return "", fmt.Errorf("finalizing cached image %s: %w", targetPath, err)
	}

	fmt.Printf("✔ Cached base image: %s\n", targetPath)

	return targetPath, nil
}
