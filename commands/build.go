package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/MuyleangIng/kforge/builder"
	"github.com/spf13/cobra"
)

// ── ANSI helpers ─────────────────────────────────────────────────────────────
const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cCyan   = "\033[36m"
	cRed    = "\033[31m"
	cYellow = "\033[33m"
	cGray   = "\033[90m"
	cWhite  = "\033[97m"
	cBlue   = "\033[34m"
)

// buildOptions holds all CLI flag values for the build command.
type buildOptions struct {
	contextPath string
	dockerfile  string
	tags        []string
	platforms   []string
	buildArgs   []string
	target      string
	cacheFrom   []string
	cacheTo     []string
	secrets     []string
	push        bool
	load        bool
	noCache     bool
	progress    string
	builderName string
}

// BuildCmd returns the `kforge build` command.
func BuildCmd() *cobra.Command {
	opts := &buildOptions{}

	cmd := &cobra.Command{
		Use:   "build [OPTIONS] PATH",
		Short: "Build an image from a Dockerfile",
		Long: `Build a Docker image using BuildKit.

For a shorter command use:  kforge push IMAGE [PATH]`,
		Example: `  kforge build -t myapp:latest .
  kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/app:latest .
  kforge build --progress spinner -t myapp .
  docker kforge build --push -t muyleangin/app:latest .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.contextPath = args[0]
			} else {
				opts.contextPath = "."
			}
			return runBuild(cmd.Context(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.dockerfile, "file", "f", "", "Dockerfile path")
	flags.StringArrayVarP(&opts.tags, "tag", "t", nil, "Image name:tag")
	flags.StringSliceVar(&opts.platforms, "platform", nil, "Target platforms (linux/amd64,linux/arm64)")
	flags.StringArrayVar(&opts.buildArgs, "build-arg", nil, "Build-time variable KEY=VALUE")
	flags.StringVar(&opts.target, "target", "", "Target build stage")
	flags.StringArrayVar(&opts.cacheFrom, "cache-from", nil, "Cache source (type=registry,ref=...)")
	flags.StringArrayVar(&opts.cacheTo, "cache-to", nil, "Cache destination (type=registry,ref=...)")
	flags.StringArrayVar(&opts.secrets, "secret", nil, "Secret (id=name,src=/path)")
	flags.BoolVar(&opts.push, "push", false, "Push to registry")
	flags.BoolVar(&opts.load, "load", false, "Load into local Docker")
	flags.BoolVar(&opts.noCache, "no-cache", false, "Disable cache")
	flags.StringVar(&opts.progress, "progress", "auto",
		"Progress style: auto|spinner|bar|banner|dots|plain")
	flags.StringVar(&opts.builderName, "builder", "", "Builder to use")

	return cmd
}

// runBuild runs `docker buildx build` with kforge's styled UI.
func runBuild(ctx context.Context, opts *buildOptions) error {
	bxBuilder := opts.builderName
	if bxBuilder == "" {
		bxBuilder = builder.Current()
		if bxBuilder == "default" {
			bxBuilder = ""
		}
	}

	printBuildHeader(opts, bxBuilder)
	start := time.Now()

	args := toBuildxArgs(opts, bxBuilder)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin

	// Use our colorized writer for stderr (buildx sends progress to stderr)
	lw := &buildLineWriter{out: os.Stderr}
	cmd.Stderr = lw
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s%s✗ Build failed%s  %s\n\n",
			cRed, cBold, cReset, cDim+elapsed.String()+cReset)
		return fmt.Errorf("build failed: %w", err)
	}

	printBuildFooter(elapsed, opts.tags)
	return nil
}

// toBuildxArgs converts kforge buildOptions into `docker buildx build` args.
func toBuildxArgs(opts *buildOptions, builderName string) []string {
	args := []string{"buildx", "build"}

	if builderName != "" {
		args = append(args, "--builder", builderName)
	}
	for _, t := range opts.tags {
		args = append(args, "-t", t)
	}
	if len(opts.platforms) > 0 {
		args = append(args, "--platform", strings.Join(opts.platforms, ","))
	}
	if opts.dockerfile != "" {
		args = append(args, "-f", opts.dockerfile)
	}
	for _, a := range opts.buildArgs {
		args = append(args, "--build-arg", a)
	}
	if opts.target != "" {
		args = append(args, "--target", opts.target)
	}
	for _, cf := range opts.cacheFrom {
		args = append(args, "--cache-from", cf)
	}
	for _, ct := range opts.cacheTo {
		args = append(args, "--cache-to", ct)
	}
	for _, s := range opts.secrets {
		args = append(args, "--secret", s)
	}
	if opts.push {
		args = append(args, "--push")
	} else if opts.load {
		args = append(args, "--load")
	} else if len(opts.tags) > 0 && len(opts.platforms) == 0 {
		args = append(args, "--load")
	}
	if opts.noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "--progress", toBuildxProgress(opts.progress))
	args = append(args, opts.contextPath)
	return args
}

func toBuildxProgress(style string) string {
	switch style {
	case "plain":
		return "plain"
	case "auto", "":
		return "plain" // we colorize plain output ourselves
	default:
		return "plain"
	}
}

// ── Live colorized output ─────────────────────────────────────────────────────

// reStepLine matches buildx plain output lines like "#3 [2/5] COPY . ."
var reStepLine = regexp.MustCompile(`^#(\d+)\s+(.+)$`)

// buildLineWriter intercepts buildx plain output and renders it with colors.
type buildLineWriter struct {
	out     io.Writer
	partial []byte
}

func (w *buildLineWriter) Write(p []byte) (n int, err error) {
	w.partial = append(w.partial, p...)
	for {
		idx := bytes.IndexByte(w.partial, '\n')
		if idx < 0 {
			break
		}
		line := string(w.partial[:idx])
		w.partial = w.partial[idx+1:]
		w.renderLine(strings.TrimRight(line, "\r"))
	}
	return len(p), nil
}

func (w *buildLineWriter) renderLine(line string) {
	if line == "" {
		return
	}

	m := reStepLine.FindStringSubmatch(line)
	if m == nil {
		// Non-step lines: auth tokens, general output
		if strings.HasPrefix(line, "View build details:") {
			fmt.Fprintf(w.out, "  %s%s%s\n", cDim, line, cReset)
		} else if strings.HasPrefix(line, " =>") || strings.HasPrefix(line, "=>") {
			fmt.Fprintf(w.out, "  %s%s%s\n", cGray, line, cReset)
		} else {
			fmt.Fprintf(w.out, "  %s%s%s\n", cGray, line, cReset)
		}
		return
	}

	content := strings.TrimSpace(m[2])

	switch {
	case strings.HasSuffix(content, "CACHED"):
		// ⚡ cached step — yellow
		name := strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(content, "CACHED")), " ")
		fmt.Fprintf(w.out, "  %s⚡%s %s%-52s%s %scached%s\n",
			cYellow, cReset, cDim, shortStep(name), cReset, cYellow, cReset)

	case strings.Contains(content, "DONE ") || strings.HasSuffix(content, "done"):
		// ✓ completed step — green
		parts := strings.LastIndex(content, "DONE ")
		if parts > 0 {
			name := strings.TrimSpace(content[:parts])
			dur := strings.TrimSpace(content[parts+5:])
			fmt.Fprintf(w.out, "  %s✓%s  %s%-52s%s %s%s%s\n",
				cGreen, cReset, cBold, shortStep(name), cReset, cDim, dur, cReset)
		} else {
			fmt.Fprintf(w.out, "  %s✓%s  %s%s%s\n", cGreen, cReset, cDim, shortStep(content), cReset)
		}

	case strings.Contains(content, "ERROR"):
		// ✗ error — red
		fmt.Fprintf(w.out, "  %s✗%s  %s%s%s\n", cRed, cReset, cRed, content, cReset)

	case strings.HasPrefix(content, "[auth]"):
		// auth token exchange — subtle
		fmt.Fprintf(w.out, "  %s🔐 %s%s\n", cDim, content[6:], cReset)

	default:
		// In-progress or info step — cyan arrow
		fmt.Fprintf(w.out, "  %s→%s  %s%s%s\n", cCyan, cReset, cGray, shortStep(content), cReset)
	}
}

// shortStep trims internal prefixes and long strings for display.
func shortStep(s string) string {
	s = strings.TrimPrefix(s, "[internal] ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// ── Styled header / footer ────────────────────────────────────────────────────

const bannerWidth = 54

func boxLine(content string) string {
	// Use visible (non-ANSI) length for correct padding
	visible := len(stripANSI(content))
	pad := bannerWidth - visible
	if pad < 0 {
		pad = 0
	}
	return cCyan + "║ " + cReset + content + strings.Repeat(" ", pad) + cCyan + " ║" + cReset
}

func printBuildHeader(opts *buildOptions, builderName string) {
	top := cCyan + "╔" + strings.Repeat("═", bannerWidth) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", bannerWidth) + "╝" + cReset

	title := cBold + cWhite + "KFORGE BUILD" + cReset +
		"  " + cDim + "v0.1.0 · KhmerStack" + cReset

	plats := strings.Join(opts.platforms, " · ")
	if plats == "" {
		plats = "native"
	}
	tags := strings.Join(opts.tags, "  ")
	if tags == "" {
		tags = cDim + "(no tag)" + cReset
	}

	reg := ""
	if len(opts.tags) > 0 {
		reg = detectRegistry(opts.tags[0])
	}

	fmt.Println()
	fmt.Println(top)
	fmt.Println(boxLine(cCyan + "  " + cReset + title))
	fmt.Println(cCyan + "║" + strings.Repeat("─", bannerWidth) + "║" + cReset)
	fmt.Println(boxLine(cDim + "  Platform  " + cReset + cCyan + plats + cReset))
	fmt.Println(boxLine(cDim + "  Tag       " + cReset + cBold + tags + cReset))
	if reg != "" {
		fmt.Println(boxLine(cDim + "  Registry  " + cReset + reg))
	}
	if builderName != "" {
		fmt.Println(boxLine(cDim + "  Builder   " + cReset + builderName))
	}
	fmt.Println(bot)
	fmt.Println()
}

func printBuildFooter(elapsed time.Duration, tags []string) {
	top := cCyan + "╔" + strings.Repeat("═", bannerWidth) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", bannerWidth) + "╝" + cReset

	msg := cGreen + cBold + "✦  BUILD COMPLETE" + cReset +
		"  " + cDim + elapsed.String() + cReset

	fmt.Println()
	fmt.Println(top)
	fmt.Println(boxLine(msg))
	if len(tags) > 0 {
		fmt.Println(boxLine(cDim + "   " + strings.Join(tags, "  ") + cReset))
	}
	fmt.Println(bot)
	fmt.Println()
}

// ── Registry detection ────────────────────────────────────────────────────────

// detectRegistry returns a human-readable registry label from an image name.
func detectRegistry(image string) string {
	img := strings.ToLower(image)
	switch {
	case strings.HasPrefix(img, "ghcr.io/"):
		return cBlue + "GitHub Container Registry (ghcr.io)" + cReset
	case strings.Contains(img, ".dkr.ecr.") && strings.Contains(img, ".amazonaws.com"):
		return cYellow + "AWS Elastic Container Registry (ECR)" + cReset
	case strings.HasSuffix(strings.Split(img, "/")[0], ".azurecr.io"):
		return cBlue + "Azure Container Registry" + cReset
	case strings.HasPrefix(img, "gcr.io/") || strings.Contains(img, ".pkg.dev/"):
		return cBlue + "Google Container Registry" + cReset
	case strings.HasPrefix(img, "docker.io/") || !strings.Contains(strings.Split(img, "/")[0], "."):
		return cBlue + "Docker Hub" + cReset
	default:
		host := strings.Split(img, "/")[0]
		return cGray + "Custom registry (" + host + ")" + cReset
	}
}

// stripANSI removes ANSI escape codes for length calculation.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// parseCSV parses comma-separated key=value into a map.
func parseCSV(s string) map[string]string {
	result := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		} else if len(kv) == 1 && kv[0] != "" {
			result[kv[0]] = ""
		}
	}
	return result
}
