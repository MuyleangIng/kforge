package project

import (
	"strings"

	"github.com/MuyleangIng/kforge/internal/meta"
)

type CISpec struct {
	Name               string
	ImageName          string
	Context            string
	Image              string
	Registry           string
	MainBranch         string
	Platforms          []string
	Auto               bool
	Verify             bool
	Push               bool
	VerifyPath         string
	DeployTarget       string
	DeployPath         string
	GitHubWorkflowFile string
	GitLabFile         string
	KforgeVersion      string
}

func ResolveCISpec(d Detection, cfg Config, provider string) CISpec {
	spec := CISpec{
		Name:               d.SuggestedImageName(),
		ImageName:          d.SuggestedImageName(),
		Context:            ".",
		MainBranch:         "main",
		Platforms:          []string{"linux/amd64"},
		Auto:               true,
		Verify:             true,
		Push:               true,
		VerifyPath:         d.VerifyPath,
		DeployTarget:       normalizeCIDeployTarget(cfg.CI.Deploy),
		DeployPath:         "/srv/" + d.SuggestedImageName(),
		GitHubWorkflowFile: "kforge-ci.yml",
		GitLabFile:         ".gitlab-ci.yml",
		KforgeVersion:      meta.DownloadVersion(),
	}
	if spec.VerifyPath == "" {
		spec.VerifyPath = d.HealthcheckPath
	}
	if spec.VerifyPath == "" {
		spec.VerifyPath = "/"
	}
	if cfg.CI.Context != "" {
		spec.Context = cfg.CI.Context
	}
	if cfg.CI.MainBranch != "" {
		spec.MainBranch = cfg.CI.MainBranch
	}
	if len(cfg.CI.Platforms) > 0 {
		spec.Platforms = append([]string(nil), cfg.CI.Platforms...)
	}
	if cfg.CI.Auto != nil {
		spec.Auto = *cfg.CI.Auto
	}
	if cfg.CI.Verify != nil {
		spec.Verify = *cfg.CI.Verify
	}
	if cfg.CI.Push != nil {
		spec.Push = *cfg.CI.Push
	}
	if cfg.CI.DeployPath != "" {
		spec.DeployPath = cfg.CI.DeployPath
	}
	if cfg.CI.GitHub.Workflow != "" {
		spec.GitHubWorkflowFile = cfg.CI.GitHub.Workflow
	}
	if cfg.CI.GitLab.File != "" {
		spec.GitLabFile = cfg.CI.GitLab.File
	}

	switch strings.ToLower(provider) {
	case "gitlab":
		if cfg.CI.Image != "" {
			spec.Image = cfg.CI.Image
			spec.Registry = registryHost(cfg.CI.Image)
		} else {
			spec.Image = "$CI_REGISTRY_IMAGE"
			spec.Registry = "$CI_REGISTRY"
		}
	default:
		if cfg.CI.Image != "" {
			spec.Image = cfg.CI.Image
			spec.Registry = registryHost(cfg.CI.Image)
		} else {
			spec.Image = ""
			spec.Registry = "ghcr.io"
		}
	}

	return spec
}

func GenerateGitHubActionsWorkflow(d Detection, cfg Config) string {
	spec := ResolveCISpec(d, cfg, "github")

	var b strings.Builder
	b.WriteString("name: kforge-ci\n\n")
	b.WriteString("on:\n")
	b.WriteString("  pull_request:\n")
	b.WriteString("  push:\n")
	b.WriteString("    branches:\n")
	b.WriteString("      - " + yamlScalar(spec.MainBranch) + "\n")
	b.WriteString("    tags:\n")
	b.WriteString("      - \"v*\"\n")
	b.WriteString("  workflow_dispatch:\n\n")
	b.WriteString("concurrency:\n")
	b.WriteString("  group: kforge-${{ github.workflow }}-${{ github.ref }}\n")
	b.WriteString("  cancel-in-progress: true\n\n")
	b.WriteString("env:\n")
	for _, line := range yamlMapLines(map[string]string{
		"KFORGE_CONTEXT":       spec.Context,
		"KFORGE_DEPLOY_PATH":   spec.DeployPath,
		"KFORGE_DEPLOY_TARGET": spec.DeployTarget,
		"KFORGE_IMAGE":         spec.Image,
		"KFORGE_IMAGE_NAME":    spec.ImageName,
		"KFORGE_MAIN_BRANCH":   spec.MainBranch,
		"KFORGE_PLATFORMS":     strings.Join(spec.Platforms, ","),
		"KFORGE_REGISTRY":      spec.Registry,
		"KFORGE_VERIFY_PATH":   spec.VerifyPath,
		"KFORGE_VERSION":       spec.KforgeVersion,
	}, 2) {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString("jobs:\n")
	b.WriteString(githubCheckJob(spec))
	if spec.Push {
		b.WriteString("\n")
		b.WriteString(githubPublishJob(spec))
	}
	if spec.DeployTarget != "" && spec.DeployTarget != "none" {
		b.WriteString("\n")
		b.WriteString(githubDeployJob(spec))
	}
	return b.String()
}

func GenerateGitLabCI(d Detection, cfg Config) string {
	spec := ResolveCISpec(d, cfg, "gitlab")

	var b strings.Builder
	b.WriteString("stages:\n")
	b.WriteString("  - check\n")
	if spec.Push {
		b.WriteString("  - publish\n")
	}
	if spec.DeployTarget != "" && spec.DeployTarget != "none" {
		b.WriteString("  - deploy\n")
	}
	b.WriteString("\nvariables:\n")
	for _, line := range yamlMapLines(map[string]string{
		"DOCKER_BUILDKIT":      "1",
		"DOCKER_HOST":          "tcp://docker:2375",
		"DOCKER_TLS_CERTDIR":   "",
		"KFORGE_CONTEXT":       spec.Context,
		"KFORGE_DEPLOY_PATH":   spec.DeployPath,
		"KFORGE_DEPLOY_TARGET": spec.DeployTarget,
		"KFORGE_IMAGE":         spec.Image,
		"KFORGE_MAIN_BRANCH":   spec.MainBranch,
		"KFORGE_PLATFORMS":     strings.Join(spec.Platforms, ","),
		"KFORGE_REGISTRY":      spec.Registry,
		"KFORGE_VERIFY_PATH":   spec.VerifyPath,
		"KFORGE_VERSION":       spec.KforgeVersion,
	}, 2) {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(".kforge_docker_job:\n")
	b.WriteString("  image: docker:27-cli\n")
	b.WriteString("  services:\n")
	b.WriteString("    - docker:27-dind\n")
	b.WriteString("  before_script:\n")
	writeBlock(&b, 4, []string{
		"- apk add --no-cache bash curl tar",
		"- mkdir -p \"$HOME/.local/bin\"",
		"- curl -fsSL -o /tmp/kforge.tar.gz \"https://github.com/MuyleangIng/kforge/releases/download/${KFORGE_VERSION}/kforge_${KFORGE_VERSION#v}_linux_amd64.tar.gz\"",
		"- tar -xzf /tmp/kforge.tar.gz -C /tmp",
		"- cp /tmp/kforge \"$HOME/.local/bin/kforge\"",
		"- chmod +x \"$HOME/.local/bin/kforge\"",
		"- export PATH=\"$HOME/.local/bin:$PATH\"",
		"- kforge version",
	})
	b.WriteString("\n")
	b.WriteString(gitlabCheckJob(spec))
	if spec.Push {
		b.WriteString("\n")
		b.WriteString(gitlabPublishJob(spec))
	}
	if spec.DeployTarget != "" && spec.DeployTarget != "none" {
		b.WriteString("\n")
		b.WriteString(gitlabDeployJob(spec))
	}
	return b.String()
}

func githubCheckJob(spec CISpec) string {
	var b strings.Builder
	b.WriteString("  check:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    permissions:\n")
	b.WriteString("      contents: read\n")
	b.WriteString("      packages: write\n")
	b.WriteString("    steps:\n")
	writeBlock(&b, 6, []string{
		"- name: Check out source",
		"  uses: actions/checkout@v4",
		"- name: Set up QEMU",
		"  uses: docker/setup-qemu-action@v3",
		"- name: Set up Docker Buildx",
		"  uses: docker/setup-buildx-action@v3",
		"- name: Install kforge",
		"  shell: bash",
		"  run: |",
	})
	writeBlock(&b, 10, []string{
		"mkdir -p \"$HOME/.local/bin\"",
		"curl -fsSL -o /tmp/kforge.tar.gz \"https://github.com/MuyleangIng/kforge/releases/download/${KFORGE_VERSION}/kforge_${KFORGE_VERSION#v}_linux_amd64.tar.gz\"",
		"tar -xzf /tmp/kforge.tar.gz -C /tmp",
		"cp /tmp/kforge \"$HOME/.local/bin/kforge\"",
		"chmod +x \"$HOME/.local/bin/kforge\"",
		"echo \"$HOME/.local/bin\" >> \"$GITHUB_PATH\"",
		"export PATH=\"$HOME/.local/bin:$PATH\"",
		"kforge version",
	})
	writeBlock(&b, 6, []string{
		"- name: Resolve image name",
		"  shell: bash",
		"  run: |",
	})
	writeBlock(&b, 10, githubResolveImageCommands(spec))
	writeBlock(&b, 6, []string{
		"- name: Inspect project",
		"  run: kforge detect \"$KFORGE_CONTEXT\"",
		"- name: " + githubCheckStepName(spec),
		"  run: |",
	})
	writeBlock(&b, 10, githubCheckCommands(spec))
	return b.String()
}

func githubPublishJob(spec CISpec) string {
	var b strings.Builder
	b.WriteString("  publish:\n")
	b.WriteString("    if: github.event_name == 'push'\n")
	b.WriteString("    needs: check\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    permissions:\n")
	b.WriteString("      contents: read\n")
	b.WriteString("      packages: write\n")
	b.WriteString("    steps:\n")
	writeBlock(&b, 6, []string{
		"- name: Check out source",
		"  uses: actions/checkout@v4",
		"- name: Set up QEMU",
		"  uses: docker/setup-qemu-action@v3",
		"- name: Set up Docker Buildx",
		"  uses: docker/setup-buildx-action@v3",
		"- name: Install kforge",
		"  shell: bash",
		"  run: |",
	})
	writeBlock(&b, 10, []string{
		"mkdir -p \"$HOME/.local/bin\"",
		"curl -fsSL -o /tmp/kforge.tar.gz \"https://github.com/MuyleangIng/kforge/releases/download/${KFORGE_VERSION}/kforge_${KFORGE_VERSION#v}_linux_amd64.tar.gz\"",
		"tar -xzf /tmp/kforge.tar.gz -C /tmp",
		"cp /tmp/kforge \"$HOME/.local/bin/kforge\"",
		"chmod +x \"$HOME/.local/bin/kforge\"",
		"echo \"$HOME/.local/bin\" >> \"$GITHUB_PATH\"",
		"export PATH=\"$HOME/.local/bin:$PATH\"",
		"kforge version",
	})
	writeBlock(&b, 6, []string{
		"- name: Resolve image name",
		"  shell: bash",
		"  run: |",
	})
	writeBlock(&b, 10, githubResolveImageCommands(spec))
	writeBlock(&b, 6, []string{
		"- name: Log in to GHCR",
		"  if: env.KFORGE_REGISTRY == 'ghcr.io'",
		"  uses: docker/login-action@v3",
		"  with:",
		"    registry: ghcr.io",
		"    username: ${{ github.actor }}",
		"    password: ${{ secrets.GITHUB_TOKEN }}",
		"- name: Log in to custom registry",
		"  if: env.KFORGE_REGISTRY != 'ghcr.io'",
		"  uses: docker/login-action@v3",
		"  with:",
		"    registry: ${{ env.KFORGE_REGISTRY }}",
		"    username: ${{ secrets.KFORGE_REGISTRY_USER }}",
		"    password: ${{ secrets.KFORGE_REGISTRY_PASSWORD }}",
		"- name: Push image",
		"  shell: bash",
		"  run: |",
	})
	writeBlock(&b, 10, githubPublishCommands(spec))
	return b.String()
}

func githubDeployJob(spec CISpec) string {
	var b strings.Builder
	jobName := "deploy_" + spec.DeployTarget
	b.WriteString("  " + jobName + ":\n")
	b.WriteString("    if: github.event_name == 'push' && github.ref_name == env.KFORGE_MAIN_BRANCH\n")
	b.WriteString("    needs: " + ciDependencyJob(spec) + "\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	switch spec.DeployTarget {
	case "compose":
		writeBlock(&b, 6, []string{
			"- name: Check out source",
			"  uses: actions/checkout@v4",
			"- name: Configure SSH",
			"  shell: bash",
			"  env:",
			"    DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}",
			"    DEPLOY_USER: ${{ secrets.DEPLOY_USER }}",
			"    DEPLOY_SSH_KEY: ${{ secrets.DEPLOY_SSH_KEY }}",
			"  run: |",
		})
		writeBlock(&b, 10, []string{
			"test -n \"$DEPLOY_HOST\"",
			"test -n \"$DEPLOY_USER\"",
			"test -n \"$DEPLOY_SSH_KEY\"",
			"install -m 700 -d ~/.ssh",
			"printf '%s\\n' \"$DEPLOY_SSH_KEY\" > ~/.ssh/id_ed25519",
			"chmod 600 ~/.ssh/id_ed25519",
			"ssh-keyscan -H \"$DEPLOY_HOST\" >> ~/.ssh/known_hosts",
		})
		writeBlock(&b, 6, []string{
			"- name: Sync project",
			"  shell: bash",
			"  env:",
			"    DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}",
			"    DEPLOY_USER: ${{ secrets.DEPLOY_USER }}",
			"  run: rsync -az --delete --exclude .git ./ \"$DEPLOY_USER@$DEPLOY_HOST:$KFORGE_DEPLOY_PATH/\"",
			"- name: Restart app",
			"  shell: bash",
			"  env:",
			"    DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}",
			"    DEPLOY_USER: ${{ secrets.DEPLOY_USER }}",
			"  run: ssh \"$DEPLOY_USER@$DEPLOY_HOST\" \"cd $KFORGE_DEPLOY_PATH && docker compose up -d --build --remove-orphans\"",
		})
	case "render":
		writeBlock(&b, 6, []string{
			"- name: Trigger Render deploy",
			"  env:",
			"    RENDER_DEPLOY_HOOK_URL: ${{ secrets.RENDER_DEPLOY_HOOK_URL }}",
			"  run: curl -fsSL -X POST \"$RENDER_DEPLOY_HOOK_URL\"",
		})
	case "fly":
		writeBlock(&b, 6, []string{
			"- name: Check out source",
			"  uses: actions/checkout@v4",
			"- name: Set up flyctl",
			"  uses: superfly/flyctl-actions/setup-flyctl@master",
			"- name: Deploy to Fly.io",
			"  env:",
			"    FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}",
			"  run: flyctl deploy --config fly.toml --remote-only",
		})
	}
	return b.String()
}

func gitlabCheckJob(spec CISpec) string {
	var b strings.Builder
	b.WriteString("check:\n")
	b.WriteString("  stage: check\n")
	b.WriteString("  extends: .kforge_docker_job\n")
	b.WriteString("  script:\n")
	writeBlock(&b, 4, append([]string{"- kforge detect \"$KFORGE_CONTEXT\""}, gitlabCheckCommands(spec)...))
	return b.String()
}

func gitlabPublishJob(spec CISpec) string {
	var b strings.Builder
	b.WriteString("publish:\n")
	b.WriteString("  stage: publish\n")
	b.WriteString("  extends: .kforge_docker_job\n")
	b.WriteString("  needs:\n")
	b.WriteString("    - check\n")
	b.WriteString("  rules:\n")
	b.WriteString("    - if: '$CI_COMMIT_BRANCH == $KFORGE_MAIN_BRANCH'\n")
	b.WriteString("    - if: '$CI_COMMIT_TAG'\n")
	b.WriteString("  script:\n")
	writeBlock(&b, 4, []string{
		"- |",
	})
	writeBlock(&b, 6, []string{
		"if [ \"$KFORGE_REGISTRY\" = \"$CI_REGISTRY\" ]; then",
		"  echo \"$CI_REGISTRY_PASSWORD\" | docker login \"$CI_REGISTRY\" -u \"$CI_REGISTRY_USER\" --password-stdin",
		"else",
		"  echo \"$KFORGE_REGISTRY_PASSWORD\" | docker login \"$KFORGE_REGISTRY\" -u \"$KFORGE_REGISTRY_USER\" --password-stdin",
		"fi",
	})
	writeBlock(&b, 4, []string{
		"- |",
	})
	writeBlock(&b, 6, gitlabPublishCommands(spec))
	return b.String()
}

func gitlabDeployJob(spec CISpec) string {
	var b strings.Builder
	jobName := "deploy_" + spec.DeployTarget
	b.WriteString(jobName + ":\n")
	b.WriteString("  stage: deploy\n")
	b.WriteString("  needs:\n")
	b.WriteString("    - " + ciDependencyJob(spec) + "\n")
	b.WriteString("  rules:\n")
	b.WriteString("    - if: '$CI_COMMIT_BRANCH == $KFORGE_MAIN_BRANCH'\n")
	switch spec.DeployTarget {
	case "compose":
		b.WriteString("  image: alpine:3.20\n")
		b.WriteString("  before_script:\n")
		writeBlock(&b, 4, []string{
			"- apk add --no-cache openssh-client rsync",
			"- install -m 700 -d ~/.ssh",
			"- printf '%s\\n' \"$DEPLOY_SSH_KEY\" > ~/.ssh/id_ed25519",
			"- chmod 600 ~/.ssh/id_ed25519",
			"- ssh-keyscan -H \"$DEPLOY_HOST\" >> ~/.ssh/known_hosts",
		})
		b.WriteString("  script:\n")
		writeBlock(&b, 4, []string{
			"- rsync -az --delete --exclude .git ./ \"$DEPLOY_USER@$DEPLOY_HOST:$KFORGE_DEPLOY_PATH/\"",
			"- ssh \"$DEPLOY_USER@$DEPLOY_HOST\" \"cd $KFORGE_DEPLOY_PATH && docker compose up -d --build --remove-orphans\"",
		})
	case "render":
		b.WriteString("  image: curlimages/curl:8.7.1\n")
		b.WriteString("  script:\n")
		writeBlock(&b, 4, []string{
			"- curl -fsSL -X POST \"$RENDER_DEPLOY_HOOK_URL\"",
		})
	case "fly":
		b.WriteString("  image: flyio/flyctl:latest\n")
		b.WriteString("  script:\n")
		writeBlock(&b, 4, []string{
			"- flyctl deploy --config fly.toml --remote-only",
		})
	}
	return b.String()
}

func githubCheckStepName(spec CISpec) string {
	if spec.Verify {
		return "Verify app"
	}
	return "Build smoke image"
}

func githubCheckCommands(spec CISpec) []string {
	if spec.Verify {
		command := "kforge verify --progress plain"
		if !spec.Auto {
			command += " --auto=false"
		}
		command += " \"$KFORGE_CONTEXT\""
		return []string{command}
	}
	command := "kforge build --progress plain --load -t \"${KFORGE_IMAGE}:ci\""
	if spec.Auto {
		command += " --auto"
	}
	command += " \"$KFORGE_CONTEXT\""
	return []string{command}
}

func githubResolveImageCommands(spec CISpec) []string {
	commands := []string{
		"IMAGE=\"$KFORGE_IMAGE\"",
		"if [ -z \"$IMAGE\" ]; then",
		"  OWNER_LOWER=\"$(printf '%s' \"$GITHUB_REPOSITORY_OWNER\" | tr '[:upper:]' '[:lower:]')\"",
		"  IMAGE=\"ghcr.io/${OWNER_LOWER}/${KFORGE_IMAGE_NAME}\"",
		"fi",
		"echo \"KFORGE_IMAGE=$IMAGE\" >> \"$GITHUB_ENV\"",
		"echo \"Resolved image: $IMAGE\"",
	}
	if spec.Registry != "" {
		commands = append(commands, "echo \"KFORGE_REGISTRY="+spec.Registry+"\" >> \"$GITHUB_ENV\"")
	}
	return commands
}

func githubPublishCommands(spec CISpec) []string {
	command := "kforge build --progress plain"
	if spec.Auto {
		command += " --auto"
	}
	command += " --platform \"$KFORGE_PLATFORMS\" --push \"${TAGS[@]}\" \"$KFORGE_CONTEXT\""
	return []string{
		"TAGS=(-t \"${KFORGE_IMAGE}:${GITHUB_SHA}\")",
		"if [ \"${GITHUB_REF}\" = \"refs/heads/${KFORGE_MAIN_BRANCH}\" ]; then TAGS+=(-t \"${KFORGE_IMAGE}:latest\"); fi",
		"if [[ \"${GITHUB_REF}\" == refs/tags/* ]]; then TAGS+=(-t \"${KFORGE_IMAGE}:${GITHUB_REF#refs/tags/}\"); fi",
		command,
	}
}

func gitlabCheckCommands(spec CISpec) []string {
	if spec.Verify {
		command := "- kforge verify --progress plain"
		if !spec.Auto {
			command += " --auto=false"
		}
		command += " \"$KFORGE_CONTEXT\""
		return []string{command}
	}
	command := "- kforge build --progress plain --load -t \"${KFORGE_IMAGE}:ci\""
	if spec.Auto {
		command += " --auto"
	}
	command += " \"$KFORGE_CONTEXT\""
	return []string{command}
}

func gitlabPublishCommands(spec CISpec) []string {
	command := "PATH=\"$HOME/.local/bin:$PATH\" kforge build --progress plain"
	if spec.Auto {
		command += " --auto"
	}
	command += " --platform \"$KFORGE_PLATFORMS\" --push $TAGS \"$KFORGE_CONTEXT\""
	return []string{
		"TAGS=\"-t ${KFORGE_IMAGE}:${CI_COMMIT_SHA}\"",
		"if [ \"$CI_COMMIT_BRANCH\" = \"$KFORGE_MAIN_BRANCH\" ]; then TAGS=\"$TAGS -t ${KFORGE_IMAGE}:latest\"; fi",
		"if [ -n \"$CI_COMMIT_TAG\" ]; then TAGS=\"$TAGS -t ${KFORGE_IMAGE}:${CI_COMMIT_TAG}\"; fi",
		"sh -c '" + strings.ReplaceAll(command, "'", `'\''`) + "'",
	}
}

func ciDependencyJob(spec CISpec) string {
	if spec.Push {
		return "publish"
	}
	return "check"
}

func normalizeCIDeployTarget(target string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	switch target {
	case "", "none", "compose", "render", "fly":
		return target
	default:
		return ""
	}
}

func registryHost(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if strings.HasPrefix(image, "$CI_REGISTRY_IMAGE") {
		return "$CI_REGISTRY"
	}
	parts := strings.Split(image, "/")
	if len(parts) == 1 {
		return "docker.io"
	}
	host := parts[0]
	if strings.Contains(host, ".") || strings.Contains(host, ":") || host == "localhost" {
		return host
	}
	return "docker.io"
}

func writeBlock(b *strings.Builder, indent int, lines []string) {
	prefix := strings.Repeat(" ", indent)
	for _, line := range lines {
		b.WriteString(prefix + line + "\n")
	}
}
