package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MuyleangIng/kforge/commands"
	"github.com/spf13/cobra"
)

const (
	toolName = "kforge"
	version  = "v0.1.0"
	vendor   = "KhmerStack / Ing Muyleang"
)

// pluginMetadata is the response Docker expects when discovering CLI plugins.
type pluginMetadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Version          string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
	URL              string `json:"URL"`
}

func main() {
	// Docker plugin protocol: `docker-kforge docker-cli-plugin-metadata`
	// Docker calls this to discover the plugin name, version, and vendor.
	if len(os.Args) > 1 && os.Args[1] == "docker-cli-plugin-metadata" {
		meta := pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           vendor,
			Version:          version,
			ShortDescription: "Personal multi-platform image builder powered by BuildKit",
			URL:              "https://github.com/MuyleangIng/kforge",
		}
		if err := json.NewEncoder(os.Stdout).Encode(meta); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Docker plugin protocol: when invoked as `docker kforge build ...`, Docker runs:
	//   docker-kforge kforge build ...
	// Strip the leading "kforge" argument so cobra receives `build ...`
	if len(os.Args) > 1 && os.Args[1] == toolName {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	root := &cobra.Command{
		Use:   toolName,
		Short: "kforge — personal multi-platform image builder",
		Long: `kforge — a personal Docker image build CLI powered by BuildKit.
Founded by KhmerStack · Built by Ing Muyleang

Works as a standalone binary AND as a Docker CLI plugin:

  Standalone:    kforge build --platform linux/amd64,linux/arm64 --push -t myrepo/app .
  Docker plugin: docker kforge build --platform linux/amd64,linux/arm64 --push -t myrepo/app .

Progress styles (--progress flag):
  auto     auto-detect: spinner if TTY, plain otherwise  (default)
  spinner  animated spinner with colored step names
  bar      ASCII progress bars per Dockerfile stage
  banner   big ASCII banner header + streaming logs
  dots     minimal pulsing dot indicator
  plain    raw log output, no colors`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		commands.BuildCmd(),
		commands.BakeCmd(),
		commands.BuilderCmd(),
		commands.VersionCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
