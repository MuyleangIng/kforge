package project

import (
	"strings"
	"testing"
)

func TestGenerateComposeFile(t *testing.T) {
	d := Detection{
		Name:            "flask-demo",
		Framework:       FrameworkFlask,
		Port:            8000,
		HealthcheckPath: "/health",
		EnvDefaults: map[string]string{
			"APP_ENV": "staging",
		},
		StartCommand: []string{"gunicorn", "--bind", "0.0.0.0:8000", "app:app"},
	}
	cfg := Config{
		Deploy: DeployConfig{
			Compose: ComposeConfig{Service: "web"},
		},
	}

	compose := GenerateComposeFile(d, cfg)
	for _, want := range []string{
		"services:",
		"  web:",
		`    image: flask-demo:dev`,
		`      - "${PORT:-8000}:8000"`,
		"    healthcheck:",
		`        - "CMD"`,
		`      APP_ENV: "staging"`,
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("expected compose file to contain %q\n%s", want, compose)
		}
	}
}

func TestGenerateRenderFile(t *testing.T) {
	d := Detection{
		Name:            "api-demo",
		Framework:       FrameworkFastAPI,
		Port:            8000,
		HealthcheckPath: "/health",
		EnvDefaults: map[string]string{
			"APP_ENV": "production",
		},
	}
	cfg := Config{
		Deploy: DeployConfig{
			Render: RenderConfig{
				Plan:   "starter",
				Region: "frankfurt",
			},
		},
	}

	render := GenerateRenderFile(d, cfg)
	for _, want := range []string{
		"name: api-demo",
		"runtime: docker",
		"healthCheckPath: /health",
		"plan: starter",
		"region: frankfurt",
		"key: APP_ENV",
	} {
		if !strings.Contains(render, want) {
			t.Fatalf("expected render file to contain %q\n%s", want, render)
		}
	}
}

func TestGenerateFlyFile(t *testing.T) {
	d := Detection{
		Name:            "spring-demo",
		Framework:       FrameworkSpring,
		Port:            8080,
		HealthcheckPath: "/actuator/health",
		EnvDefaults: map[string]string{
			"SPRING_PROFILES_ACTIVE": "prod",
		},
	}
	cfg := Config{
		Deploy: DeployConfig{
			Fly: FlyConfig{
				App:           "spring-prod",
				PrimaryRegion: "sin",
				MemoryMB:      1024,
			},
		},
	}

	fly := GenerateFlyFile(d, cfg)
	for _, want := range []string{
		`app = "spring-prod"`,
		`primary_region = "sin"`,
		`internal_port = 8080`,
		`path = "/actuator/health"`,
		`memory = "1024mb"`,
	} {
		if !strings.Contains(fly, want) {
			t.Fatalf("expected fly.toml to contain %q\n%s", want, fly)
		}
	}
}
