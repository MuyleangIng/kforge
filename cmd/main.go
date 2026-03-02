package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MuyleangIng/kforge/commands"
	"github.com/spf13/cobra"
)

const (
	toolName = "kforge"
	version  = "v0.1.0"
	vendor   = "KhmerStack / Ing Muyleang"
)

// pluginMetadata is returned when Docker discovers this CLI plugin.
type pluginMetadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Version          string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
	URL              string `json:"URL"`
}

func main() {
	// ── Docker plugin protocol ────────────────────────────────────────────────
	//
	// When Docker discovers plugins it calls:
	//   docker-kforge docker-cli-plugin-metadata
	// We respond with JSON metadata.
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

	// When invoked as a Docker plugin, Docker calls:
	//   docker-kforge [--docker-global-flags...] kforge <subcommand> [args...]
	//
	// We strip any leading Docker global flags (--context, --host, --tls*, etc.)
	// and the plugin-name arg ("kforge") so cobra only sees <subcommand> [args...].
	os.Args = stripDockerPluginArgs(os.Args)

	root := &cobra.Command{
		Use:   toolName,
		Short: "kforge — personal multi-platform image builder",
		Long: `kforge — a personal Docker image build CLI powered by BuildKit.
Founded by KhmerStack · Built by Ing Muyleang

Works as a standalone binary AND as a Docker CLI plugin:

  Standalone:    kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/app .
  Docker plugin: docker kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/app .

Progress styles (--progress flag):
  auto     auto-detect: spinner if TTY, plain otherwise  (default)
  spinner  animated spinner + colored step names
  bar      ASCII progress bars per Dockerfile stage
  banner   big ASCII banner header + streaming logs
  dots     minimal pulsing dot indicator
  plain    raw log output, no colors

Setup:
  kforge setup   interactive wizard: QEMU emulation or multi-node builders`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		commands.BuildCmd(),
		commands.BakeCmd(),
		commands.BuilderCmd(),
		commands.SetupCmd(),
		commands.VersionCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// stripDockerPluginArgs removes Docker global flags and the plugin-name arg
// that Docker prepends when calling a plugin binary.
//
// Docker calls: docker-kforge [--context X] [--host X] [...] kforge <args>
// After strip:  docker-kforge <args>
func stripDockerPluginArgs(args []string) []string {
	// Docker global flags that take a value argument
	dockerValueFlags := map[string]bool{
		"--context": true, "-c": true,
		"--host": true, "-H": true,
		"--log-level": true, "-l": true,
		"--config": true,
		"--tlscacert": true,
		"--tlscert": true,
		"--tlskey": true,
	}
	// Docker global boolean flags
	dockerBoolFlags := map[string]bool{
		"--debug": true, "-D": true,
		"--tls": true, "--tlsverify": true,
	}

	result := []string{args[0]} // keep argv[0] (the binary name)
	i := 1
	pluginNameStripped := false

	for i < len(args) {
		arg := args[i]

		// Strip Docker global value flags (--flag value or --flag=value)
		if dockerValueFlags[arg] {
			i += 2 // skip flag and its value
			continue
		}
		if dockerBoolFlags[arg] {
			i++
			continue
		}
		// --flag=value form
		for flag := range dockerValueFlags {
			if strings.HasPrefix(arg, flag+"=") {
				i++
				goto next
			}
		}

		// Strip the plugin-name arg ("kforge") — only the first occurrence
		if !pluginNameStripped && arg == toolName {
			pluginNameStripped = true
			i++
			continue
		}

		result = append(result, arg)
	next:
		i++
	}
	return result
}
