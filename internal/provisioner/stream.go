package provisioner

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

// NodeLineWriter wraps an underlying io.Writer and formats incoming output
// line-by-line with node attribution and ANSI color highlights.
type NodeLineWriter struct {
	mu       sync.Mutex
	dest     io.Writer
	nodeName string
	isStderr bool
	buf      bytes.Buffer
}

// NewNodeLineWriter creates a new streaming line writer for a specific node.
func NewNodeLineWriter(dest io.Writer, nodeName string, isStderr bool) *NodeLineWriter {
	return &NodeLineWriter{
		dest:     dest,
		nodeName: nodeName,
		isStderr: isStderr,
	}
}

// Write processes incoming chunks, buffers partial lines, and flushes completed lines.
func (w *NodeLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(p)

	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			_, _ = w.buf.Write(p)

			break
		}

		_, _ = w.buf.Write(p[:idx])
		p = p[idx+1:]

		line := w.buf.String()
		w.buf.Reset()

		if err := w.emitLine(line); err != nil {
			return 0, err
		}
	}

	return total, nil
}

// Flush emits any remaining buffered line (e.g. without trailing newline).
func (w *NodeLineWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf.Len() > 0 {
		line := w.buf.String()
		w.buf.Reset()

		return w.emitLine(line)
	}

	return nil
}

func (w *NodeLineWriter) emitLine(rawLine string) error {
	line := strings.TrimRight(rawLine, "\r")
	if len(strings.TrimSpace(line)) == 0 {
		_, err := w.dest.Write([]byte("\n"))

		return err
	}

	// Check if this line is an Errant step header emitted by a guest script, e.g. "==> [7/7] Upgrading..."
	if strings.HasPrefix(line, "==>") {
		formatted := w.formatStepLine(line)
		_, err := w.dest.Write([]byte(formatted + "\n"))

		return err
	}

	// Regular command output
	var prefix string
	if w.isStderr {
		prefix = FormatNodeStderrPrefix(w.nodeName)
	} else {
		prefix = FormatNodePrefix(w.nodeName)
	}

	_, err := w.dest.Write([]byte(prefix + line + "\n"))

	return err
}

func (w *NodeLineWriter) formatStepLine(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "==>"))

	// Check if rest starts with a bracket tag like [7/7] or [common.sh]
	if strings.HasPrefix(rest, "[") {
		closeIdx := strings.Index(rest, "]")
		if closeIdx != -1 {
			tag := rest[1:closeIdx]
			content := strings.TrimSpace(rest[closeIdx+1:])
			styledTag := StyleStepTag("[" + tag + "]")

			return FormatStep(w.nodeName, styledTag+" "+content)
		}
	}

	return FormatStep(w.nodeName, rest)
}
