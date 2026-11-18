package provisioner

import (
	"bytes"
	"strings"
	"testing"
)

func TestNodeLineWriter_StepLine(t *testing.T) {
	var buf bytes.Buffer

	w := NewNodeLineWriter(&buf, "kube-cp", false)

	input := "==> [7/7] Upgrading system packages...\n"

	n, err := w.Write([]byte(input))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(input) {
		t.Fatalf("expected write count %d, got %d", len(input), n)
	}

	out := buf.String()
	if !strings.Contains(out, "kube-cp") {
		t.Errorf("expected node name 'kube-cp' in output, got: %s", out)
	}

	if !strings.Contains(out, "[7/7]") {
		t.Errorf("expected tag '[7/7]' in output, got: %s", out)
	}

	if !strings.Contains(out, "Upgrading system packages...") {
		t.Errorf("expected step text in output, got: %s", out)
	}
}

func TestNodeLineWriter_RegularLine(t *testing.T) {
	var buf bytes.Buffer

	w := NewNodeLineWriter(&buf, "worker-1", false)

	input := "Setting up package xyz\n"

	_, err := w.Write([]byte(input))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "worker-1:") {
		t.Errorf("expected prefix 'worker-1:' in output, got: %s", out)
	}

	if !strings.Contains(out, "Setting up package xyz") {
		t.Errorf("expected text in output, got: %s", out)
	}
}

func TestNodeLineWriter_StderrLine(t *testing.T) {
	var buf bytes.Buffer

	w := NewNodeLineWriter(&buf, "worker-2", true)

	input := "warning: deprecated key\n"

	_, err := w.Write([]byte(input))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "worker-2:") {
		t.Errorf("expected prefix 'worker-2:' in output, got: %s", out)
	}

	if !strings.Contains(out, "warning: deprecated key") {
		t.Errorf("expected warning in output, got: %s", out)
	}
}

func TestNodeLineWriter_ChunkedWrites(t *testing.T) {
	var buf bytes.Buffer

	w := NewNodeLineWriter(&buf, "node-1", false)

	_, _ = w.Write([]byte("First half of "))

	if buf.Len() != 0 {
		t.Errorf("expected nothing emitted yet, got: %s", buf.String())
	}

	_, _ = w.Write([]byte("line\nSecond line without newline"))

	out := buf.String()
	if !strings.Contains(out, "First half of line") {
		t.Errorf("expected completed line in output, got: %s", out)
	}

	if strings.Contains(out, "Second line") {
		t.Errorf("did not expect uncompleted line before flush, got: %s", out)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	finalOut := buf.String()
	if !strings.Contains(finalOut, "Second line without newline") {
		t.Errorf("expected flushed line in final output, got: %s", finalOut)
	}
}
