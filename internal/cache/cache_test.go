package cache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCacheDir(t *testing.T) {
	orig := os.Getenv("ERRANT_CACHE_DIR")
	defer func() { _ = os.Setenv("ERRANT_CACHE_DIR", orig) }()

	_ = os.Setenv("ERRANT_CACHE_DIR", "/custom/cache/dir")

	if got := GetCacheDir(); got != "/custom/cache/dir" {
		t.Errorf("expected /custom/cache/dir, got %s", got)
	}

	_ = os.Unsetenv("ERRANT_CACHE_DIR")

	origXDG := os.Getenv("XDG_CACHE_HOME")
	defer func() { _ = os.Setenv("XDG_CACHE_HOME", origXDG) }()

	_ = os.Setenv("XDG_CACHE_HOME", "/custom/xdg")

	if got := GetCacheDir(); got != "/custom/xdg/errant/images" {
		t.Errorf("expected /custom/xdg/errant/images, got %s", got)
	}
}

func TestResolveImageLocalFile(t *testing.T) {
	localPath := "/tmp/local.img"

	got, err := ResolveImage(localPath)
	if err != nil {
		t.Fatalf("ResolveImage failed: %v", err)
	}

	if got != localPath {
		t.Errorf("expected %s, got %s", localPath, got)
	}
}

func TestResolveImageRemote(t *testing.T) {
	tmpDir := t.TempDir()
	orig := os.Getenv("ERRANT_CACHE_DIR")

	defer func() { _ = os.Setenv("ERRANT_CACHE_DIR", orig) }()

	_ = os.Setenv("ERRANT_CACHE_DIR", tmpDir)

	// Start mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock image data"))
	}))
	defer server.Close()

	remoteURL := server.URL + "/test-distro.qcow2"

	cachedPath, err := ResolveImage(remoteURL)
	if err != nil {
		t.Fatalf("ResolveImage failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "test-distro.qcow2")
	if cachedPath != expectedFile {
		t.Errorf("expected %s, got %s", expectedFile, cachedPath)
	}

	// Second resolve should hit cache directly
	secondPath, err := ResolveImage(remoteURL)
	if err != nil {
		t.Fatalf("second ResolveImage failed: %v", err)
	}

	if secondPath != cachedPath {
		t.Errorf("expected cached path %s, got %s", cachedPath, secondPath)
	}
}
