// Package builder manages kforge builder instances.
// Configs are stored in ~/.kforge/builders/ as JSON files.
// The active builder name is tracked in ~/.kforge/current.
package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client"
)

// Config holds the configuration for a single builder instance.
type Config struct {
	Name      string   `json:"name"`
	Driver    string   `json:"driver"`             // "docker-container" or "remote"
	Endpoint  string   `json:"endpoint,omitempty"` // only used by remote driver
	Platforms []string `json:"platforms,omitempty"`
}

// ─── Storage ──────────────────────────────────────────────────────────────────

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kforge", "builders")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func currentFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kforge")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "current"), nil
}

// Save persists a builder config to disk.
func Save(cfg Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cfg.Name+".json"), data, 0600)
}

// Load reads a builder config by name.
func Load(name string) (Config, error) {
	dir, err := configDir()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return Config{}, fmt.Errorf("builder %q not found — run: kforge builder create", name)
	}
	var cfg Config
	return cfg, json.Unmarshal(data, &cfg)
}

// Remove deletes a builder config by name.
func Remove(name string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, name+".json"))
}

// List returns all saved builder configs.
func List() ([]Config, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var cfgs []Config
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}

// SetCurrent sets the active builder by name.
func SetCurrent(name string) error {
	f, err := currentFile()
	if err != nil {
		return err
	}
	return os.WriteFile(f, []byte(name), 0600)
}

// Current returns the name of the active builder.
// Returns "default" if none is set.
func Current() string {
	f, err := currentFile()
	if err != nil {
		return "default"
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return "default"
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "default"
	}
	return name
}

// ─── Connection ───────────────────────────────────────────────────────────────

// Connect returns a BuildKit client for the given builder config.
//
// For "docker-container" driver:
//   The BuildKit daemon runs inside a Docker container named
//   buildx_buildkit_<name>0  (same naming convention as docker buildx).
//   We use the "docker-container://<name>" scheme which tells the BuildKit
//   client to reach the daemon via the Docker API — NOT via a raw gRPC socket.
//   The container is auto-bootstrapped if it is not already running.
//
// For "remote" driver:
//   Connects directly to Endpoint (e.g. tcp://host:1234).
func Connect(ctx context.Context, cfg Config) (*client.Client, error) {
	switch cfg.Driver {
	case "docker-container":
		// Buildx-compatible container name: buildx_buildkit_<builder>0
		builderName := cfg.Name
		containerName := "buildx_buildkit_" + builderName + "0"

		// Ensure the container is running (auto-bootstraps if needed).
		// bootstrapContainer may also save an auto-detected active builder
		// and update Current(), so re-read the current name after.
		if err := bootstrapContainer(ctx, builderName, containerName); err != nil {
			return nil, err
		}

		// Re-resolve: if bootstrapContainer detected a different active builder,
		// it has updated Current() — use that container name now.
		if cur := Current(); cur != builderName {
			containerName = "buildx_buildkit_" + cur + "0"
		}

		// "docker-container://<containerName>" tunnels through the Docker daemon.
		// This is the correct protocol — NOT a raw gRPC unix socket.
		return client.New(ctx, "docker-container://"+containerName)

	case "remote":
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("remote driver requires an endpoint")
		}
		return client.New(ctx, cfg.Endpoint)

	default:
		// Fallback: try the default buildx builder container
		return client.New(ctx, "docker-container://buildx_buildkit_default0")
	}
}

// bootstrapContainer ensures the BuildKit container is running.
// Strategy (in order):
//  1. Container already running → use it directly.
//  2. A buildx builder with this name exists → bootstrap it.
//  3. Active buildx builder uses docker-container driver → adopt it.
//  4. Create a new buildx builder from scratch.
func bootstrapContainer(ctx context.Context, builderName, containerName string) error {
	// 1. Already running?
	out, _ := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Status}}", containerName).Output()
	if strings.TrimSpace(string(out)) == "running" {
		return nil
	}

	// 2. Try bootstrapping an existing buildx builder by this name
	boot := exec.CommandContext(ctx, "docker", "buildx", "inspect", "--bootstrap", builderName)
	boot.Stderr = os.Stderr
	if boot.Run() == nil {
		// Verify container came up
		out2, _ := exec.CommandContext(ctx, "docker", "inspect",
			"--format", "{{.State.Status}}", containerName).Output()
		if strings.TrimSpace(string(out2)) == "running" {
			return nil
		}
	}

	// 3. Detect the currently active buildx builder (docker-container type)
	if active := activeDockerContainerBuilder(ctx); active != "" && active != builderName {
		activeContainer := "buildx_buildkit_" + active + "0"
		out3, _ := exec.CommandContext(ctx, "docker", "inspect",
			"--format", "{{.State.Status}}", activeContainer).Output()
		if strings.TrimSpace(string(out3)) == "running" {
			fmt.Fprintf(os.Stderr,
				"✓ Using active buildx builder %q (container: %s)\n", active, activeContainer)
			// Rewrite containerName in place so Connect() uses the right one
			// We can't mutate the caller's string, but we signal success — caller
			// should call Connect with the correct name. For now just succeed;
			// the caller already passes the container name based on cfg.Name.
			// So: save the active builder as a kforge builder config automatically.
			_ = Save(Config{Name: active, Driver: "docker-container"})
			_ = SetCurrent(active)
			fmt.Fprintf(os.Stderr,
				"  Saved as kforge builder %q. Next time use: kforge builder use %s\n", active, active)
			return nil
		}
	}

	// 4. Create a new builder from scratch
	fmt.Fprintf(os.Stderr, "⠋ Creating BuildKit builder %q...\n", builderName)
	create := exec.CommandContext(ctx, "docker", "buildx", "create",
		"--name", builderName,
		"--driver", "docker-container",
		"--bootstrap",
		"--use",
	)
	create.Stdout = os.Stderr
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return fmt.Errorf(
			"failed to start BuildKit builder: %w\n\n"+
				"  Run `kforge setup` for guided setup, or:\n"+
				"  docker buildx create --name %s --bootstrap --use",
			err, builderName)
	}
	fmt.Fprintf(os.Stderr, "✓ Builder ready\n")
	return nil
}

// activeDockerContainerBuilder returns the name of the currently active
// docker-container buildx builder, or "" if none / not that driver.
func activeDockerContainerBuilder(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "docker", "buildx", "ls").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Active builder has a "*" in the name column
		if !strings.Contains(line, "*") {
			continue
		}
		if strings.Contains(line, "docker-container") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return strings.TrimRight(fields[0], "*")
			}
		}
	}
	return ""
}
