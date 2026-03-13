package project

import (
	"fmt"
	"sort"
	"strings"
)

func GenerateDockerfile(d Detection) string {
	switch d.Framework {
	case FrameworkHTML:
		return htmlDockerfile()
	case FrameworkReact, FrameworkVue:
		return spaDockerfile(d)
	case FrameworkNext:
		return nextDockerfile(d)
	case FrameworkNest:
		return nestDockerfile(d)
	case FrameworkNode:
		return nodeDockerfile(d)
	case FrameworkSpring:
		return springDockerfile(d)
	case FrameworkFastAPI:
		return fastAPIDockerfile(d)
	case FrameworkFlask:
		return flaskDockerfile(d)
	case FrameworkDjango:
		return djangoDockerfile(d)
	default:
		return ""
	}
}

func GenerateDockerignore(d Detection) string {
	base := []string{
		".DS_Store",
		".git",
		".gitignore",
	}
	switch d.Framework {
	case FrameworkHTML:
		return strings.Join(base, "\n") + "\n"
	case FrameworkSpring:
		lines := append(base,
			"target",
			"build",
			".gradle",
			".idea",
			"*.iml",
		)
		return strings.Join(lines, "\n") + "\n"
	case FrameworkFastAPI, FrameworkFlask, FrameworkDjango:
		lines := append(base,
			"__pycache__",
			"*.pyc",
			".pytest_cache",
			".mypy_cache",
			".ruff_cache",
			".venv",
			"venv",
			"dist",
			"build",
			"staticfiles",
		)
		return strings.Join(lines, "\n") + "\n"
	default:
		lines := append(base,
			"node_modules",
			"npm-debug.log*",
			"yarn-error.log*",
			"pnpm-debug.log*",
			".next",
			"dist",
			"build",
			"coverage",
		)
		return strings.Join(lines, "\n") + "\n"
	}
}

func GenerateBakeFile(d Detection) string {
	return strings.TrimSpace(fmt.Sprintf(`variable "TAG" {
  default = "latest"
}

target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  tags       = ["%s:${TAG}"]
  platforms  = ["linux/amd64"]
}

group "default" {
  targets = ["app"]
}
`, d.SuggestedImageName())) + "\n"
}

func htmlDockerfile() string {
	return strings.TrimSpace(`FROM nginx:1.27-alpine AS release

COPY . /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
`) + "\n"
}

func spaDockerfile(d Detection) string {
	return strings.TrimSpace(fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* .npmrc* .yarnrc* ./
RUN %s

COPY . .
RUN %s

FROM nginx:1.27-alpine AS release

COPY --from=build /app/%s /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
`, d.NodeVersion, packageManagerInstallCmd(d, false), packageManagerRunCmd(d.PackageManager, "build"), d.BuildOutput)) + "\n"
}

func nextDockerfile(d Detection) string {
	if d.NextStandalone {
		publicCopy := ""
		if d.HasPublicDir {
			publicCopy = "\nCOPY --from=build /app/public ./public"
		}

		return strings.TrimSpace(fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* .npmrc* .yarnrc* ./
RUN %s

COPY . .
RUN %s

FROM node:%s-alpine AS release

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

%s
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static

EXPOSE 3000

CMD ["node", "server.js"]
`, d.NodeVersion, packageManagerInstallCmd(d, false), packageManagerRunCmd(d.PackageManager, "build"), d.NodeVersion, publicCopy)) + "\n"
	}

	return strings.TrimSpace(fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* .npmrc* .yarnrc* ./
RUN %s

COPY . .
RUN %s

FROM node:%s-alpine AS release

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

COPY --from=build /app ./
RUN rm -rf node_modules && %s

EXPOSE 3000

CMD %s
`, d.NodeVersion, packageManagerInstallCmd(d, false), packageManagerRunCmd(d.PackageManager, "build"), d.NodeVersion, packageManagerInstallCmd(d, true), jsonArray(d.StartCommand))) + "\n"
}

func nestDockerfile(d Detection) string {
	return strings.TrimSpace(fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* .npmrc* .yarnrc* ./
RUN %s

COPY . .
RUN %s

FROM node:%s-alpine AS release

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

COPY --from=build /app ./
RUN rm -rf node_modules && %s

EXPOSE 3000

CMD ["node", "dist/main.js"]
`, d.NodeVersion, packageManagerInstallCmd(d, false), packageManagerRunCmd(d.PackageManager, "build"), d.NodeVersion, packageManagerInstallCmd(d, true))) + "\n"
}

func nodeDockerfile(d Detection) string {
	buildRun := `echo "No build script detected"`
	if d.HasBuildScript {
		buildRun = packageManagerRunCmd(d.PackageManager, "build")
	}
	return strings.TrimSpace(fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* .npmrc* .yarnrc* ./
RUN %s

COPY . .
RUN %s

FROM node:%s-alpine AS release

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

COPY --from=build /app ./
RUN rm -rf node_modules && %s

EXPOSE 3000

CMD %s
`, d.NodeVersion, packageManagerInstallCmd(d, false), buildRun, d.NodeVersion, packageManagerInstallCmd(d, true), jsonArray(d.StartCommand))) + "\n"
}

func springDockerfile(d Detection) string {
	buildImage := fmt.Sprintf("maven:3.9.9-eclipse-temurin-%s", d.JavaVersion)
	buildCmd := "./mvnw -q -DskipTests package || mvn -q -DskipTests package"
	artifactCmd := `jar="$(find target -maxdepth 1 -name '*.jar' ! -name '*-sources.jar' ! -name '*-javadoc.jar' | head -n 1)" && test -n "$jar" && cp "$jar" app.jar`
	if d.BuildTool == BuildToolGradle {
		buildImage = fmt.Sprintf("gradle:8.10.0-jdk%s", d.JavaVersion)
		buildCmd = "if [ -x ./gradlew ]; then ./gradlew --no-daemon build -x test; else gradle --no-daemon build -x test; fi"
		artifactCmd = `jar="$(find build/libs -maxdepth 1 -name '*.jar' ! -name '*-plain.jar' | head -n 1)" && test -n "$jar" && cp "$jar" app.jar`
	}

	healthSetup := ""
	if d.HealthcheckPath != "" {
		healthSetup = strings.TrimSpace(fmt.Sprintf(`
ENV HEALTHCHECK_PATH=%s
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 CMD sh -c 'curl -fsS http://127.0.0.1:${SERVER_PORT}${HEALTHCHECK_PATH:-%s} >/dev/null || exit 1'
`, d.HealthcheckPath, d.HealthcheckPath))
	}

	return strings.TrimSpace(fmt.Sprintf(`FROM %s AS build

WORKDIR /workspace

COPY . .
RUN %s
RUN %s

FROM eclipse-temurin:%s-jre-jammy AS release

WORKDIR /app
ENV SERVER_PORT=8080
ENV SPRING_PROFILES_ACTIVE=prod
ENV APP_ENV=production
ENV JAVA_OPTS=""
%s
%s
COPY --from=build /workspace/app.jar ./app.jar

EXPOSE 8080

ENTRYPOINT ["sh", "-c", "java ${JAVA_OPTS} -Dserver.port=${SERVER_PORT:-8080} -Dspring.profiles.active=${SPRING_PROFILES_ACTIVE:-prod} -jar app.jar"]
`, buildImage, buildCmd, artifactCmd, d.JavaVersion, healthSetup, envBlock(d.EnvDefaults))) + "\n"
}

func fastAPIDockerfile(d Detection) string {
	installCmd := "pip install --no-cache-dir ."
	if d.HasRequirements {
		installCmd = "pip install --no-cache-dir -r requirements.txt"
	}

	healthSetup := ""
	if d.HealthcheckPath != "" {
		healthSetup = strings.TrimSpace(fmt.Sprintf(`
ENV HEALTHCHECK_PATH=%s
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD python -c "import os, urllib.request; port=os.environ.get('UVICORN_PORT', os.environ.get('PORT', '8000')); path=os.environ.get('HEALTHCHECK_PATH', '%s'); urllib.request.urlopen(f'http://127.0.0.1:{port}{path}', timeout=2).read()" || exit 1
`, d.HealthcheckPath, d.HealthcheckPath))
	}

	return strings.TrimSpace(fmt.Sprintf(`FROM python:%s-slim AS build

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv

RUN python -m venv ${VENV_PATH}
ENV PATH="${VENV_PATH}/bin:${PATH}"

COPY . .
RUN pip install --upgrade pip
RUN %s

FROM python:%s-slim AS release

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv
ENV PATH="${VENV_PATH}/bin:${PATH}"
ENV APP_ENV=production
ENV UVICORN_HOST=0.0.0.0
ENV UVICORN_PORT=8000
ENV PORT=8000
ENV UVICORN_WORKERS=1
ENV APP_MODULE=%s

COPY --from=build ${VENV_PATH} ${VENV_PATH}
COPY . .
%s
%s

EXPOSE 8000

CMD ["sh", "-c", "uvicorn ${APP_MODULE} --host ${UVICORN_HOST:-0.0.0.0} --port ${UVICORN_PORT:-8000} --workers ${UVICORN_WORKERS:-1}"]
`, d.PythonVersion, installCmd, d.PythonVersion, d.AppModule, healthSetup, envBlock(d.EnvDefaults))) + "\n"
}

func flaskDockerfile(d Detection) string {
	installCmd := "pip install --no-cache-dir . gunicorn"
	if d.HasRequirements {
		installCmd = "pip install --no-cache-dir -r requirements.txt gunicorn"
	}

	healthSetup := ""
	if d.HealthcheckPath != "" {
		healthSetup = strings.TrimSpace(fmt.Sprintf(`
ENV HEALTHCHECK_PATH=%s
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD python -c "import os, urllib.request; port=os.environ.get('PORT', '8000'); path=os.environ.get('HEALTHCHECK_PATH', '%s'); urllib.request.urlopen(f'http://127.0.0.1:{port}{path}', timeout=2).read()" || exit 1
`, d.HealthcheckPath, d.HealthcheckPath))
	}

	return strings.TrimSpace(fmt.Sprintf(`FROM python:%s-slim AS build

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv

RUN python -m venv ${VENV_PATH}
ENV PATH="${VENV_PATH}/bin:${PATH}"

COPY . .
RUN pip install --upgrade pip
RUN %s

FROM python:%s-slim AS release

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv
ENV PATH="${VENV_PATH}/bin:${PATH}"
ENV APP_ENV=production
ENV PORT=8000
ENV GUNICORN_WORKERS=2
ENV APP_MODULE=%s

COPY --from=build ${VENV_PATH} ${VENV_PATH}
COPY . .
%s
%s

EXPOSE 8000

CMD ["sh", "-c", "gunicorn --bind 0.0.0.0:${PORT:-8000} --workers ${GUNICORN_WORKERS:-2} ${APP_MODULE}"]
`, d.PythonVersion, installCmd, d.PythonVersion, d.AppModule, healthSetup, envBlock(d.EnvDefaults))) + "\n"
}

func djangoDockerfile(d Detection) string {
	installCmd := "pip install --no-cache-dir . gunicorn"
	if d.HasRequirements {
		installCmd = "pip install --no-cache-dir -r requirements.txt gunicorn"
	}

	collectstaticCmd := `python manage.py collectstatic --noinput || true`
	healthSetup := ""
	if d.HealthcheckPath != "" {
		healthSetup = strings.TrimSpace(fmt.Sprintf(`
ENV HEALTHCHECK_PATH=%s
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 CMD python -c "import os, urllib.request; port=os.environ.get('PORT', '8000'); path=os.environ.get('HEALTHCHECK_PATH', '%s'); urllib.request.urlopen(f'http://127.0.0.1:{port}{path}', timeout=2).read()" || exit 1
`, d.HealthcheckPath, d.HealthcheckPath))
	}

	return strings.TrimSpace(fmt.Sprintf(`FROM python:%s-slim AS build

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv

RUN python -m venv ${VENV_PATH}
ENV PATH="${VENV_PATH}/bin:${PATH}"

COPY . .
RUN pip install --upgrade pip
RUN %s
RUN %s

FROM python:%s-slim AS release

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV VENV_PATH=/opt/venv
ENV PATH="${VENV_PATH}/bin:${PATH}"
ENV APP_ENV=production
ENV PORT=8000
ENV GUNICORN_WORKERS=2
ENV DJANGO_SETTINGS_MODULE=%s
ENV APP_MODULE=%s

COPY --from=build ${VENV_PATH} ${VENV_PATH}
COPY . .
%s
%s

EXPOSE 8000

CMD ["sh", "-c", "gunicorn --bind 0.0.0.0:${PORT:-8000} --workers ${GUNICORN_WORKERS:-2} ${APP_MODULE}"]
`, d.PythonVersion, installCmd, collectstaticCmd, d.PythonVersion, d.SettingsModule, d.AppModule, healthSetup, envBlock(d.EnvDefaults))) + "\n"
}

func packageManagerInstallCmd(d Detection, production bool) string {
	switch d.PackageManager {
	case PackageManagerPNPM:
		if production {
			return "corepack enable && pnpm install --prod --frozen-lockfile"
		}
		return "corepack enable && pnpm install --frozen-lockfile"
	case PackageManagerYarn:
		if production {
			return "corepack enable && yarn install --production=true --frozen-lockfile"
		}
		return "corepack enable && yarn install --frozen-lockfile"
	default:
		if d.HasLockfile {
			if production {
				return "npm ci --omit=dev"
			}
			return "npm ci"
		}
		if production {
			return "npm install --omit=dev"
		}
		return "npm install"
	}
}

func packageManagerRunCmd(pm PackageManager, script string) string {
	switch pm {
	case PackageManagerPNPM:
		return "pnpm " + script
	case PackageManagerYarn:
		return "yarn " + script
	default:
		return "npm run " + script
	}
}

func jsonArray(parts []string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, `\`, `\\`)
		part = strings.ReplaceAll(part, `"`, `\"`)
		quoted = append(quoted, fmt.Sprintf("%q", part))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func envBlock(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("ENV %s=%s", key, values[key]))
	}
	return strings.Join(lines, "\n")
}
