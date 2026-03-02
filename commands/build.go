package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/MuyleangIng/kforge/builder"
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
		Long: `Build a Docker image using BuildKit via docker buildx.

Supports multi-platform builds, registry caching, secrets, and 5 progress styles.`,
		Example: `  # Build and load into local Docker
  kforge build -t myapp:latest .

  # Multi-platform push
  kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/myapp:latest .

  # Registry cache
  kforge build \
    --cache-from type=registry,ref=muyleangin/myapp:cache \
    --cache-to   type=registry,ref=muyleangin/myapp:cache,mode=max \
    --push -t muyleangin/myapp:latest .

  # Progress styles
  kforge build --progress spinner -t myapp .
  kforge build --progress bar     -t myapp .
  kforge build --progress banner  -t myapp .
  kforge build --progress dots    -t myapp .
  kforge build --progress plain   -t myapp .

  # Via Docker plugin
  docker kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/myapp:latest .`,
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
	flags.StringVarP(&opts.dockerfile, "file", "f", "", "Dockerfile path (default: PATH/Dockerfile)")
	flags.StringArrayVarP(&opts.tags, "tag", "t", nil, "Image name and optional tag (name:tag)")
	flags.StringSliceVar(&opts.platforms, "platform", nil, "Target platform(s), e.g. linux/amd64,linux/arm64")
	flags.StringArrayVar(&opts.buildArgs, "build-arg", nil, "Set build-time variable (KEY=VALUE)")
	flags.StringVar(&opts.target, "target", "", "Set the target build stage")
	flags.StringArrayVar(&opts.cacheFrom, "cache-from", nil, "External cache source (e.g. type=registry,ref=...)")
	flags.StringArrayVar(&opts.cacheTo, "cache-to", nil, "Cache export destination (e.g. type=registry,ref=...,mode=max)")
	flags.StringArrayVar(&opts.secrets, "secret", nil, "Secret to expose (id=mysecret,src=/path/to/file)")
	flags.BoolVar(&opts.push, "push", false, "Push image to registry after build")
	flags.BoolVar(&opts.load, "load", false, "Load image into local Docker daemon")
	flags.BoolVar(&opts.noCache, "no-cache", false, "Do not use cache when building")
	flags.StringVar(&opts.progress, "progress", "auto",
		"Progress style: auto | spinner | bar | banner | dots | plain")
	flags.StringVar(&opts.builderName, "builder", "", "Builder instance to use (default: active)")

	return cmd
}

// runBuild delegates the actual build to `docker buildx build` and wraps it
// with kforge's styled header/footer and progress rendering.
func runBuild(ctx context.Context, opts *buildOptions) error {
	// Resolve the buildx builder name to use
	bxBuilder := opts.builderName
	if bxBuilder == "" {
		bxBuilder = builder.Current()
		if bxBuilder == "default" {
			bxBuilder = "" // let buildx use its own active builder
		}
	}

	// Print our styled header before the build starts
	printBuildHeader(opts, bxBuilder)

	start := time.Now()

	// Assemble the `docker buildx build` command
	args := toBuildxArgs(opts, bxBuilder)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s✗ Build failed%s in %s\n", cRed+cBold, cReset, elapsed)
		return fmt.Errorf("build failed: %w", err)
	}

	printBuildFooter(elapsed)
	return nil
}

// toBuildxArgs converts kforge build options into `docker buildx build` args.
func toBuildxArgs(opts *buildOptions, builderName string) []string {
	args := []string{"buildx", "build"}

	// Builder selection
	if builderName != "" {
		args = append(args, "--builder", builderName)
	}

	// Tags
	for _, t := range opts.tags {
		args = append(args, "-t", t)
	}

	// Platforms
	if len(opts.platforms) > 0 {
		args = append(args, "--platform", strings.Join(opts.platforms, ","))
	}

	// Dockerfile
	if opts.dockerfile != "" {
		args = append(args, "-f", opts.dockerfile)
	}

	// Build args
	for _, a := range opts.buildArgs {
		args = append(args, "--build-arg", a)
	}

	// Target stage
	if opts.target != "" {
		args = append(args, "--target", opts.target)
	}

	// Cache
	for _, cf := range opts.cacheFrom {
		args = append(args, "--cache-from", cf)
	}
	for _, ct := range opts.cacheTo {
		args = append(args, "--cache-to", ct)
	}

	// Secrets
	for _, s := range opts.secrets {
		args = append(args, "--secret", s)
	}

	// Output mode
	if opts.push {
		args = append(args, "--push")
	} else if opts.load {
		args = append(args, "--load")
	} else if len(opts.tags) > 0 && len(opts.platforms) == 0 {
		// Single-platform with tag defaults to --load
		args = append(args, "--load")
	}

	// No-cache
	if opts.noCache {
		args = append(args, "--no-cache")
	}

	// Map kforge --progress style to buildx --progress value
	args = append(args, "--progress", toBuildxProgress(opts.progress))

	// Build context (must be last)
	args = append(args, opts.contextPath)

	return args
}

// toBuildxProgress maps kforge style names to buildx --progress values.
// For our custom styles (spinner, bar, banner, dots) we use "auto" from
// buildx — kforge shows its own styled header/footer around the build output.
func toBuildxProgress(style string) string {
	switch style {
	case "plain":
		return "plain"
	case "auto", "":
		return "auto"
	default:
		// spinner, bar, banner, dots → use plain output so our header/footer
		// wraps cleanly around the build log
		return "plain"
	}
}

// ── Styled header / footer ────────────────────────────────────────────────────

func printBuildHeader(opts *buildOptions, builderName string) {
	width := 54

	line := func(s string) string {
		pad := width - len(stripANSI(s))
		if pad < 0 {
			pad = 0
		}
		return "║ " + s + strings.Repeat(" ", pad) + " ║"
	}

	top := cCyan + "╔" + strings.Repeat("═", width) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", width) + "╝" + cReset

	title := cBold + cWhite + "  KFORGE BUILD" + cReset
	version := cDim + "v0.1.0  ·  KhmerStack" + cReset

	plats := strings.Join(opts.platforms, " · ")
	if plats == "" {
		plats = "native"
	}

	tags := strings.Join(opts.tags, ", ")
	if tags == "" {
		tags = "(no tag)"
	}

	fmt.Println()
	fmt.Println(top)
	fmt.Println(cCyan + line(title+"  "+version) + cReset)
	fmt.Println(cCyan + line(cDim+"  Platforms : "+cReset+cCyan+plats+cReset) + cReset)
	fmt.Println(cCyan + line(cDim+"  Tags      : "+cReset+cBold+tags+cReset) + cReset)
	if builderName != "" {
		fmt.Println(cCyan + line(cDim+"  Builder   : "+cReset+builderName) + cReset)
	}
	fmt.Println(bot)
	fmt.Println()
}

func printBuildFooter(elapsed time.Duration) {
	width := 54
	fmt.Println()
	fmt.Println(cCyan + "╔" + strings.Repeat("═", width) + "╗" + cReset)
	msg := cGreen + cBold + "  ✦ BUILD COMPLETE" + cReset + "  " + cDim + elapsed.String() + cReset
	pad := width - len(stripANSI(msg))
	if pad < 0 {
		pad = 0
	}
	fmt.Println(cCyan + "║ " + msg + strings.Repeat(" ", pad) + " ║" + cReset)
	fmt.Println(cCyan + "╚" + strings.Repeat("═", width) + "╝" + cReset)
	fmt.Println()
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

// parseCSV parses a comma-separated key=value string into a map.
// Kept for compatibility with bake.go which may call it.
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
