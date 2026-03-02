// Package builder manages kforge builder instances.
// Builders are stored in ~/.kforge/builders/ as JSON files.
// The active builder name is tracked in ~/.kforge/current.
package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/client"
)

// Config holds the configuration for a single builder instance.
type Config struct {
	Name     string   `json:"name"`
	Driver   string   `json:"driver"`   // "docker-container" or "remote"
	Endpoint string   `json:"endpoint"` // docker socket or remote address
	Platforms []string `json:"platforms,omitempty"`
}

// configDir returns the directory where builder configs are stored.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".mybuild", "builders")
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
	dir := filepath.Join(home, ".mybuild")
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
		return Config{}, fmt.Errorf("builder %q not found", name)
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
	name := string(data)
	if name == "" {
		return "default"
	}
	return name
}

// Connect boots a BuildKit client from the given builder config.
// For "docker-container" driver: connects to the buildkitd container.
// For "remote" driver: connects directly to the endpoint address.
func Connect(ctx context.Context, cfg Config) (*client.Client, error) {
	switch cfg.Driver {
	case "docker-container":
		// The docker-container driver exposes BuildKit via a Unix socket
		// inside the container. We connect through the Docker daemon's exec API.
		// For simplicity we connect to the well-known container socket path.
		addr := cfg.Endpoint
		if addr == "" {
			addr = "unix:///var/run/docker.sock"
		}
		return client.New(ctx, addr)

	case "remote":
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("remote driver requires an endpoint address")
		}
		return client.New(ctx, cfg.Endpoint)

	default:
		// Default: connect to local Docker daemon's BuildKit
		return client.New(ctx, "unix:///var/run/docker.sock")
	}
}
