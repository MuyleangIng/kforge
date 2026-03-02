// Package progress provides multiple styled progress displays for kforge builds.
//
// Available styles (set via --progress flag):
//
//	spinner  — animated spinner + colored step names (default when TTY)
//	bar      — per-step ASCII progress bars
//	banner   — big ASCII banner header + streaming plain logs
//	dots     — minimal pulsing dots
//	plain    — raw BuildKit log output (no colors)
//	auto     — spinner if TTY, plain otherwise
package progress

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moby/buildkit/client"
	"golang.org/x/term"
)

// ANSI color codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	white  = "\033[97m"
	gray   = "\033[90m"
)

// spinnerFrames are the animation frames for the spinner style.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Style represents a progress display mode.
type Style string

const (
	StyleAuto    Style = "auto"
	StyleSpinner Style = "spinner"
	StyleBar     Style = "bar"
	StyleBanner  Style = "banner"
	StyleDots    Style = "dots"
	StylePlain   Style = "plain"
)

// Display reads from ch and writes formatted progress to w.
// It returns the list of build warnings when the channel closes.
func Display(w io.Writer, style Style, ch <-chan *client.SolveStatus, toolName, version string, platforms []string) error {
	// Resolve "auto"
	if style == StyleAuto {
		if isTerminal(w) {
			style = StyleSpinner
		} else {
			style = StylePlain
		}
	}

	switch style {
	case StyleSpinner:
		return displaySpinner(w, ch)
	case StyleBar:
		return displayBar(w, ch)
	case StyleBanner:
		return displayBanner(w, ch, toolName, version, platforms)
	case StyleDots:
		return displayDots(w, ch)
	default: // plain
		return displayPlain(w, ch)
	}
}

// ─── SPINNER ─────────────────────────────────────────────────────────────────

type stepState struct {
	name      string
	started   *time.Time
	completed *time.Time
	cached    bool
	errored   bool
}

func displaySpinner(w io.Writer, ch <-chan *client.SolveStatus) error {
	steps := map[string]*stepState{}
	var mu sync.Mutex
	done := make(chan struct{})
	frame := 0

	// Spinner goroutine
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				frame = (frame + 1) % len(spinnerFrames)
				// Reprint active steps
				active := 0
				for _, s := range steps {
					if s.started != nil && s.completed == nil {
						active++
					}
				}
				if active > 0 {
					// Move cursor up active lines and redraw
					for _, s := range steps {
						if s.started != nil && s.completed == nil {
							dur := time.Since(*s.started).Round(time.Millisecond)
							fmt.Fprintf(w, "\r%s%s%s %s%s%s %s%s\033[K\n",
								cyan, spinnerFrames[frame], reset,
								bold, shortName(s.name), reset,
								dim, dur)
						}
					}
					// Move cursor back up
					fmt.Fprintf(w, "\033[%dA", active)
				}
				mu.Unlock()
			}
		}
	}()

	start := time.Now()

	for status := range ch {
		mu.Lock()

		for _, v := range status.Vertexes {
			id := v.Digest.String()
			if _, ok := steps[id]; !ok {
				steps[id] = &stepState{name: v.Name}
			}
			s := steps[id]
			s.name = v.Name
			s.cached = v.Cached

			if v.Started != nil && s.started == nil {
				t := *v.Started
				s.started = &t
				// Print "starting" line
				prefix := cyan + spinnerFrames[frame] + reset
				fmt.Fprintf(w, "%s %s%s%s\033[K\n", prefix, bold, shortName(v.Name), reset)
			}

			if v.Completed != nil && s.completed == nil {
				t := *v.Completed
				s.completed = &t

				var dur time.Duration
				if s.started != nil {
					dur = t.Sub(*s.started).Round(time.Millisecond)
				}

				// Overwrite the spinner line with completion
				var icon, col string
				if v.Error != "" {
					icon, col = "✗", red
					s.errored = true
				} else if v.Cached {
					icon, col = "⚡", yellow
				} else {
					icon, col = "✓", green
				}
				fmt.Fprintf(w, "\033[1A\r%s%s%s %s%s%s %s%s%s\033[K\n",
					col, icon, reset,
					bold, shortName(v.Name), reset,
					dim, dur, reset)
			}
		}

		// Print log lines
		for _, l := range status.Logs {
			id := l.Vertex.String()
			name := ""
			if s, ok := steps[id]; ok {
				name = shortName(s.name)
			}
			lines := strings.Split(strings.TrimRight(string(l.Data), "\n"), "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(w, "  %s│%s %s\n", gray, reset, line)
					_ = name
				}
			}
		}

		mu.Unlock()
	}

	close(done)
	total := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(w, "\n%s✦ Build complete%s in %s%s%s\n", green+bold, reset, bold, total, reset)
	return nil
}

// ─── BAR ─────────────────────────────────────────────────────────────────────

func displayBar(w io.Writer, ch <-chan *client.SolveStatus) error {
	type barStep struct {
		name      string
		done      bool
		cached    bool
		errored   bool
		started   *time.Time
		completed *time.Time
	}

	steps := map[string]*barStep{}
	order := []string{}
	printed := map[string]bool{}
	lineCount := 0

	printBar := func() {
		if lineCount > 0 {
			fmt.Fprintf(w, "\033[%dA", lineCount)
		}
		lineCount = 0
		for _, id := range order {
			s := steps[id]
			var elapsed time.Duration
			var progress float64

			if s.started != nil {
				if s.done {
					elapsed = s.completed.Sub(*s.started).Round(time.Millisecond)
					progress = 1.0
				} else {
					elapsed = time.Since(*s.started).Round(time.Millisecond)
					// Fake progress: asymptotically approaches 95%
					secs := elapsed.Seconds()
					progress = 1 - math.Exp(-secs/8)
					if progress > 0.95 {
						progress = 0.95
					}
				}
			}

			barWidth := 20
			filled := int(float64(barWidth) * progress)
			if filled > barWidth {
				filled = barWidth
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

			var col, status string
			switch {
			case s.errored:
				col, status = red, "FAIL"
			case s.cached:
				col, status = yellow, "CACHED"
			case s.done:
				col, status = green, "DONE"
			default:
				col, status = cyan, "..."
			}

			pct := int(progress * 100)
			label := pad(shortName(s.name), 30)
			fmt.Fprintf(w, "%s%-6s%s %s%s%s %3d%% %s%s%s\033[K\n",
				col, status, reset,
				dim, label, reset,
				pct, col, bar, reset)
			fmt.Fprintf(w, "       %s%s%s\033[K\n", gray, elapsed, reset)
			lineCount += 2
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			printBar()
		}
	}()

	start := time.Now()

	for status := range ch {
		for _, v := range status.Vertexes {
			id := v.Digest.String()
			if _, ok := steps[id]; !ok {
				steps[id] = &barStep{name: v.Name}
				if !printed[id] {
					order = append(order, id)
					printed[id] = true
				}
			}
			s := steps[id]
			s.name = v.Name
			s.cached = v.Cached
			if v.Started != nil && s.started == nil {
				t := *v.Started
				s.started = &t
			}
			if v.Completed != nil && !s.done {
				t := *v.Completed
				s.completed = &t
				s.done = true
				if v.Error != "" {
					s.errored = true
				}
			}
		}
	}

	// Final print (all done)
	ticker.Stop()
	printBar()

	total := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(w, "\n%s✦ Build complete%s in %s%s%s\n", green+bold, reset, bold, total, reset)
	return nil
}

// ─── BANNER ──────────────────────────────────────────────────────────────────

func displayBanner(w io.Writer, ch <-chan *client.SolveStatus, toolName, version string, platforms []string) error {
	width := 46
	plats := strings.Join(platforms, " · ")
	if plats == "" {
		plats = "native"
	}

	line := func(s string) string {
		pad := width - 2 - len(s)
		if pad < 0 {
			pad = 0
		}
		return "║ " + s + strings.Repeat(" ", pad) + " ║"
	}

	top := "╔" + strings.Repeat("═", width) + "╗"
	bot := "╚" + strings.Repeat("═", width) + "╝"

	title := bold + white + strings.ToUpper(toolName) + " BUILD  " + version + reset
	sub := dim + plats + reset

	fmt.Fprintln(w, cyan+top+reset)
	fmt.Fprintln(w, cyan+line(title)+reset)
	fmt.Fprintln(w, cyan+line(sub)+reset)
	fmt.Fprintln(w, cyan+bot+reset)
	fmt.Fprintln(w)

	start := time.Now()

	for status := range ch {
		for _, v := range status.Vertexes {
			if v.Started != nil && v.Completed == nil {
				fmt.Fprintf(w, "%s▶%s  %s\n", cyan, reset, shortName(v.Name))
			}
			if v.Completed != nil {
				icon := green + "✓" + reset
				if v.Error != "" {
					icon = red + "✗" + reset
				} else if v.Cached {
					icon = yellow + "⚡" + reset
				}
				var dur time.Duration
				if v.Started != nil {
					dur = v.Completed.Sub(*v.Started).Round(time.Millisecond)
				}
				fmt.Fprintf(w, "%s  %s%s %s%s\n", icon, dim, shortName(v.Name), dur, reset)
			}
		}
		for _, l := range status.Logs {
			lines := strings.Split(strings.TrimRight(string(l.Data), "\n"), "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(w, "  %s%s%s\n", gray, line, reset)
				}
			}
		}
	}

	total := time.Since(start).Round(time.Millisecond)
	fmt.Fprintln(w)
	fmt.Fprintln(w, cyan+"╔"+strings.Repeat("═", width)+"╗"+reset)
	msg := bold + green + "BUILD COMPLETE" + reset + "  " + dim + total.String() + reset
	fmt.Fprintln(w, cyan+line(msg)+reset)
	fmt.Fprintln(w, cyan+"╚"+strings.Repeat("═", width)+"╝"+reset)
	return nil
}

// ─── DOTS ────────────────────────────────────────────────────────────────────

var dotFrames = []string{"●", "○", "◉"}

func displayDots(w io.Writer, ch <-chan *client.SolveStatus) error {
	active := ""
	frame := 0
	done := make(chan struct{})
	var mu sync.Mutex

	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				frame = (frame + 1) % len(dotFrames)
				if active != "" {
					fmt.Fprintf(w, "\r%s%s%s %s\033[K",
						cyan, dotFrames[frame], reset, active)
				}
				mu.Unlock()
			}
		}
	}()

	start := time.Now()

	for status := range ch {
		mu.Lock()
		for _, v := range status.Vertexes {
			if v.Started != nil && v.Completed == nil {
				active = dim + shortName(v.Name) + reset
				fmt.Fprintf(w, "\r%s%s%s %s\033[K",
					cyan, dotFrames[frame], reset, active)
			}
			if v.Completed != nil {
				var dur time.Duration
				if v.Started != nil {
					dur = v.Completed.Sub(*v.Started).Round(time.Millisecond)
				}
				icon, col := "·", green
				if v.Error != "" {
					icon, col = "✗", red
				} else if v.Cached {
					icon, col = "·", yellow
				}
				fmt.Fprintf(w, "\r  %s%s%s %s%s%s\033[K\n",
					col, icon, reset,
					dim, shortName(v.Name)+"  "+dur.String(), reset)
				active = ""
			}
		}
		mu.Unlock()
	}

	close(done)
	total := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(w, "\r%s● Done%s in %s%s%s\033[K\n", green+bold, reset, bold, total, reset)
	return nil
}

// ─── PLAIN ───────────────────────────────────────────────────────────────────

func displayPlain(w io.Writer, ch <-chan *client.SolveStatus) error {
	start := time.Now()
	for status := range ch {
		for _, v := range status.Vertexes {
			if v.Started != nil && v.Completed == nil {
				fmt.Fprintf(w, "#   %s\n", v.Name)
			}
			if v.Completed != nil {
				cached := ""
				if v.Cached {
					cached = " (cached)"
				}
				if v.Error != "" {
					fmt.Fprintf(w, "FAIL %s: %s\n", v.Name, v.Error)
				} else {
					var dur time.Duration
					if v.Started != nil {
						dur = v.Completed.Sub(*v.Started).Round(time.Millisecond)
					}
					fmt.Fprintf(w, "DONE %s%s %s\n", v.Name, cached, dur)
				}
			}
		}
		for _, l := range status.Logs {
			lines := strings.Split(strings.TrimRight(string(l.Data), "\n"), "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintln(w, line)
				}
			}
		}
	}
	total := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(w, "\nBuild complete in %s\n", total)
	return nil
}

// ─── HELPERS ─────────────────────────────────────────────────────────────────

// shortName trims BuildKit internal prefixes from step names.
func shortName(s string) string {
	s = strings.TrimPrefix(s, "[internal] ")
	s = strings.TrimPrefix(s, "[context ] ")
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// pad pads or truncates s to exactly n characters.
func pad(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

// isTerminal reports whether w is a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
