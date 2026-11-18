package cache

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	startTime  time.Time
	lastPrint  time.Time
	lastPct    int
	isTTY      bool
	done       bool
}

func newProgressReader(r io.Reader, total int64) *progressReader {
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	now := time.Now()

	return &progressReader{
		reader:    r,
		total:     total,
		startTime: now,
		lastPrint: now,
		lastPct:   -1,
		isTTY:     isTTY,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		pr.updateProgress()
	}

	if err == io.EOF {
		pr.finish()
	}

	return n, err
}

func (pr *progressReader) updateProgress() {
	now := time.Now()
	if now.Sub(pr.lastPrint) < 150*time.Millisecond && pr.downloaded < pr.total {
		return
	}

	pr.lastPrint = now
	speed := pr.calculateSpeed(now)

	if pr.isTTY {
		pr.renderTTY(speed)
	} else {
		pr.renderNonTTY()
	}
}

func (pr *progressReader) calculateSpeed(now time.Time) string {
	elapsed := now.Sub(pr.startTime).Seconds()
	if elapsed <= 0.05 {
		return "-- MB/s"
	}

	bps := float64(pr.downloaded) / elapsed

	return FormatBytes(int64(bps)) + "/s"
}

func (pr *progressReader) renderTTY(speed string) {
	if pr.total > 0 {
		ratio := float64(pr.downloaded) / float64(pr.total)
		bar := renderBar(ratio, 25)
		pct := ratio * 100
		eta := pr.calculateETA()

		fmt.Printf("\r    Progress: [%s] %5.1f%% (%s / %s) %s %s   ",
			bar, pct, FormatBytes(pr.downloaded), FormatBytes(pr.total), speed, eta)
	} else {
		fmt.Printf("\r    Downloaded: %s (%s)   ", FormatBytes(pr.downloaded), speed)
	}
}

func (pr *progressReader) renderNonTTY() {
	if pr.total > 0 {
		pct := int((float64(pr.downloaded) / float64(pr.total)) * 100)
		if pct/10 != pr.lastPct/10 {
			pr.lastPct = pct
			fmt.Printf("==> [Errant] Download progress: %d%% (%s / %s)\n",
				pct, FormatBytes(pr.downloaded), FormatBytes(pr.total))
		}
	}
}

func (pr *progressReader) calculateETA() string {
	now := time.Now()
	elapsed := now.Sub(pr.startTime).Seconds()

	if elapsed <= 0.05 || pr.downloaded <= 0 {
		return "ETA --"
	}

	bps := float64(pr.downloaded) / elapsed
	if bps <= 0 {
		return "ETA --"
	}

	remainingBytes := pr.total - pr.downloaded
	if remainingBytes <= 0 {
		return "ETA 0s"
	}

	secs := int64(float64(remainingBytes) / bps)
	if secs >= 60 {
		return fmt.Sprintf("ETA %dm%02ds", secs/60, secs%60)
	}

	return fmt.Sprintf("ETA %ds", secs)
}

func (pr *progressReader) finish() {
	if pr.done {
		return
	}

	pr.done = true

	if pr.isTTY {
		if pr.total > 0 {
			bar := renderBar(1.0, 25)
			fmt.Printf("\r    Progress: [%s] 100.0%% (%s / %s) Complete!%s\n",
				bar, FormatBytes(pr.total), FormatBytes(pr.total), strings.Repeat(" ", 15))
		} else {
			fmt.Printf("\r    Downloaded: %s Complete!%s\n",
				FormatBytes(pr.downloaded), strings.Repeat(" ", 20))
		}
	} else if pr.total > 0 && pr.lastPct < 100 {
		fmt.Printf("==> [Errant] Download complete: %s\n", FormatBytes(pr.downloaded))
	}
}

func renderBar(ratio float64, width int) string {
	if ratio < 0 {
		ratio = 0
	}

	if ratio > 1 {
		ratio = 1
	}

	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}

	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// FormatBytes formats byte counts into human-readable strings (e.g. 1.5 MB, 2.50 GB).
func FormatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
