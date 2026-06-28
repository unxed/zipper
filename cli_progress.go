package main

import (
	"fmt"
	"os"
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
	if d < 0 {
		d = 0
	}
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

func printStatus(bytes, entries, totalBytes, totalEntries int64, op string, speed float64, isFinal bool) {
	var status string
	if totalBytes > 0 {
		pct := float64(bytes) / float64(totalBytes) * 100
		if pct > 100 {
			pct = 100
		}
		if isFinal {
			pct = 100
			bytes = totalBytes
			entries = totalEntries
		}

		var etaStr string
		if isFinal {
			etaStr = "00:00"
		} else if speed > 1024 { // Only show ETA if speed is reasonable (> 1 KB/s)
			etaSecs := float64(totalBytes-bytes) / speed
			if etaSecs >= 0 && etaSecs < 3600*24*365 { // Less than 1 year
				etaStr = formatDuration(time.Duration(etaSecs * float64(time.Second)))
			} else {
				etaStr = "--:--"
			}
		} else {
			etaStr = "--:--"
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
			formatBytes(int64(speed)), etaStr,
			entries, totalEntries)
	} else {
		status = fmt.Sprintf("%s | %s | %s/s | %d files",
			op, formatBytes(bytes), formatBytes(int64(speed)), entries)
	}

	fmt.Fprintf(os.Stderr, "\r\033[K%s", status)
}

func startProgressBar(p archive.Progresser, totalBytes, totalEntries int64, op string) func() {
	if p == nil {
		return func() {}
	}
	fmt.Fprint(os.Stderr, "\033[?25l")
	done := make(chan struct{})
	closed := make(chan struct{})
	start := time.Now()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				bytes, entries := p.Written()
				elapsed := time.Since(start).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(bytes) / elapsed
				}
				printStatus(bytes, entries, totalBytes, totalEntries, op, speed, true)
				fmt.Fprint(os.Stderr, "\033[?25h\n")
				close(closed)
				return
			case now := <-ticker.C:
				bytes, entries := p.Written()
				elapsed := now.Sub(start).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(bytes) / elapsed
				}

				printStatus(bytes, entries, totalBytes, totalEntries, op, speed, false)
			}
		}
	}()

	return func() {
		close(done)
		<-closed
	}
}
