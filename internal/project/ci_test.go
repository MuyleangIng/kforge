package project

import (
	"strings"
	"testing"
)

func TestGenerateGitHubActionsWorkflow(t *testing.T) {
	auto := true
	verify := true
	push := true
	d := Detection{
		Name:            "fastapi-demo",
		Framework:       FrameworkFastAPI,
		Port:            8000,
		HealthcheckPath: "/health",
		VerifyPath:      "/health",
	}
	cfg := Config{
		CI: CIConfig{
			MainBranch: "main",
			Platforms:  []string{"linux/amd64", "linux/arm64"},
			Auto:       &auto,
			Verify:     &verify,
			Push:       &push,
			Deploy:     "render",
		},
	}

	workflow := GenerateGitHubActionsWorkflow(d, cfg)
	for _, want := range []string{
		"name: kforge-ci",
		"workflow_dispatch:",
		`KFORGE_IMAGE: "ghcr.io/${{ github.repository_owner }}/fastapi-demo"`,
		`KFORGE_PLATFORMS: "linux/amd64,linux/arm64"`,
		"uses: docker/setup-buildx-action@v3",
		`run: kforge detect "$KFORGE_CONTEXT"`,
		`kforge verify --progress plain "$KFORGE_CONTEXT"`,
		`uses: docker/login-action@v3`,
		`curl -fsSL -X POST "$RENDER_DEPLOY_HOOK_URL"`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("expected GitHub workflow to contain %q\n%s", want, workflow)
		}
	}
}

func TestGenerateGitLabCI(t *testing.T) {
	auto := true
	verify := true
	push := true
	d := Detection{
		Name:            "spring-demo",
		Framework:       FrameworkSpring,
		Port:            8080,
		HealthcheckPath: "/actuator/health",
		VerifyPath:      "/actuator/health",
	}
	cfg := Config{
		CI: CIConfig{
			MainBranch: "main",
			Auto:       &auto,
			Verify:     &verify,
			Push:       &push,
			Deploy:     "compose",
			DeployPath: "/srv/spring-demo",
		},
	}

	pipeline := GenerateGitLabCI(d, cfg)
	for _, want := range []string{
		"stages:",
		".kforge_docker_job:",
		`KFORGE_IMAGE: "$CI_REGISTRY_IMAGE"`,
		`KFORGE_DEPLOY_PATH: "/srv/spring-demo"`,
		`kforge verify --progress plain "$KFORGE_CONTEXT"`,
		`echo "$CI_REGISTRY_PASSWORD" | docker login "$CI_REGISTRY" -u "$CI_REGISTRY_USER" --password-stdin`,
		`rsync -az --delete --exclude .git ./ "$DEPLOY_USER@$DEPLOY_HOST:$KFORGE_DEPLOY_PATH/"`,
		`docker compose up -d --build --remove-orphans`,
	} {
		if !strings.Contains(pipeline, want) {
			t.Fatalf("expected GitLab CI to contain %q\n%s", want, pipeline)
		}
	}
}
