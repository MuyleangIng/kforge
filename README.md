# kforge

> Personal multi-platform Docker image builder powered by BuildKit

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://go.dev)

---

## About

**kforge** is a personal Docker image build CLI inspired by Docker Buildx, built on top of BuildKit.
It works both as a **standalone binary** and as a **Docker CLI plugin** (`docker kforge`).

**Made by:** Ing Muyleang
**Founder:** [KhmerStack](https://github.com/KhmerStack)

---

## Features

- **Multi-platform builds** — `linux/amd64`, `linux/arm64`, and more simultaneously
- **Docker plugin mode** — works as `docker kforge build ...`
- **5 progress styles** — spinner, bar, banner, dots, plain (pick at runtime)
- **Declarative bake config** — define targets in `kforge.hcl` or `kforge.json`
- **Flexible caching** — registry, local, S3, Azure, GitHub Actions
- **Secrets** — inject files without baking them into layers
- **Registry auth** — reads your `~/.docker/config.json` automatically
- **Builder management** — create, list, switch, and remove builders

---

## Install

```bash
git clone https://github.com/MuyleangIng/kforge
cd kforge
go build -o kforge ./cmd/
sudo mv kforge /usr/local/bin/
```

### Install as Docker CLI plugin

```bash
mkdir -p ~/.docker/cli-plugins
cp $(which kforge) ~/.docker/cli-plugins/docker-kforge
```

Now use both:

```bash
kforge build ...           # standalone
docker kforge build ...    # via Docker CLI
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

## Usage

### Build

```bash
# Build and load into local Docker
kforge build -t myapp:latest .

# Docker plugin mode
docker kforge build -t myapp:latest .

# Multi-platform push
kforge build --platform linux/amd64,linux/arm64 --push -t myrepo/myapp:latest .

# Registry cache
kforge build \
  --cache-from type=registry,ref=myrepo/myapp:cache \
  --cache-to   type=registry,ref=myrepo/myapp:cache,mode=max \
  --push -t myrepo/myapp:latest .

# Build args + target stage
kforge build --build-arg VERSION=1.2.3 --target release -t myapp:1.2.3 .

# Secrets
kforge build --secret id=mysecret,src=./token.txt -t myapp .
```

### Bake (declarative builds)

Create a `kforge.hcl` file:

```hcl
variable "TAG" { default = "latest" }

target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = ["myrepo/app:${TAG}"]
  cache-from = ["type=registry,ref=myrepo/app:cache"]
  cache-to   = ["type=registry,ref=myrepo/app:cache,mode=max"]
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
TAG=1.0.0 kforge bake                    # pass variable via env
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

---

## Cache Backends

| Type | Example |
|---|---|
| Registry | `type=registry,ref=myrepo/app:cache` |
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
│   └── version.go           # kforge version
├── builder/builder.go       # builder config store (~/.kforge/)
├── bake/bake.go             # HCL + JSON config file parser
└── util/progress/
    └── progress.go          # 5 styled progress renderers
```

---

## License

MIT License

Copyright (c) 2024 Ing Muyleang / KhmerStack

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

<p align="center">
  Made with ❤️ by <strong>Ing Muyleang</strong> · Founder of <a href="https://github.com/KhmerStack">KhmerStack</a>
</p>
