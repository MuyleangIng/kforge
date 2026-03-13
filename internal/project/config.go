package project

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Framework      string            `yaml:"framework"`
	Image          string            `yaml:"image"`
	Node           string            `yaml:"node"`
	Python         string            `yaml:"python"`
	Java           string            `yaml:"java"`
	Port           int               `yaml:"port"`
	Healthcheck    string            `yaml:"healthcheck"`
	AppModule      string            `yaml:"app_module"`
	SettingsModule string            `yaml:"settings_module"`
	StartCommand   []string          `yaml:"start_command"`
	Env            map[string]string `yaml:"env"`
	Verify         VerifyConfig      `yaml:"verify"`
	Deploy         DeployConfig      `yaml:"deploy"`
	CI             CIConfig          `yaml:"ci"`
}

type VerifyConfig struct {
	Path           string            `yaml:"path"`
	Port           int               `yaml:"port"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
	Env            map[string]string `yaml:"env"`
}

type DeployConfig struct {
	Port        int               `yaml:"port"`
	Healthcheck string            `yaml:"healthcheck"`
	Command     []string          `yaml:"command"`
	Domains     []string          `yaml:"domains"`
	Env         map[string]string `yaml:"env"`
	Compose     ComposeConfig     `yaml:"compose"`
	Render      RenderConfig      `yaml:"render"`
	Fly         FlyConfig         `yaml:"fly"`
}

type ComposeConfig struct {
	Service string `yaml:"service"`
}

type RenderConfig struct {
	Name   string `yaml:"name"`
	Plan   string `yaml:"plan"`
	Region string `yaml:"region"`
}

type FlyConfig struct {
	App           string `yaml:"app"`
	PrimaryRegion string `yaml:"primary_region"`
	MemoryMB      int    `yaml:"memory_mb"`
}

type CIConfig struct {
	Image      string         `yaml:"image"`
	MainBranch string         `yaml:"main_branch"`
	Context    string         `yaml:"context"`
	Platforms  []string       `yaml:"platforms"`
	Auto       *bool          `yaml:"auto"`
	Verify     *bool          `yaml:"verify"`
	Push       *bool          `yaml:"push"`
	Deploy     string         `yaml:"deploy"`
	DeployPath string         `yaml:"deploy_path"`
	GitHub     GitHubCIConfig `yaml:"github"`
	GitLab     GitLabCIConfig `yaml:"gitlab"`
}

type GitHubCIConfig struct {
	Workflow string `yaml:"workflow"`
}

type GitLabCIConfig struct {
	File string `yaml:"file"`
}

func LoadConfig(root string) (Config, string, error) {
	for _, name := range []string{".kforge.yml", ".kforge.yaml"} {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Config{}, "", err
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, path, err
		}
		return cfg, path, nil
	}
	return Config{}, "", nil
}
