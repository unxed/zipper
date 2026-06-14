package main

import (
    "os"
	"fmt"
	"strings"
	"time"

	"github.com/unxed/zipper/archive"
)

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func startProgressBar(p archive.Progresser, totalBytes, totalEntries int64, op string) func() {
	if p == nil {
		return func() {}
	}
	done := make(chan struct{})
	start := time.Now()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var lastBytes int64
		var lastTime = start
		var emaSpeed float64

		for {
			select {
			case <-done:
				fmt.Print("\r" + strings.Repeat(" ", 100) + "\r")
				return
			case now := <-ticker.C:
				bytes, entries := p.Written()

				dt := now.Sub(lastTime).Seconds()
				if dt > 0 {
					instantSpeed := float64(bytes-lastBytes) / dt
					if emaSpeed == 0 {
						emaSpeed = instantSpeed
					} else {
						emaSpeed = 0.2*instantSpeed + 0.8*emaSpeed
					}
				}

				var status string
				if totalBytes > 0 {
					pct := float64(bytes) / float64(totalBytes) * 100
					if pct > 100 {
						pct = 100
					}

					var eta time.Duration
					if emaSpeed > 0 {
						eta = time.Duration(float64(totalBytes-bytes) / emaSpeed * float64(time.Second))
					}

					barLen := 20
					filled := int(float64(barLen) * pct / 100)
					if filled > barLen {
						filled = barLen
					}
					bar := strings.Repeat("=", filled) + strings.Repeat("-", barLen-filled)
					if filled > 0 && filled < barLen {
						bar = strings.Repeat("=", filled-1) + ">" + strings.Repeat("-", barLen-filled)
					}

					status = fmt.Sprintf("%s [%s] %5.1f%% | %s / %s | %s/s | ETA: %s | %d/%d files",
						op, bar, pct, formatBytes(bytes), formatBytes(totalBytes),
						formatBytes(int64(emaSpeed)), formatDuration(eta),
						entries, totalEntries)
				} else {
					status = fmt.Sprintf("%s | %s | %s/s | %d files",
						op, formatBytes(bytes), formatBytes(int64(emaSpeed)), entries)
				}

				if len(status) > 100 {
					status = status[:100]
				}
				fmt.Fprintf(os.Stderr, "\r%-100s", status)

				lastBytes = bytes
				lastTime = now
			}
		}
	}()

	return func() {
		close(done)
	}
}