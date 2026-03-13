package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// PushCmd returns the `kforge push` shortcut command.
// It replaces the long:
//
//	kforge build --platform linux/amd64,linux/arm64 --push --cache-from ... -t IMAGE PATH
//
// with:
//
//	kforge push IMAGE [PATH]
func PushCmd() *cobra.Command {
	var (
		platforms     []string
		extraTags     []string
		buildArgs     []string
		dockerfile    string
		target        string
		noCache       bool
		cacheRef      string
		progress      string
		builderName   string
		dryRun        bool
		auto          bool
		autoFramework string
		autoNode      string
		autoPython    string
		autoJava      string
	)

	cmd := &cobra.Command{
		Use:   "push IMAGE [PATH]",
		Short: "Build + push a multi-platform image in one command",
		Long: `Build and push a multi-platform Docker image in a single command.

Defaults:
  · Platforms: linux/amd64, linux/arm64
  · Cache:     automatic registry cache (IMAGE:buildcache)
  · Output:    always pushed to registry

Registry support:
  Docker Hub    muyleangin/app:latest
  GHCR          ghcr.io/muyleangin/app:latest
  ECR           123456.dkr.ecr.us-east-1.amazonaws.com/app:latest
  ACR           myacr.azurecr.io/app:latest
  Custom        registry.myco.com/app:latest`,
		Example: `  kforge push muyleangin/myapp .
  kforge push muyleangin/myapp:v1.2 .
  kforge push ghcr.io/muyleangin/myapp .
  kforge push muyleangin/myapp . --platform linux/amd64
  kforge push muyleangin/myapp . --no-cache
  docker kforge push muyleangin/myapp .`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			image := autoTag(args[0])
			path := "."
			if len(args) == 2 {
				path = args[1]
			}
			return runPush(cmd.Context(), image, path, platforms, extraTags,
				buildArgs, dockerfile, target, cacheRef, noCache, progress, builderName, dryRun, auto, autoFramework, autoNode, autoPython, autoJava)
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVar(&platforms, "platform", []string{"linux/amd64", "linux/arm64"},
		"Target platforms")
	flags.StringArrayVarP(&extraTags, "tag", "t", nil, "Additional image tags")
	flags.StringArrayVar(&buildArgs, "build-arg", nil, "Build-time variable KEY=VALUE")
	flags.StringVarP(&dockerfile, "file", "f", "", "Dockerfile path")
	flags.StringVar(&target, "target", "", "Target build stage")
	flags.BoolVar(&noCache, "no-cache", false, "Disable cache")
	flags.StringVar(&cacheRef, "cache-ref", "",
		"Custom cache image ref (default: IMAGE:buildcache)")
	flags.StringVar(&progress, "progress", "auto",
		"Progress style: auto|spinner|bar|banner|dots|plain")
	flags.StringVar(&builderName, "builder", "", "Builder instance to use")
	flags.BoolVar(&dryRun, "dry-run", false, "Print the docker buildx command and exit")
	flags.BoolVar(&auto, "auto", false, "Generate a temporary Dockerfile when none exists")
	flags.StringVar(&autoFramework, "framework", "", "Force project type for --auto: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&autoNode, "node", "", "Override detected Node.js major version for --auto")
	flags.StringVar(&autoPython, "python", "", "Override detected Python version for FastAPI, Django, or Flask --auto pushes")
	flags.StringVar(&autoJava, "java", "", "Override detected Java version for Spring --auto pushes")

	return cmd
}

func runPush(ctx context.Context,
	image, path string,
	platforms, extraTags, buildArgs []string,
	dockerfile, target, cacheRef string,
	noCache bool, progress, builderName string, dryRun, auto bool, autoFramework, autoNode, autoPython, autoJava string) error {

	// Build the full tag list
	tags := []string{image}
	tags = append(tags, extraTags...)

	// Auto-cache: derive cache ref from image name
	var cacheFrom, cacheTo []string
	if !noCache {
		ref := cacheRef
		if ref == "" {
			ref = cacheImageRef(image)
		}
		cacheFrom = []string{"type=registry,ref=" + ref}
		cacheTo = []string{"type=registry,ref=" + ref + ",mode=max"}
		fmt.Printf("  %s⚡ Cache ref: %s%s\n", cDim, ref, cReset)
	}

	opts := &buildOptions{
		contextPath:   path,
		dockerfile:    dockerfile,
		tags:          tags,
		platforms:     platforms,
		buildArgs:     buildArgs,
		target:        target,
		cacheFrom:     cacheFrom,
		cacheTo:       cacheTo,
		push:          true,
		noCache:       noCache,
		progress:      progress,
		builderName:   builderName,
		dryRun:        dryRun,
		auto:          auto,
		autoFramework: autoFramework,
		autoNode:      autoNode,
		autoPython:    autoPython,
		autoJava:      autoJava,
	}

	return runBuild(ctx, opts)
}

// autoTag appends ":latest" if the image has no tag.
func autoTag(image string) string {
	// Don't add tag if digest (@sha256:...) is present
	if strings.Contains(image, "@") {
		return image
	}
	// Find the last segment after the last "/"
	last := image
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		last = image[idx+1:]
	}
	// If the last segment has no ":", add ":latest"
	if !strings.Contains(last, ":") {
		return image + ":latest"
	}
	return image
}

// cacheImageRef derives the cache image ref from the main image.
// e.g. "muyleangin/app:v1.2" → "muyleangin/app:buildcache"
//
//	"ghcr.io/user/app:latest" → "ghcr.io/user/app:buildcache"
func cacheImageRef(image string) string {
	// Strip tag/digest
	base := image
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		host := image[:idx]
		last := image[idx+1:]
		if i := strings.Index(last, ":"); i >= 0 {
			last = last[:i]
		}
		if i := strings.Index(last, "@"); i >= 0 {
			last = last[:i]
		}
		base = host + "/" + last
	} else {
		// No slash: plain "myapp" or "myapp:tag"
		if i := strings.Index(base, ":"); i >= 0 {
			base = base[:i]
		}
	}
	return base + ":buildcache"
}
