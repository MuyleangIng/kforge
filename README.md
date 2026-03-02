# buildforge

A personal multi-platform Docker image build CLI powered by BuildKit.

## Features

- **Multi-platform builds** — build for `linux/amd64`, `linux/arm64`, and more simultaneously
- **Declarative bake config** — define targets in `buildforge.hcl` or `buildforge.json`
- **Flexible caching** — registry, local, S3, Azure, GitHub Actions cache backends
- **Secrets** — inject secrets without baking them into layers
- **Registry auth** — reads your `~/.docker/config.json` automatically

## Install

```bash
git clone https://github.com/MuyleangIng/buildforge
cd buildforge
go build -o buildforge ./cmd/
mv buildforge /usr/local/bin/
```

## Usage

### Build

```bash
# Build and load into local Docker
buildforge build -t myapp:latest .

# Multi-platform push
buildforge build --platform linux/amd64,linux/arm64 --push -t myrepo/myapp:latest .

# With registry cache
buildforge build \
  --cache-from type=registry,ref=myrepo/myapp:cache \
  --cache-to   type=registry,ref=myrepo/myapp:cache,mode=max \
  --push -t myrepo/myapp:latest .

# With build args and target stage
buildforge build --build-arg VERSION=1.2.3 --target release -t myapp:1.2.3 .
```

### Bake (declarative builds)

Create a `buildforge.hcl` file:

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

Then run:

```bash
buildforge bake              # builds "default" group
buildforge bake app          # builds specific target
buildforge bake --set app.platforms=linux/arm64  # override field
TAG=1.0.0 buildforge bake   # pass variable via env
```

### Builder management

```bash
buildforge builder create --name mybuilder         # create a builder
buildforge builder ls                              # list builders
buildforge builder use mybuilder                   # set active builder
buildforge builder rm mybuilder                    # remove builder
```

## Cache backends

| Type | Example |
|---|---|
| Registry | `type=registry,ref=myrepo/app:cache` |
| Local | `type=local,dest=/tmp/cache` |
| GitHub Actions | `type=gha` |
| S3 | `type=s3,bucket=mybucket,region=us-east-1` |
| Azure Blob | `type=azblob,account=myaccount,name=mycache` |

## Secrets

```bash
buildforge build --secret id=mysecret,src=/path/to/secret .
```

In your Dockerfile:
```dockerfile
RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret
```
