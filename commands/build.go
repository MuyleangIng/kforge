package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/MuyleangIng/kforge/builder"
	kprogress "github.com/MuyleangIng/kforge/util/progress"
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

Supports multi-platform builds, registry caching, secrets, and flexible output modes.`,
		Example: `  # Build and load into local Docker
  kforge build -t myapp:latest .

  # Multi-platform push
  kforge build --platform linux/amd64,linux/arm64 --push -t myrepo/myapp:latest .

  # Registry cache
  kforge build \
    --cache-from type=registry,ref=myrepo/myapp:cache \
    --cache-to   type=registry,ref=myrepo/myapp:cache,mode=max \
    --push -t myrepo/myapp:latest .

  # Different progress styles
  kforge build --progress spinner -t myapp .
  kforge build --progress bar     -t myapp .
  kforge build --progress banner  -t myapp .
  kforge build --progress dots    -t myapp .
  kforge build --progress plain   -t myapp .

  # Via Docker CLI plugin
  docker kforge build --platform linux/amd64,linux/arm64 --push -t myrepo/myapp:latest .`,
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
	flags.StringVar(&opts.builderName, "builder", "", "Builder to use (default: active builder)")

	return cmd
}

// runBuild executes the build using the BuildKit client.
func runBuild(ctx context.Context, opts *buildOptions) error {
	// 1. Resolve builder config
	builderName := opts.builderName
	if builderName == "" {
		builderName = builder.Current()
	}

	var cfg builder.Config
	if loaded, err := builder.Load(builderName); err == nil {
		cfg = loaded
	} else {
		cfg = builder.Config{Name: "default", Driver: "docker-container"}
	}

	// 2. Connect to BuildKit
	c, err := builder.Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to BuildKit: %w\n\nTip: run `kforge builder create` first", err)
	}
	defer c.Close()

	// 3. Build session (auth + secrets)
	sess, err := buildSession(opts)
	if err != nil {
		return err
	}

	// 4. Construct SolveOpt
	so, err := buildSolveOpt(opts, sess)
	if err != nil {
		return err
	}

	// 5. Progress channel — display in one goroutine, solve in another
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return kprogress.Display(
			os.Stderr,
			kprogress.Style(opts.progress),
			ch,
			"kforge",
			"v0.1.0",
			opts.platforms,
		)
	})

	var resp *client.SolveResponse
	eg.Go(func() error {
		var solveErr error
		resp, solveErr = c.Solve(ctx, nil, *so, ch)
		return solveErr
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if resp != nil {
		for k, v := range resp.ExporterResponse {
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	return nil
}

// buildSolveOpt converts CLI options to a BuildKit SolveOpt.
func buildSolveOpt(opts *buildOptions, sess []session.Attachable) (*client.SolveOpt, error) {
	so := &client.SolveOpt{
		Frontend:      "dockerfile.v0",
		FrontendAttrs: map[string]string{},
		LocalDirs:     map[string]string{},
		Session:       sess,
	}

	contextPath := opts.contextPath
	if contextPath == "" {
		contextPath = "."
	}
	so.LocalDirs["context"] = contextPath

	if opts.dockerfile != "" {
		so.LocalDirs["dockerfile"] = opts.dockerfile
	} else {
		so.LocalDirs["dockerfile"] = contextPath
	}

	if len(opts.platforms) > 0 {
		so.FrontendAttrs["platform"] = strings.Join(opts.platforms, ",")
	}
	if opts.target != "" {
		so.FrontendAttrs["target"] = opts.target
	}
	if opts.noCache {
		so.FrontendAttrs["no-cache"] = ""
	}

	for _, arg := range opts.buildArgs {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --build-arg %q: expected KEY=VALUE", arg)
		}
		so.FrontendAttrs["build-arg:"+parts[0]] = parts[1]
	}

	for _, cf := range opts.cacheFrom {
		entry, err := parseCacheEntry(cf)
		if err != nil {
			return nil, fmt.Errorf("invalid --cache-from %q: %w", cf, err)
		}
		so.CacheImports = append(so.CacheImports, entry)
	}
	for _, ct := range opts.cacheTo {
		entry, err := parseCacheEntry(ct)
		if err != nil {
			return nil, fmt.Errorf("invalid --cache-to %q: %w", ct, err)
		}
		so.CacheExports = append(so.CacheExports, entry)
	}

	if opts.push {
		attrs := map[string]string{"push": "true"}
		if len(opts.tags) > 0 {
			attrs["name"] = strings.Join(opts.tags, ",")
		}
		so.Exports = append(so.Exports, client.ExportEntry{
			Type:  client.ExporterImage,
			Attrs: attrs,
		})
	} else if opts.load {
		attrs := map[string]string{}
		if len(opts.tags) > 0 {
			attrs["name"] = strings.Join(opts.tags, ",")
		}
		so.Exports = append(so.Exports, client.ExportEntry{
			Type:  client.ExporterDocker,
			Attrs: attrs,
		})
	} else if len(opts.tags) > 0 {
		so.Exports = append(so.Exports, client.ExportEntry{
			Type:  client.ExporterDocker,
			Attrs: map[string]string{"name": strings.Join(opts.tags, ",")},
		})
	}

	return so, nil
}

// buildSession creates BuildKit session attachables (auth + secrets).
func buildSession(opts *buildOptions) ([]session.Attachable, error) {
	var sess []session.Attachable

	dockerCfg := cliconfig.LoadDefaultConfigFile(os.Stderr)
	sess = append(sess, authprovider.NewDockerAuthProvider(dockerCfg))

	if len(opts.secrets) > 0 {
		secretSrc, err := parseSecrets(opts.secrets)
		if err != nil {
			return nil, err
		}
		store, err := secretsprovider.NewStore(secretSrc)
		if err != nil {
			return nil, err
		}
		sess = append(sess, secretsprovider.NewSecretProvider(store))
	}

	return sess, nil
}

// parseSecrets parses --secret flags into secretsprovider.Source entries.
func parseSecrets(secrets []string) ([]secretsprovider.Source, error) {
	var sources []secretsprovider.Source
	for _, s := range secrets {
		attrs := parseCSV(s)
		id, ok := attrs["id"]
		if !ok {
			return nil, fmt.Errorf("secret %q missing required field: id", s)
		}
		src := attrs["src"]
		if src == "" {
			src = attrs["source"]
		}
		sources = append(sources, secretsprovider.Source{ID: id, FilePath: src})
	}
	return sources, nil
}

// parseCacheEntry converts a comma-separated key=value string into a CacheOptionsEntry.
func parseCacheEntry(s string) (client.CacheOptionsEntry, error) {
	attrs := parseCSV(s)
	cacheType, ok := attrs["type"]
	if !ok {
		cacheType = "registry"
		attrs = map[string]string{"type": "registry", "ref": s}
	}
	delete(attrs, "type")
	return client.CacheOptionsEntry{Type: cacheType, Attrs: attrs}, nil
}

// parseCSV parses a comma-separated key=value string into a map.
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
