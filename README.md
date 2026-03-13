# kforge

> Personal multi-platform Docker image builder powered by BuildKit

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://go.dev)
[![Release](https://img.shields.io/badge/release-v1.1.1-blue.svg)](https://github.com/MuyleangIng/kforge/releases/tag/v1.1.1)

---

## About

**kforge** is a personal Docker image build CLI inspired by Docker Buildx, built on top of BuildKit.
It works both as a **standalone binary** and as a **Docker CLI plugin** (`docker kforge`).

**Made by:** Ing Muyleang
**Founder:** [KhmerStack](https://github.com/KhmerStack)

---

## Features

- **Multi-platform builds** — `linux/amd64`, `linux/arm64`, and more simultaneously
- **Docker plugin mode** — works as `docker kforge build ...` (same as `docker buildx`)
- **Interactive setup wizard** — `kforge setup` guides QEMU or multi-node configuration
- **5 progress styles** — spinner, bar, banner, dots, plain (pick at runtime)
- **Declarative bake config** — define targets in `kforge.hcl` or `kforge.json`
- **Project detection + Dockerfile generation** — detect Next.js, React, Vue, NestJS, Node, Spring Boot, FastAPI, Flask, Django, or plain HTML and generate a suitable Dockerfile
- **Project overrides** — `.kforge.yml` can pin framework, runtime, image name, healthcheck, env defaults, and verify behavior
- **Starter project generator** — `kforge init` can create a demo app or generate Docker assets from an existing project
- **Build + run + check** — `kforge verify` builds locally, runs the container, waits for readiness, and checks HTTP endpoints
- **CI/CD bootstrap** — `kforge ci init` generates GitHub Actions and GitLab CI pipelines for build, verify, push, and optional deploy stages
- **Deploy bootstrap** — `kforge deploy init` generates `docker-compose.yml`, `render.yaml`, and `fly.toml` from detected project settings
- **Environment diagnostics** — `kforge doctor` checks Docker, Buildx, contexts, and builders
- **Flexible caching** — registry, local, S3, Azure, GitHub Actions
- **Secrets** — inject files without baking them into layers
- **Registry auth** — reads your `~/.docker/config.json` automatically
- **Builder management** — create, list, switch, and remove builders

---

## Install

Download pre-built packages from the [v1.1.1 release page](https://github.com/MuyleangIng/kforge/releases/tag/v1.1.1).
Verify downloads with [checksums.txt](https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/checksums.txt).

### macOS (Apple Silicon / arm64)

**tar.gz (terminal):**
```bash
curl -Lo kforge.tar.gz https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_darwin_arm64.tar.gz
tar -xzf kforge.tar.gz
sudo mv kforge /usr/local/bin/

# Verify
kforge version
```

**DMG installer (GUI):**
```bash
curl -Lo kforge.dmg https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_v1.1.1_darwin_arm64.dmg
open kforge.dmg
```

### macOS (Intel / amd64)

**tar.gz (terminal):**
```bash
curl -Lo kforge.tar.gz https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_darwin_amd64.tar.gz
tar -xzf kforge.tar.gz
sudo mv kforge /usr/local/bin/

# Verify
kforge version
```

**DMG installer (GUI):**
```bash
curl -Lo kforge.dmg https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_v1.1.1_darwin_amd64.dmg
open kforge.dmg
```

### Linux (amd64)

**Debian / Ubuntu (.deb):**
```bash
curl -Lo kforge.deb https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_amd64.deb
sudo dpkg -i kforge.deb

# Verify
kforge version
```

**RHEL / Fedora / CentOS (.rpm):**
```bash
curl -Lo kforge.rpm https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_amd64.rpm
sudo rpm -i kforge.rpm

# Verify
kforge version
```

**tar.gz (any distro):**
```bash
curl -Lo kforge.tar.gz https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_amd64.tar.gz
tar -xzf kforge.tar.gz
sudo mv kforge /usr/local/bin/

# Verify
kforge version
```

### Linux (arm64)

**Debian / Ubuntu (.deb):**
```bash
curl -Lo kforge.deb https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_arm64.deb
sudo dpkg -i kforge.deb

# Verify
kforge version
```

**RHEL / Fedora / CentOS (.rpm):**
```bash
curl -Lo kforge.rpm https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_arm64.rpm
sudo rpm -i kforge.rpm

# Verify
kforge version
```

**tar.gz (any distro):**
```bash
curl -Lo kforge.tar.gz https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_linux_arm64.tar.gz
tar -xzf kforge.tar.gz
sudo mv kforge /usr/local/bin/

# Verify
kforge version
```

### Windows (amd64)

```powershell
# Download zip
curl -Lo kforge.zip https://github.com/MuyleangIng/kforge/releases/download/v1.1.1/kforge_1.1.1_windows_amd64.zip

# Extract
Expand-Archive kforge.zip -DestinationPath C:\tools\kforge

# Add C:\tools\kforge to your PATH, then verify
kforge version
```

### Homebrew (coming soon)

```bash
# brew install kforge
```

### Build from source

```bash
git clone https://github.com/MuyleangIng/kforge
cd kforge
go build -o kforge ./cmd/
sudo mv kforge /usr/local/bin/
```

### Install as Docker CLI plugin

```bash
mkdir -p ~/.docker/cli-plugins
go build -o ~/.docker/cli-plugins/docker-kforge ./cmd/
chmod +x ~/.docker/cli-plugins/docker-kforge
```

Now use both:

```bash
kforge build ...           # standalone
docker kforge build ...    # via Docker CLI (just like docker buildx)
```

---

## Progress Styles

Use `--progress <style>` during any build:

| Style | What you see |
|---|---|
| `auto` | Spinner if TTY, plain otherwise **(default)** |
| `spinner` | Animated spinner + colored step names + timing |
| `bar` | ASCII progress bar per Dockerfile stage |
| `banner` | Big ASCII banner header + streaming logs |
| `dots` | Minimal pulsing dot + step name |
| `plain` | Raw log output, no colors |

```bash
kforge build --progress spinner -t myapp .
kforge build --progress bar     -t myapp .
kforge build --progress banner  -t myapp .
kforge build --progress dots    -t myapp .
kforge build --progress plain   -t myapp .
```

---

## Setup (Multi-platform wizard)

Run the interactive setup wizard to configure your builder:

```bash
kforge setup
# or via Docker plugin:
docker kforge setup
```

The wizard guides you through:

```
  ██╗  ██╗███████╗ ██████╗ ██████╗  ██████╗ ███████╗
  ██║ ██╔╝██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
  █████╔╝ █████╗  ██║   ██║██████╔╝██║  ███╗█████╗
  ██╔═██╗ ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
  ██║  ██╗██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
  ╚═╝  ╚═╝╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝

Choose your build strategy:
  1) QEMU emulation      Build all platforms on one machine (easiest)
  2) Multiple native nodes  Use separate machines per platform (fastest)
  3) Both (recommended)  Native nodes first, QEMU as fallback
  q) Quit
```

**Option 1 — QEMU (one machine):**
Installs QEMU via `docker run --privileged --rm tonistiigi/binfmt --install all`
then creates a BuildKit builder. Best for most people.

**Option 2 — Multiple native nodes:**
Prompts you for Docker context names per platform, then runs:
```bash
docker buildx create --use --name mybuild node-amd64
docker buildx create --append --name mybuild node-arm64
```

---

## Usage

### Detect a project and generate Docker assets

If a project does not have a `Dockerfile` yet, `kforge` can detect the app type and generate one for you.

```bash
# Inspect the current project
kforge detect

# Detect another directory
kforge detect ./apps/web

# Show the generated Dockerfile without writing files
kforge detect --print-dockerfile

# Generate Dockerfile + .dockerignore + kforge.hcl from an existing app
kforge init --detect

# Force a framework or Node version if detection needs help
kforge init --detect --framework next --node 20
kforge init --detect --framework fastapi --python 3.12
kforge init --detect --framework spring --java 21
kforge init --detect --framework flask --python 3.12
kforge init --detect --framework django --python 3.12
```

Detection currently supports:

- Next.js
- React / Vite-style SPA
- Vue
- NestJS
- Generic Node.js apps
- Spring Boot
- FastAPI
- Flask
- Django
- Plain static HTML

Runtime version selection is resolved from:

- Node: `package.json` `engines.node`, then `.nvmrc`, then `.node-version`
- Python: `pyproject.toml` `requires-python`, then `.python-version`, then `runtime.txt`
- Java: Maven/Gradle project settings when present

You can override the detected runtime with `--node`, `--python`, or `--java`.

### Project overrides with `.kforge.yml`

You can pin or adjust detection results in a `.kforge.yml` file at the project root.

```yaml
image: api-demo
framework: flask
python: "3.12"
port: 8000
healthcheck: /health
app_module: app:app
env:
  APP_ENV: staging
verify:
  path: /health
  timeout_seconds: 20
deploy:
  compose:
    service: web
  render:
    name: api-demo
    plan: starter
    region: oregon
  fly:
    app: api-demo
    primary_region: iad
    memory_mb: 512
ci:
  main_branch: main
  platforms:
    - linux/amd64
  deploy: render
  deploy_path: /srv/api-demo
  github:
    workflow: kforge-ci.yml
```

Useful fields:

- `framework`
- `image`
- `node`, `python`, `java`
- `port`
- `healthcheck`
- `app_module`
- `start_command`
- `env`
- `verify.path`, `verify.port`, `verify.timeout_seconds`, `verify.env`
- `deploy.port`, `deploy.healthcheck`, `deploy.command`, `deploy.env`
- `deploy.compose.service`
- `deploy.render.name`, `deploy.render.plan`, `deploy.render.region`
- `deploy.fly.app`, `deploy.fly.primary_region`, `deploy.fly.memory_mb`
- `ci.image`, `ci.main_branch`, `ci.context`, `ci.platforms`
- `ci.auto`, `ci.verify`, `ci.push`
- `ci.deploy`, `ci.deploy_path`
- `ci.github.workflow`, `ci.gitlab.file`

### Build

```bash
# Build and load into local Docker
kforge build -t muyleangin/myapp:latest .

# Docker plugin mode (same as docker buildx!)
docker kforge build -t muyleangin/myapp:latest .

# Multi-platform push
kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/myapp:latest .

# Registry cache
kforge build \
  --cache-from type=registry,ref=muyleangin/myapp:cache \
  --cache-to   type=registry,ref=muyleangin/myapp:cache,mode=max \
  --push -t muyleangin/myapp:latest .

# Build args + target stage
kforge build --build-arg VERSION=1.2.3 --target release -t myapp:1.2.3 .

# Build without a Dockerfile by auto-generating one in a temp file
kforge build --auto -t myapp:dev .

# Same, but force a project type or Node version
kforge build --auto --framework next --node 20 -t myapp:dev .

# Backend auto-detect
kforge build --auto --framework fastapi --python 3.12 -t api:dev .
kforge build --auto --framework spring --java 21 -t api:dev .
kforge build --auto --framework flask --python 3.12 -t api:dev .
kforge build --auto --framework django --python 3.12 -t api:dev .

# Secrets
kforge build --secret id=mysecret,src=./token.txt -t myapp .
```

If no `Dockerfile` exists and `--auto` is not set, `kforge build` now stops with a helpful message telling you to run `kforge init --detect` or `kforge build --auto`.

### Bake (declarative builds)

Create a `kforge.hcl` file:

```hcl
variable "TAG" { default = "latest" }

target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["muyleangin/app:${TAG}"]
  cache-from = ["type=registry,ref=muyleangin/app:cache"]
  cache-to   = ["type=registry,ref=muyleangin/app:cache,mode=max"]
  push       = true
}

group "default" {
  targets = ["app"]
}
```

```bash
kforge bake                              # builds "default" group
kforge bake app                          # builds specific target
kforge bake --set app.platforms=linux/arm64
TAG=1.1.1 kforge bake                    # pass variable via env
kforge bake -f ci/kforge.hcl            # custom file
```

### Builder Management

```bash
kforge builder create --name mybuilder
kforge builder create --name remote --driver remote --endpoint tcp://buildkitd:1234
kforge builder ls
kforge builder use mybuilder
kforge builder rm mybuilder
```

### Doctor

```bash
kforge doctor
```

### Verify

```bash
kforge verify
kforge verify ./examples/fastapi-auto
kforge verify --path /health --env APP_ENV=staging .
kforge verify --keep-running ./examples/django-auto
```

`kforge verify` will:

- build the image locally
- run it on a random localhost port
- wait for the detected or configured health path
- print the HTTP response summary
- stop the container unless `--keep-running` is set

### CI/CD

```bash
kforge ci init
kforge ci init --target github
kforge ci init --target gitlab --deploy compose
kforge ci init --print .
```

`kforge ci init` will:

- detect the project and load `.kforge.yml` overrides
- generate `.github/workflows/kforge-ci.yml` and/or `.gitlab-ci.yml`
- create missing Docker assets first when the project does not have a `Dockerfile`
- optionally add a deploy stage for `compose`, `render`, or `fly`
- generate the matching deploy file if the selected CI deploy target needs one

The generated pipelines are based on the current `kforge` commands:

- `kforge detect`
- `kforge verify`
- `kforge build --push`
- deploy hooks for `docker compose`, Render, or Fly.io

### Deploy

```bash
kforge deploy init
kforge deploy init ./examples/flask-auto
kforge deploy init --target compose
kforge deploy init --target render,fly ./examples/django-auto
kforge deploy init --print .
```

`kforge deploy init` will:

- detect the project and load `.kforge.yml` overrides
- generate missing Docker assets when `Dockerfile` or `kforge.hcl` do not exist yet
- write `docker-compose.yml`, `render.yaml`, and `fly.toml`
- let you limit output with `--target compose`, `--target render`, or `--target fly`
- print the files instead of writing them when `--print` is set

### Init

```bash
kforge init --name myapp
kforge init --dir ./demo --force
kforge init --detect
kforge init --detect --framework react --node 22
kforge init --detect --framework fastapi --python 3.12
kforge init --detect --framework spring --java 21
kforge init --detect --framework flask --python 3.12
kforge init --detect --framework django --python 3.12
kforge init --detect --print-dockerfile
```

---

## Cache Backends

| Type | Example |
|---|---|
| Registry | `type=registry,ref=muyleangin/app:cache` |
| Local | `type=local,dest=/tmp/cache` |
| GitHub Actions | `type=gha` |
| S3 | `type=s3,bucket=mybucket,region=us-east-1` |
| Azure Blob | `type=azblob,account=myaccount,name=mycache` |

---

## Secrets

```bash
kforge build --secret id=mysecret,src=/path/to/secret .
```

In your Dockerfile:

```dockerfile
RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret
```

---

## Project Structure

```
kforge/
├── cmd/main.go              # entry point (standalone + Docker plugin)
├── commands/
│   ├── build.go             # kforge build
│   ├── bake.go              # kforge bake
│   ├── builder.go           # kforge builder create/ls/use/rm
│   ├── ci.go                # kforge ci init
│   ├── detect.go            # kforge detect
│   ├── deploy.go            # kforge deploy init
│   ├── doctor.go            # kforge doctor
│   ├── init.go              # kforge init
│   ├── verify.go            # kforge verify
│   └── version.go           # kforge version
├── internal/project/        # project detection + generated Docker templates
├── internal/meta/meta.go    # shared version/tool metadata
├── builder/builder.go       # builder config store (~/.kforge/)
├── bake/bake.go             # HCL + JSON config file parser
└── util/progress/
    └── progress.go          # 5 styled progress renderers
```

---

<p align="center">
  Made with ❤️ by <strong>Ing Muyleang</strong> · Founder of <a href="https://github.com/KhmerStack">KhmerStack</a>
</p>
