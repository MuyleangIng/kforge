package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MuyleangIng/kforge/builder"
	"github.com/MuyleangIng/kforge/internal/meta"
	"github.com/MuyleangIng/kforge/internal/project"
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
	contextPath   string
	dockerfile    string
	tags          []string
	platforms     []string
	buildArgs     []string
	target        string
	cacheFrom     []string
	cacheTo       []string
	secrets       []string
	push          bool
	load          bool
	noCache       bool
	progress      string
	builderName   string
	dryRun        bool
	auto          bool
	autoFramework string
	autoNode      string
	autoPython    string
	autoJava      string
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
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Print the docker buildx command and exit")
	flags.BoolVar(&opts.auto, "auto", false, "Generate a temporary Dockerfile when none exists")
	flags.StringVar(&opts.autoFramework, "framework", "", "Force project type for --auto: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&opts.autoNode, "node", "", "Override detected Node.js major version for --auto")
	flags.StringVar(&opts.autoPython, "python", "", "Override detected Python version for FastAPI, Django, or Flask --auto builds")
	flags.StringVar(&opts.autoJava, "java", "", "Override detected Java version for Spring --auto builds")

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

	detection, cleanup, err := ensureDockerfileForBuild(opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	args := toBuildxArgs(opts, bxBuilder)
	style := resolveProgressStyle(opts.progress, os.Stderr)

	printBuildHeader(opts, bxBuilder, style)
	if detection != nil {
		fmt.Printf("  %sAuto-detected %s project%s  %s%s%s  %s%s%s\n\n",
			cCyan, detection.DisplayFramework(), cReset,
			cBold, detection.SuggestedImageName(), cReset,
			cDim, autoDetectionSuffix(*detection), cReset)
	}
	if opts.dryRun {
		fmt.Printf("  %s$ docker %s%s\n\n", cCyan, shellJoin(args), cReset)
		return nil
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin

	renderer := newProgressRenderer(style, os.Stderr)
	defer func() {
		_ = renderer.Close()
	}()
	cmd.Stderr = renderer
	cmd.Stdout = os.Stdout

	err = cmd.Run()
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

func printBuildHeader(opts *buildOptions, builderName, style string) {
	top := cCyan + "╔" + strings.Repeat("═", bannerWidth) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", bannerWidth) + "╝" + cReset

	title := cBold + cWhite + "KFORGE BUILD" + cReset +
		"  " + cDim + meta.DisplayVersion() + " · KhmerStack" + cReset

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
	fmt.Println(boxLine(cDim + "  Progress  " + cReset + strings.ToUpper(style)))
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
	host, hasHost := registryHost(img)
	switch {
	case strings.HasPrefix(img, "ghcr.io/"):
		return cBlue + "GitHub Container Registry (ghcr.io)" + cReset
	case strings.Contains(img, ".dkr.ecr.") && strings.Contains(img, ".amazonaws.com"):
		return cYellow + "AWS Elastic Container Registry (ECR)" + cReset
	case hasHost && strings.HasSuffix(host, ".azurecr.io"):
		return cBlue + "Azure Container Registry" + cReset
	case strings.HasPrefix(img, "gcr.io/") || strings.Contains(img, ".pkg.dev/"):
		return cBlue + "Google Container Registry" + cReset
	case !hasHost || host == "docker.io":
		return cBlue + "Docker Hub" + cReset
	case host == "localhost" || strings.HasPrefix(host, "localhost:"):
		return cBlue + "Local registry (" + host + ")" + cReset
	default:
		return cGray + "Custom registry (" + host + ")" + cReset
	}
}

func registryHost(image string) (string, bool) {
	first, rest, found := strings.Cut(image, "/")
	if !found {
		return "", false
	}
	if first == "localhost" || strings.Contains(first, ".") || strings.Contains(first, ":") {
		return first, true
	}
	_ = rest
	return "", false
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

func shellJoin(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			parts = append(parts, `""`)
			continue
		}
		if strings.ContainsAny(arg, " \t\n\"'") {
			parts = append(parts, strconvQuote(arg))
			continue
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}

func strconvQuote(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(s) + `"`
}

func autoDetectionSuffix(detection project.Detection) string {
	parts := []string{}
	if toolchain := detection.ToolchainDisplay(); toolchain != "" {
		parts = append(parts, toolchain)
	}
	if detection.NodeVersion != "" {
		parts = append(parts, "node "+detection.NodeVersion)
	}
	if detection.JavaVersion != "" {
		parts = append(parts, "java "+detection.JavaVersion)
	}
	if detection.PythonVersion != "" {
		parts = append(parts, "python "+detection.PythonVersion)
	}
	if detection.HealthcheckPath != "" {
		parts = append(parts, "health "+detection.HealthcheckPath)
	}
	return strings.Join(parts, " · ")
}
