package main

import (
	"fmt"
	"os"

	"github.com/MuyleangIng/buildforge/commands"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "buildforge",
		Short: "My personal multi-platform image builder",
		Long: `buildforge — a personal Docker image build CLI powered by BuildKit.

Supports multi-platform builds, declarative bake configs, and flexible caching.`,
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
