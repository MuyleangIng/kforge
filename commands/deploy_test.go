package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunDeployInitCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("Flask==3.0.3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("from flask import Flask\napp = Flask(__name__)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".kforge.yml"), []byte("image: deploy-demo\nhealthcheck: /health\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runDeployInit(dir, []string{"compose", "render", "fly"}, "", "", "", "", false, false); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Dockerfile", ".dockerignore", "kforge.hcl", "docker-compose.yml", "render.yaml", "fly.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}
