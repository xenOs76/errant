package cache

import (
	"bytes"
	"io"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{104857600, "100.0 MB"},
		{1073741824, "1.00 GB"},
		{2684354560, "2.50 GB"},
	}

	for _, tc := range tests {
		got := FormatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("FormatBytes(%d) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestRenderBar(t *testing.T) {
	tests := []struct {
		ratio    float64
		width    int
		expected string
	}{
		{0.0, 10, "░░░░░░░░░░"},
		{0.5, 10, "█████░░░░░"},
		{1.0, 10, "██████████"},
		{-0.1, 5, "░░░░░"},
		{1.5, 5, "█████"},
	}

	for _, tc := range tests {
		got := renderBar(tc.ratio, tc.width)
		if got != tc.expected {
			t.Errorf("renderBar(%f, %d) = %q, expected %q", tc.ratio, tc.width, got, tc.expected)
		}
	}
}

func TestProgressReader(t *testing.T) {
	data := []byte("hello world this is a test payload for download progress reader")
	total := int64(len(data))
	src := bytes.NewReader(data)

	pr := newProgressReader(src, total)
	pr.isTTY = false

	var dest bytes.Buffer

	n, err := io.Copy(&dest, pr)
	if err != nil {
		t.Fatalf("io.Copy failed: %v", err)
	}

	if n != total {
		t.Errorf("copied bytes = %d, expected %d", n, total)
	}

	if pr.downloaded != total {
		t.Errorf("pr.downloaded = %d, expected %d", pr.downloaded, total)
	}

	if dest.String() != string(data) {
		t.Errorf("payload mismatch: got %q, expected %q", dest.String(), string(data))
	}

	pr.finish()
}
