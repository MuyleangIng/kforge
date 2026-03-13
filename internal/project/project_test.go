package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectNextProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "@acme/web",
  "scripts": {
    "build": "next build",
    "start": "next start"
  },
  "dependencies": {
    "next": "14.2.0",
    "react": "18.3.0",
    "react-dom": "18.3.0"
  },
  "engines": {
    "node": "20.x"
  }
}`)
	writeFile(t, filepath.Join(dir, "next.config.js"), `module.exports = { output: "standalone" }`+"\n")
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")

	d, err := Detect(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkNext {
		t.Fatalf("expected next framework, got %q", d.Framework)
	}
	if d.PackageManager != PackageManagerPNPM {
		t.Fatalf("expected pnpm package manager, got %q", d.PackageManager)
	}
	if d.NodeVersion != "20" {
		t.Fatalf("expected node 20, got %q", d.NodeVersion)
	}
	if !d.NextStandalone {
		t.Fatal("expected standalone Next.js detection to be true")
	}
	if d.BuildOutput != ".next/standalone" {
		t.Fatalf("expected standalone build output, got %q", d.BuildOutput)
	}
	if got := strings.Join(d.StartCommand, " "); got != "node server.js" {
		t.Fatalf("unexpected start command: %q", got)
	}
}

func TestDetectHTMLProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>Demo</title>")

	d, err := Detect(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkHTML {
		t.Fatalf("expected html framework, got %q", d.Framework)
	}
	if d.Port != 80 {
		t.Fatalf("expected port 80, got %d", d.Port)
	}
}

func TestDetectFastAPIProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "fastapi==0.115.0\nuvicorn[standard]==0.34.0\n")
	writeFile(t, filepath.Join(dir, ".python-version"), "3.12.2\n")
	writeFile(t, filepath.Join(dir, "main.py"), `from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def root():
    return {"ok": True}

@app.get("/health")
def health():
    return {"status": "ok"}
`)

	d, err := DetectWithOverrides(dir, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkFastAPI {
		t.Fatalf("expected fastapi framework, got %q", d.Framework)
	}
	if d.BuildTool != BuildToolPip {
		t.Fatalf("expected pip build tool, got %q", d.BuildTool)
	}
	if d.PythonVersion != "3.12" {
		t.Fatalf("expected python 3.12, got %q", d.PythonVersion)
	}
	if d.AppModule != "main:app" {
		t.Fatalf("expected app module main:app, got %q", d.AppModule)
	}
	if d.HealthcheckPath != "/health" {
		t.Fatalf("expected /health healthcheck, got %q", d.HealthcheckPath)
	}
}

func TestDetectSpringProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pom.xml"), `<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>spring-demo</artifactId>
  <version>0.0.1-SNAPSHOT</version>
  <properties>
    <java.version>21</java.version>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-actuator</artifactId>
    </dependency>
  </dependencies>
</project>
`)
	writeFile(t, filepath.Join(dir, "src", "main", "java", "com", "example", "DemoApplication.java"), `package com.example;

import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class DemoApplication {}
`)

	d, err := DetectWithOverrides(dir, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkSpring {
		t.Fatalf("expected spring framework, got %q", d.Framework)
	}
	if d.BuildTool != BuildToolMaven {
		t.Fatalf("expected maven build tool, got %q", d.BuildTool)
	}
	if d.JavaVersion != "21" {
		t.Fatalf("expected java 21, got %q", d.JavaVersion)
	}
	if d.HealthcheckPath != "/actuator/health" {
		t.Fatalf("expected actuator healthcheck, got %q", d.HealthcheckPath)
	}
	if got := strings.Join(d.StartCommand, " "); got != "java -jar app.jar" {
		t.Fatalf("unexpected start command: %q", got)
	}
}

func TestDetectFlaskProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "flask==3.0.3\n")
	writeFile(t, filepath.Join(dir, "app.py"), `from flask import Flask, jsonify

app = Flask(__name__)

@app.get("/")
def root():
    return jsonify(ok=True)

@app.get("/health")
def health():
    return jsonify(status="ok")
`)

	d, err := DetectWithOverrides(dir, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkFlask {
		t.Fatalf("expected flask framework, got %q", d.Framework)
	}
	if d.AppModule != "app:app" {
		t.Fatalf("expected flask app module app:app, got %q", d.AppModule)
	}
	if d.HealthcheckPath != "/health" {
		t.Fatalf("expected flask healthcheck /health, got %q", d.HealthcheckPath)
	}
	if got := strings.Join(d.StartCommand, " "); got != "gunicorn --bind 0.0.0.0:8000 app:app" {
		t.Fatalf("unexpected flask start command: %q", got)
	}
}

func TestDetectDjangoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "django==5.1.0\n")
	writeFile(t, filepath.Join(dir, "manage.py"), `#!/usr/bin/env python
import os

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "config.settings")
`)
	writeFile(t, filepath.Join(dir, "config", "settings.py"), "SECRET_KEY='test'\n")
	writeFile(t, filepath.Join(dir, "config", "wsgi.py"), "application = None\n")
	writeFile(t, filepath.Join(dir, "config", "urls.py"), `urlpatterns = [
    ("health/", "health"),
]
`)

	d, err := DetectWithOverrides(dir, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Framework != FrameworkDjango {
		t.Fatalf("expected django framework, got %q", d.Framework)
	}
	if d.SettingsModule != "config.settings" {
		t.Fatalf("expected django settings module config.settings, got %q", d.SettingsModule)
	}
	if d.AppModule != "config.wsgi:application" {
		t.Fatalf("expected django app module config.wsgi:application, got %q", d.AppModule)
	}
	if d.HealthcheckPath != "/health/" {
		t.Fatalf("expected django healthcheck /health/, got %q", d.HealthcheckPath)
	}
}

func TestConfigOverridesDetection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "flask==3.0.3\n")
	writeFile(t, filepath.Join(dir, "app.py"), `from flask import Flask
app = Flask(__name__)
`)
	writeFile(t, filepath.Join(dir, ".kforge.yml"), `image: custom-api
framework: flask
port: 9000
healthcheck: /ready
app_module: app:app
start_command:
  - gunicorn
  - --bind
  - 0.0.0.0:9000
  - app:app
env:
  APP_ENV: qa
verify:
  path: /ready
  timeout_seconds: 12
`)

	d, err := DetectWithOverrides(dir, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "custom-api" {
		t.Fatalf("expected image name custom-api, got %q", d.Name)
	}
	if d.Port != 9000 {
		t.Fatalf("expected port 9000, got %d", d.Port)
	}
	if d.HealthcheckPath != "/ready" || d.VerifyPath != "/ready" {
		t.Fatalf("expected ready paths, got health=%q verify=%q", d.HealthcheckPath, d.VerifyPath)
	}
	if d.VerifyTimeout != 12 {
		t.Fatalf("expected verify timeout 12, got %d", d.VerifyTimeout)
	}
	if d.EnvDefaults["APP_ENV"] != "qa" {
		t.Fatalf("expected env override APP_ENV=qa, got %#v", d.EnvDefaults)
	}
	if got := strings.Join(d.StartCommand, " "); got != "gunicorn --bind 0.0.0.0:9000 app:app" {
		t.Fatalf("unexpected overridden start command: %q", got)
	}
}

func TestGenerateReactDockerfile(t *testing.T) {
	d := Detection{
		Name:           "demoapp",
		Framework:      FrameworkReact,
		PackageManager: PackageManagerNPM,
		HasLockfile:    true,
		NodeVersion:    "22",
		BuildOutput:    "dist",
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"FROM node:22-alpine AS build",
		"RUN npm ci",
		"RUN npm run build",
		"FROM nginx:1.27-alpine AS release",
		"COPY --from=build /app/dist /usr/share/nginx/html",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
}

func TestGenerateNextStandaloneDockerfile(t *testing.T) {
	d := Detection{
		Name:           "web",
		Framework:      FrameworkNext,
		PackageManager: PackageManagerNPM,
		NodeVersion:    "20",
		NextStandalone: true,
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"FROM node:20-alpine AS build",
		"COPY --from=build /app/.next/standalone ./",
		"COPY --from=build /app/.next/static ./.next/static",
		`CMD ["node", "server.js"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
	if strings.Contains(dockerfile, "npm install --omit=dev") {
		t.Fatalf("did not expect standalone Dockerfile to reinstall production dependencies\n%s", dockerfile)
	}
	if strings.Contains(dockerfile, "COPY --from=build /app/public ./public") {
		t.Fatalf("did not expect standalone Dockerfile to copy public/ when none exists\n%s", dockerfile)
	}
}

func TestGenerateFastAPIDockerfile(t *testing.T) {
	d := Detection{
		Framework:       FrameworkFastAPI,
		BuildTool:       BuildToolPip,
		PythonVersion:   "3.12",
		AppModule:       "main:app",
		HasRequirements: true,
		HealthcheckPath: "/health",
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"FROM python:3.12-slim AS build",
		"RUN pip install --no-cache-dir -r requirements.txt",
		"ENV APP_MODULE=main:app",
		"HEALTHCHECK --interval=30s",
		`CMD ["sh", "-c", "uvicorn ${APP_MODULE} --host ${UVICORN_HOST:-0.0.0.0} --port ${UVICORN_PORT:-8000} --workers ${UVICORN_WORKERS:-1}"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
}

func TestGenerateSpringDockerfile(t *testing.T) {
	d := Detection{
		Framework:       FrameworkSpring,
		BuildTool:       BuildToolMaven,
		JavaVersion:     "21",
		HealthcheckPath: "/actuator/health",
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"FROM maven:3.9.9-eclipse-temurin-21 AS build",
		"FROM eclipse-temurin:21-jre-jammy AS release",
		"ENV SPRING_PROFILES_ACTIVE=prod",
		"HEALTHCHECK --interval=30s",
		`ENTRYPOINT ["sh", "-c", "java ${JAVA_OPTS} -Dserver.port=${SERVER_PORT:-8080} -Dspring.profiles.active=${SPRING_PROFILES_ACTIVE:-prod} -jar app.jar"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
}

func TestGenerateFlaskDockerfile(t *testing.T) {
	d := Detection{
		Framework:       FrameworkFlask,
		BuildTool:       BuildToolPip,
		PythonVersion:   "3.12",
		AppModule:       "app:app",
		HasRequirements: true,
		HealthcheckPath: "/health",
		EnvDefaults: map[string]string{
			"APP_ENV": "qa",
		},
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"RUN pip install --no-cache-dir -r requirements.txt gunicorn",
		"ENV APP_MODULE=app:app",
		"ENV APP_ENV=qa",
		`CMD ["sh", "-c", "gunicorn --bind 0.0.0.0:${PORT:-8000} --workers ${GUNICORN_WORKERS:-2} ${APP_MODULE}"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Flask Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
}

func TestGenerateDjangoDockerfile(t *testing.T) {
	d := Detection{
		Framework:       FrameworkDjango,
		BuildTool:       BuildToolPip,
		PythonVersion:   "3.12",
		AppModule:       "config.wsgi:application",
		SettingsModule:  "config.settings",
		HasRequirements: true,
		HealthcheckPath: "/health/",
	}

	dockerfile := GenerateDockerfile(d)
	for _, want := range []string{
		"RUN python manage.py collectstatic --noinput || true",
		"ENV DJANGO_SETTINGS_MODULE=config.settings",
		"ENV APP_MODULE=config.wsgi:application",
		`CMD ["sh", "-c", "gunicorn --bind 0.0.0.0:${PORT:-8000} --workers ${GUNICORN_WORKERS:-2} ${APP_MODULE}"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("expected generated Django Dockerfile to contain %q\n%s", want, dockerfile)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
