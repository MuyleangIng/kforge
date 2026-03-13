package project

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type DeploySpec struct {
	Name         string
	ServiceName  string
	Port         int
	Healthcheck  string
	Command      []string
	Env          map[string]string
	Domains      []string
	RenderName   string
	RenderPlan   string
	RenderRegion string
	FlyApp       string
	FlyRegion    string
	FlyMemoryMB  int
}

func ResolveDeploySpec(d Detection, cfg Config) DeploySpec {
	name := d.SuggestedImageName()
	serviceName := name
	if cfg.Deploy.Compose.Service != "" {
		serviceName = sanitizeName(cfg.Deploy.Compose.Service)
	}
	port := d.Port
	if cfg.Deploy.Port > 0 {
		port = cfg.Deploy.Port
	}
	if port == 0 {
		port = 8080
	}
	healthcheck := d.HealthcheckPath
	if cfg.Deploy.Healthcheck != "" {
		healthcheck = cfg.Deploy.Healthcheck
	}
	if healthcheck == "" {
		healthcheck = "/"
	}
	command := d.StartCommand
	if len(cfg.Deploy.Command) > 0 {
		command = append([]string(nil), cfg.Deploy.Command...)
	}
	env := mergeStringMaps(d.EnvDefaults, cfg.Deploy.Env)
	if env == nil {
		env = map[string]string{}
	}
	if _, ok := env["PORT"]; !ok && port > 0 && d.Framework != FrameworkSpring {
		env["PORT"] = strconv.Itoa(port)
	}
	if d.Framework == FrameworkSpring {
		if _, ok := env["SERVER_PORT"]; !ok {
			env["SERVER_PORT"] = strconv.Itoa(port)
		}
	}

	renderName := name
	if cfg.Deploy.Render.Name != "" {
		renderName = sanitizeName(cfg.Deploy.Render.Name)
	}
	renderPlan := cfg.Deploy.Render.Plan
	if renderPlan == "" {
		renderPlan = "starter"
	}
	renderRegion := cfg.Deploy.Render.Region
	if renderRegion == "" {
		renderRegion = "oregon"
	}

	flyApp := name
	if cfg.Deploy.Fly.App != "" {
		flyApp = sanitizeName(cfg.Deploy.Fly.App)
	}
	flyRegion := cfg.Deploy.Fly.PrimaryRegion
	if flyRegion == "" {
		flyRegion = "iad"
	}
	flyMemory := cfg.Deploy.Fly.MemoryMB
	if flyMemory == 0 {
		flyMemory = 512
	}

	return DeploySpec{
		Name:         name,
		ServiceName:  serviceName,
		Port:         port,
		Healthcheck:  healthcheck,
		Command:      command,
		Env:          env,
		Domains:      append([]string(nil), cfg.Deploy.Domains...),
		RenderName:   renderName,
		RenderPlan:   renderPlan,
		RenderRegion: renderRegion,
		FlyApp:       flyApp,
		FlyRegion:    flyRegion,
		FlyMemoryMB:  flyMemory,
	}
}

func GenerateComposeFile(d Detection, cfg Config) string {
	spec := ResolveDeploySpec(d, cfg)

	var b strings.Builder
	b.WriteString("services:\n")
	b.WriteString("  " + spec.ServiceName + ":\n")
	b.WriteString("    build:\n")
	b.WriteString("      context: .\n")
	b.WriteString("      dockerfile: Dockerfile\n")
	b.WriteString("    image: " + spec.Name + ":dev\n")
	b.WriteString("    ports:\n")
	b.WriteString(fmt.Sprintf("      - \"${PORT:-%d}:%d\"\n", spec.Port, spec.Port))
	if len(spec.Env) > 0 {
		b.WriteString("    environment:\n")
		for _, line := range yamlMapLines(spec.Env, 6) {
			b.WriteString(line)
		}
	}
	if len(spec.Command) > 0 {
		b.WriteString("    command:\n")
		for _, arg := range spec.Command {
			b.WriteString("      - " + yamlScalar(arg) + "\n")
		}
	}
	if spec.Healthcheck != "" {
		b.WriteString("    healthcheck:\n")
		b.WriteString("      test:\n")
		for _, arg := range composeHealthcheckCommand(d, spec.Port, spec.Healthcheck) {
			b.WriteString("        - " + yamlScalar(arg) + "\n")
		}
		b.WriteString("      interval: 30s\n")
		b.WriteString("      timeout: 5s\n")
		b.WriteString("      retries: 3\n")
		b.WriteString("      start_period: 10s\n")
	}
	return b.String()
}

func GenerateRenderFile(d Detection, cfg Config) string {
	spec := ResolveDeploySpec(d, cfg)

	var b strings.Builder
	b.WriteString("services:\n")
	b.WriteString("  - type: web\n")
	b.WriteString("    name: " + spec.RenderName + "\n")
	b.WriteString("    runtime: docker\n")
	b.WriteString("    plan: " + spec.RenderPlan + "\n")
	b.WriteString("    region: " + spec.RenderRegion + "\n")
	b.WriteString("    dockerContext: .\n")
	b.WriteString("    dockerfilePath: ./Dockerfile\n")
	b.WriteString("    autoDeploy: true\n")
	if spec.Healthcheck != "" {
		b.WriteString("    healthCheckPath: " + spec.Healthcheck + "\n")
	}
	if len(spec.Env) > 0 {
		b.WriteString("    envVars:\n")
		keys := sortedKeys(spec.Env)
		for _, key := range keys {
			b.WriteString("      - key: " + key + "\n")
			b.WriteString("        value: " + yamlScalar(spec.Env[key]) + "\n")
		}
	}
	return b.String()
}

func GenerateFlyFile(d Detection, cfg Config) string {
	spec := ResolveDeploySpec(d, cfg)

	var b strings.Builder
	b.WriteString("app = " + tomlString(spec.FlyApp) + "\n")
	b.WriteString("primary_region = " + tomlString(spec.FlyRegion) + "\n\n")
	b.WriteString("[build]\n")
	b.WriteString("  dockerfile = \"Dockerfile\"\n\n")
	if len(spec.Env) > 0 {
		b.WriteString("[env]\n")
		for _, key := range sortedKeys(spec.Env) {
			b.WriteString("  " + key + " = " + tomlString(spec.Env[key]) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("[http_service]\n")
	b.WriteString(fmt.Sprintf("  internal_port = %d\n", spec.Port))
	b.WriteString("  force_https = true\n")
	b.WriteString("  auto_stop_machines = \"stop\"\n")
	b.WriteString("  auto_start_machines = true\n")
	b.WriteString("  min_machines_running = 0\n")
	b.WriteString("  processes = [\"app\"]\n\n")
	if spec.Healthcheck != "" {
		b.WriteString("  [[http_service.checks]]\n")
		b.WriteString("    interval = \"15s\"\n")
		b.WriteString("    timeout = \"5s\"\n")
		b.WriteString("    grace_period = \"10s\"\n")
		b.WriteString("    method = \"GET\"\n")
		b.WriteString("    path = " + tomlString(spec.Healthcheck) + "\n\n")
	}
	b.WriteString("[vm]\n")
	b.WriteString("  cpu_kind = \"shared\"\n")
	b.WriteString("  cpus = 1\n")
	b.WriteString(fmt.Sprintf("  memory = %q\n", fmt.Sprintf("%dmb", spec.FlyMemoryMB)))
	return b.String()
}

func yamlMapLines(values map[string]string, indent int) []string {
	keys := sortedKeys(values)
	prefix := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s%s: %s\n", prefix, key, yamlScalar(values[key])))
	}
	return lines
}

func yamlScalar(value string) string {
	return strconv.Quote(value)
}

func tomlString(value string) string {
	return strconv.Quote(value)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func composeHealthcheckCommand(d Detection, port int, path string) []string {
	switch d.Framework {
	case FrameworkSpring:
		return []string{"CMD-SHELL", fmt.Sprintf("curl -fsS http://127.0.0.1:%d%s >/dev/null || exit 1", port, path)}
	case FrameworkHTML:
		return []string{"CMD-SHELL", fmt.Sprintf("wget -qO- http://127.0.0.1:%d%s >/dev/null || exit 1", port, path)}
	case FrameworkFastAPI, FrameworkFlask, FrameworkDjango:
		return []string{"CMD", "python", "-c", fmt.Sprintf("import urllib.request; urllib.request.urlopen('http://127.0.0.1:%d%s', timeout=2).read()", port, path)}
	default:
		return []string{"CMD", "node", "-e", fmt.Sprintf("fetch('http://127.0.0.1:%d%s').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))", port, path)}
	}
}
