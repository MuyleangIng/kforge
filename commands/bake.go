package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/MuyleangIng/buildforge/bake"
	"golang.org/x/sync/errgroup"
)

// BakeCmd returns the `buildforge bake` command.
func BakeCmd() *cobra.Command {
	var (
		file        string
		set         []string
		targets     []string
		builderName string
		push        bool
		load        bool
		noCache     bool
		progress    string
	)

	cmd := &cobra.Command{
		Use:   "bake [OPTIONS] [TARGET...]",
		Short: "Build from a bake config file",
		Long: `Build one or more targets defined in a buildforge.hcl or buildforge.json config file.

If no targets are specified, the "default" group is built.
If no "default" group exists, all targets are built.`,
		Example: `  # Build default group
  buildforge bake

  # Build specific targets
  buildforge bake app frontend

  # Override a field
  buildforge bake --set app.platforms=linux/arm64

  # Use a custom file
  buildforge bake -f ci/buildforge.hcl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets = args
			return runBake(cmd.Context(), file, set, targets, builderName, push, load, noCache, progress)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&file, "file", "f", "", "Bake config file (default: buildforge.hcl or buildforge.json)")
	flags.StringArrayVar(&set, "set", nil, "Override target field (target.field=value)")
	flags.StringVar(&builderName, "builder", "", "Builder to use (default: active builder)")
	flags.BoolVar(&push, "push", false, "Push all images to registry (overrides per-target setting)")
	flags.BoolVar(&load, "load", false, "Load all images into local Docker")
	flags.BoolVar(&noCache, "no-cache", false, "Do not use cache")
	flags.StringVar(&progress, "progress", "auto", "Progress output: auto, plain, tty")

	return cmd
}

func runBake(ctx context.Context, file string, set, targetNames []string,
	builderName string, push, load, noCache bool, progress string) error {

	// 1. Load config file
	f, err := bake.Load(file)
	if err != nil {
		return err
	}

	// 2. Apply --set overrides
	if len(set) > 0 {
		if err := f.ApplySet(set); err != nil {
			return err
		}
	}

	// 3. Resolve targets
	targets, err := f.ResolveTargets(targetNames)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no targets to build")
	}

	fmt.Printf("Building %d target(s)...\n", len(targets))

	// 4. Build each target (in parallel)
	eg, ctx := errgroup.WithContext(ctx)
	for _, t := range targets {
		t := t // capture loop variable
		eg.Go(func() error {
			return buildTarget(ctx, t, builderName, push, load, noCache, progress)
		})
	}
	return eg.Wait()
}

// buildTarget converts a bake target into build options and runs the build.
func buildTarget(ctx context.Context, t bake.Target, builderName string,
	globalPush, globalLoad, globalNoCache bool, progress string) error {

	fmt.Printf("[%s] starting build\n", t.Name)

	opts := &buildOptions{
		contextPath: t.Context,
		dockerfile:  t.Dockerfile,
		tags:        t.Tags,
		platforms:   t.Platforms,
		buildArgs:   t.BuildArgs,
		target:      t.Target,
		cacheFrom:   t.CacheFrom,
		cacheTo:     t.CacheTo,
		secrets:     t.Secrets,
		push:        t.Push || globalPush,
		load:        t.Load || globalLoad,
		noCache:     t.NoCache || globalNoCache,
		progress:    progress,
		builderName: builderName,
	}

	if opts.contextPath == "" {
		opts.contextPath = "."
	}

	if err := runBuild(ctx, opts); err != nil {
		return fmt.Errorf("[%s] %w", t.Name, err)
	}

	fmt.Printf("[%s] done\n", t.Name)
	return nil
}
