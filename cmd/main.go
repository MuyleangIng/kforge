package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MuyleangIng/kforge/commands"
	"github.com/MuyleangIng/kforge/internal/meta"
	"github.com/spf13/cobra"
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
		pluginMeta := pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           meta.Vendor,
			Version:          meta.DisplayVersion(),
			ShortDescription: "Personal multi-platform image builder powered by BuildKit",
			URL:              meta.URL,
		}
		if err := json.NewEncoder(os.Stdout).Encode(pluginMeta); err != nil {
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
		Use:   meta.ToolName,
		Short: "kforge — personal multi-platform image builder",
		Long: `kforge — Personal multi-platform image builder · KhmerStack · Ing Muyleang

  ★  Quickest way to build + push:
       kforge push muyleangin/myapp .
       kforge push ghcr.io/muyleangin/myapp .
       docker kforge push muyleangin/myapp .

  ★  Full build control:
       kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/app .

  ★  Login to any registry:
       kforge login                   (Docker Hub)
       kforge login ghcr.io
       kforge login myregistry.io

  ★  Environment checks:
       kforge doctor

  ★  Starter project:
       kforge init --name myapp
       kforge init --detect

  ★  Detect existing apps:
       kforge detect
       kforge build --auto .

  ★  Build + run + check:
       kforge verify
       kforge verify --path /health

  ★  CI/CD bootstrap:
       kforge ci init
       kforge ci init --target github

  ★  Deploy bootstrap:
       kforge deploy init
       kforge deploy init --target compose

  ★  First time setup:
       kforge setup`,
		Version:       meta.DisplayVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		commands.PushCmd(),
		commands.BuildCmd(),
		commands.BakeCmd(),
		commands.LoginCmd(),
		commands.BuilderCmd(),
		commands.DoctorCmd(),
		commands.DetectCmd(),
		commands.CICmd(),
		commands.DeployCmd(),
		commands.InitCmd(),
		commands.VerifyCmd(),
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
		"--config":    true,
		"--tlscacert": true,
		"--tlscert":   true,
		"--tlskey":    true,
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
		if !pluginNameStripped && arg == meta.ToolName {
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
