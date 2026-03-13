package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MuyleangIng/kforge/internal/meta"
	"github.com/spf13/cobra"
)

// LoginCmd returns the `kforge login` command.
func LoginCmd() *cobra.Command {
	var username string

	cmd := &cobra.Command{
		Use:   "login [REGISTRY]",
		Short: "Log in to a container registry",
		Long: `Log in to a container registry with a styled kforge UI.

Default registry: Docker Hub

Supported registries:
  Docker Hub    kforge login
  GHCR          kforge login ghcr.io
  ECR           kforge login 123456.dkr.ecr.us-east-1.amazonaws.com
  ACR           kforge login myacr.azurecr.io
  Custom        kforge login registry.myco.com`,
		Example: `  kforge login
  kforge login ghcr.io
  kforge login ghcr.io -u muyleangin
  kforge login myregistry.io`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := ""
			if len(args) > 0 {
				registry = args[0]
			}
			return runLogin(cmd.Context(), registry, username)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Username (optional)")
	return cmd
}

func runLogin(ctx context.Context, registry, username string) error {
	regName, regLabel := registryInfo(registry)

	printLoginHeader(regName, regLabel)

	args := []string{"login"}
	if username != "" {
		args = append(args, "-u", username)
	}
	if registry != "" {
		args = append(args, registry)
	}

	start := time.Now()
	dockerCmd := exec.CommandContext(ctx, "docker", args...)
	dockerCmd.Stdin = os.Stdin
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if err := dockerCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s✗ Login failed%s\n", cRed+cBold, cReset)
		return fmt.Errorf("login failed: %w", err)
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	printLoginFooter(regLabel, elapsed)
	return nil
}

// registryInfo returns (raw host, display label) for a registry address.
func registryInfo(registry string) (string, string) {
	if registry == "" {
		return "docker.io", "Docker Hub"
	}
	r := strings.ToLower(registry)
	switch {
	case r == "ghcr.io":
		return registry, "GitHub Container Registry"
	case strings.Contains(r, ".dkr.ecr.") && strings.Contains(r, ".amazonaws.com"):
		return registry, "AWS ECR"
	case strings.HasSuffix(r, ".azurecr.io"):
		return registry, "Azure Container Registry"
	case r == "gcr.io" || strings.HasSuffix(r, ".pkg.dev"):
		return registry, "Google Container Registry"
	default:
		return registry, registry
	}
}

func printLoginHeader(registry, label string) {
	top := cCyan + "╔" + strings.Repeat("═", bannerWidth) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", bannerWidth) + "╝" + cReset

	title := cBold + cWhite + "KFORGE LOGIN" + cReset +
		"  " + cDim + meta.DisplayVersion() + " · KhmerStack" + cReset

	fmt.Println()
	fmt.Println(top)
	fmt.Println(boxLine(cCyan + "  " + cReset + title))
	fmt.Println(cCyan + "║" + strings.Repeat("─", bannerWidth) + "║" + cReset)
	fmt.Println(boxLine(cDim + "  Registry  " + cReset + cBlue + cBold + label + cReset))
	if registry != "docker.io" && registry != "" {
		fmt.Println(boxLine(cDim + "  Host      " + cReset + registry))
	}
	fmt.Println(bot)
	fmt.Println()
}

func printLoginFooter(label string, elapsed time.Duration) {
	top := cCyan + "╔" + strings.Repeat("═", bannerWidth) + "╗" + cReset
	bot := cCyan + "╚" + strings.Repeat("═", bannerWidth) + "╝" + cReset
	msg := cGreen + cBold + "✦  Logged in to " + label + cReset +
		"  " + cDim + elapsed.String() + cReset

	fmt.Println()
	fmt.Println(top)
	fmt.Println(boxLine(msg))
	fmt.Println(bot)
	fmt.Println()
}
