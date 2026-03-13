package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInitCreatesStarterFiles(t *testing.T) {
	dir := t.TempDir()

	if err := runInit(dir, "demoapp", false); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Dockerfile", ".dockerignore", "index.html", "kforge.hcl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}
