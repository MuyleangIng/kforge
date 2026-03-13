package bake

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHCLVariableDefaultAndEnvOverride(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "kforge.hcl")
		src := `variable "TAG" {
  default = "latest"
}

target "app" {
  tags = ["repo/app:${TAG}"]
}
`
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		file, err := parseHCL(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := file.Targets[0].Tags[0]; got != "repo/app:latest" {
			t.Fatalf("expected default tag expansion, got %q", got)
		}
	})

	t.Run("environment override", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "kforge.hcl")
		src := `variable "TAG" {
  default = "latest"
}

target "app" {
  tags = ["repo/app:${TAG}"]
}
`
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		t.Setenv("TAG", "1.2.3")
		file, err := parseHCL(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := file.Targets[0].Tags[0]; got != "repo/app:1.2.3" {
			t.Fatalf("expected env override tag expansion, got %q", got)
		}
	})
}
