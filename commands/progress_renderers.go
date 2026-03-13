package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

type progressRenderer interface {
	io.Writer
	Close() error
}

var progressStepLineRE = regexp.MustCompile(`^#(\d+)\s+(.+)$`)

type stepStatus int

const (
	stepUnknown stepStatus = iota
	stepActive
	stepDone
	stepCached
	stepError
	stepOther
)

type progressLine struct {
	raw      string
	isStep   bool
	stepID   string
	content  string
	name     string
	duration string
	status   stepStatus
}

func resolveProgressStyle(style string, out io.Writer) string {
	switch style {
	case "", "auto":
		if isTerminalWriter(out) {
			return "spinner"
		}
		return "plain"
	case "spinner", "bar", "banner", "dots", "plain":
		return style
	default:
		return "plain"
	}
}

func toBuildxProgress(style string) string {
	if resolveProgressStyle(style, os.Stderr) == "plain" {
		return "plain"
	}
	return "plain"
}

func newProgressRenderer(style string, out io.Writer) progressRenderer {
	switch resolveProgressStyle(style, out) {
	case "spinner":
		return newTickerRenderer(out, []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"})
	case "dots":
		return newTickerRenderer(out, []string{"●", "○", "◉"})
	case "bar":
		return newBarRenderer(out)
	case "banner":
		return newBannerRenderer(out)
	default:
		return newPlainRenderer(out)
	}
}

type plainRenderer struct {
	out io.Writer
}

func newPlainRenderer(out io.Writer) progressRenderer {
	return &plainRenderer{out: out}
}

func (r *plainRenderer) Write(p []byte) (int, error) {
	return r.out.Write(p)
}

func (r *plainRenderer) Close() error {
	return nil
}

type lineDispatchRenderer struct {
	out        io.Writer
	partial    []byte
	handleLine func(string)
	closeFn    func()
}

func (r *lineDispatchRenderer) Write(p []byte) (int, error) {
	r.partial = append(r.partial, p...)
	for {
		idx := bytes.IndexByte(r.partial, '\n')
		if idx < 0 {
			break
		}
		line := string(r.partial[:idx])
		r.partial = r.partial[idx+1:]
		r.handleLine(strings.TrimRight(line, "\r"))
	}
	return len(p), nil
}

func (r *lineDispatchRenderer) Close() error {
	if len(r.partial) > 0 {
		r.handleLine(strings.TrimRight(string(r.partial), "\r"))
		r.partial = nil
	}
	if r.closeFn != nil {
		r.closeFn()
	}
	return nil
}

type tickerRenderer struct {
	*lineDispatchRenderer
	out     io.Writer
	frames  []string
	frame   int
	active  string
	started time.Time
	mu      sync.Mutex
	stopCh  chan struct{}
}

func newTickerRenderer(out io.Writer, frames []string) progressRenderer {
	r := &tickerRenderer{
		out:    out,
		frames: frames,
		stopCh: make(chan struct{}),
	}

	r.lineDispatchRenderer = &lineDispatchRenderer{
		out: out,
		handleLine: func(line string) {
			r.handleLine(line)
		},
		closeFn: func() {
			close(r.stopCh)
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.active != "" {
				fmt.Fprintf(r.out, "\r\033[K")
			}
		},
	}

	go r.animate()
	return r
}

func (r *tickerRenderer) animate() {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.active != "" {
				r.frame = (r.frame + 1) % len(r.frames)
				elapsed := time.Since(r.started).Round(time.Millisecond)
				fmt.Fprintf(r.out, "\r%s%s%s %s%s%s %s%s%s\033[K",
					cCyan, r.frames[r.frame], cReset,
					cBold, shortStep(r.active), cReset,
					cDim, elapsed, cReset)
			}
			r.mu.Unlock()
		}
	}
}

func (r *tickerRenderer) handleLine(line string) {
	if line == "" {
		return
	}

	ev := parseProgressLine(line)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active != "" {
		fmt.Fprintf(r.out, "\r\033[K")
	}

	switch ev.status {
	case stepActive:
		if ev.name == "" {
			ev.name = ev.content
		}
		r.active = ev.name
		if r.started.IsZero() || shortStep(r.active) != shortStep(ev.name) {
			r.started = time.Now()
		}
	case stepDone:
		fmt.Fprintf(r.out, "  %s✓%s  %s%-52s%s %s%s%s\n",
			cGreen, cReset, cBold, shortStep(ev.name), cReset, cDim, defaultDuration(ev.duration), cReset)
		if r.active == ev.name || ev.stepID != "" {
			r.active = ""
			r.started = time.Time{}
		}
	case stepCached:
		fmt.Fprintf(r.out, "  %s⚡%s %s%-52s%s %scached%s\n",
			cYellow, cReset, cDim, shortStep(ev.name), cReset, cYellow, cReset)
		r.active = ""
		r.started = time.Time{}
	case stepError:
		fmt.Fprintf(r.out, "  %s✗%s  %s%s%s\n", cRed, cReset, cRed, ev.content, cReset)
		r.active = ""
		r.started = time.Time{}
	default:
		renderOtherLine(r.out, ev.raw)
	}
}

type barRenderer struct {
	*lineDispatchRenderer
	out    io.Writer
	steps  map[string]stepStatus
	order  []string
	stepMu sync.Mutex
}

func newBarRenderer(out io.Writer) progressRenderer {
	r := &barRenderer{
		out:   out,
		steps: map[string]stepStatus{},
	}
	r.lineDispatchRenderer = &lineDispatchRenderer{
		out: out,
		handleLine: func(line string) {
			r.handleLine(line)
		},
	}
	return r
}

func (r *barRenderer) handleLine(line string) {
	if line == "" {
		return
	}

	ev := parseProgressLine(line)
	if ev.status == stepOther {
		renderOtherLine(r.out, ev.raw)
		return
	}

	r.stepMu.Lock()
	defer r.stepMu.Unlock()

	if ev.stepID != "" {
		if _, ok := r.steps[ev.stepID]; !ok {
			r.order = append(r.order, ev.stepID)
		}
		r.steps[ev.stepID] = ev.status
	}

	total := len(r.steps)
	done := 0
	for _, status := range r.steps {
		if status == stepDone || status == stepCached {
			done++
		}
	}
	if total == 0 {
		total = 1
	}

	pct := int(float64(done) / float64(total) * 100)
	barWidth := 20
	filled := pct * barWidth / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	icon := "→"
	color := cCyan
	label := ev.name
	switch ev.status {
	case stepDone:
		icon = "✓"
		color = cGreen
	case stepCached:
		icon = "⚡"
		color = cYellow
	case stepError:
		icon = "✗"
		color = cRed
		label = ev.content
	}

	fmt.Fprintf(r.out, "  %s%s%s [%s%s%s] %3d%% %s%s%s\n",
		color, icon, cReset,
		color, bar, cReset,
		pct,
		cDim, shortStep(label), cReset)
}

type bannerRenderer struct {
	*lineDispatchRenderer
}

func newBannerRenderer(out io.Writer) progressRenderer {
	r := &bannerRenderer{}
	r.lineDispatchRenderer = &lineDispatchRenderer{
		out: out,
		handleLine: func(line string) {
			renderBannerLine(out, line)
		},
	}
	return r
}

func renderBannerLine(out io.Writer, line string) {
	if line == "" {
		return
	}

	ev := parseProgressLine(line)
	switch ev.status {
	case stepCached:
		fmt.Fprintf(out, "  %s⚡%s %s%s%s\n", cYellow, cReset, cDim, shortStep(ev.name), cReset)
	case stepDone:
		fmt.Fprintf(out, "  %s✓%s  %s%s%s %s%s%s\n",
			cGreen, cReset, cBold, shortStep(ev.name), cReset, cDim, defaultDuration(ev.duration), cReset)
	case stepError:
		fmt.Fprintf(out, "  %s✗%s  %s%s%s\n", cRed, cReset, cRed, ev.content, cReset)
	case stepActive:
		fmt.Fprintf(out, "  %s▶%s  %s%s%s\n", cCyan, cReset, cGray, shortStep(ev.name), cReset)
	default:
		renderOtherLine(out, ev.raw)
	}
}

func parseProgressLine(line string) progressLine {
	ev := progressLine{
		raw:    line,
		name:   line,
		status: stepOther,
	}

	m := progressStepLineRE.FindStringSubmatch(line)
	if m == nil {
		return ev
	}

	content := strings.TrimSpace(m[2])
	ev.isStep = true
	ev.stepID = m[1]
	ev.content = content
	ev.name = content

	switch {
	case strings.Contains(content, "ERROR"):
		ev.status = stepError
	case strings.HasSuffix(content, "CACHED"):
		ev.status = stepCached
		ev.name = strings.TrimSpace(strings.TrimSuffix(content, "CACHED"))
	case strings.Contains(content, "DONE "):
		idx := strings.LastIndex(content, "DONE ")
		ev.status = stepDone
		ev.name = strings.TrimSpace(content[:idx])
		ev.duration = strings.TrimSpace(content[idx+5:])
	case strings.HasSuffix(content, " done"):
		ev.status = stepDone
		ev.name = strings.TrimSpace(strings.TrimSuffix(content, " done"))
	default:
		ev.status = stepActive
	}

	if ev.name == "" {
		ev.name = content
	}

	return ev
}

func renderOtherLine(out io.Writer, line string) {
	switch {
	case strings.HasPrefix(line, "View build details:"):
		fmt.Fprintf(out, "  %s%s%s\n", cDim, line, cReset)
	case strings.HasPrefix(line, " =>") || strings.HasPrefix(line, "=>"):
		fmt.Fprintf(out, "  %s%s%s\n", cGray, line, cReset)
	case strings.HasPrefix(strings.TrimSpace(line), "ERROR:"):
		fmt.Fprintf(out, "  %s%s%s\n", cRed, line, cReset)
	case strings.HasPrefix(line, "#") && strings.Contains(line, "[auth]"):
		fmt.Fprintf(out, "  %s🔐 %s%s\n", cDim, strings.TrimPrefix(strings.TrimSpace(strings.SplitN(line, "[auth]", 2)[1]), " "), cReset)
	default:
		fmt.Fprintf(out, "  %s%s%s\n", cGray, line, cReset)
	}
}

func defaultDuration(duration string) string {
	if duration == "" {
		return "done"
	}
	return duration
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
