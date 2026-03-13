package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MuyleangIng/kforge/internal/project"
)

func ensureDockerfileForBuild(opts *buildOptions) (*project.Detection, func(), error) {
	root, err := filepath.Abs(opts.contextPath)
	if err != nil {
		return nil, nil, err
	}

	if opts.dockerfile != "" {
		if opts.auto {
			return nil, nil, fmt.Errorf("--auto cannot be used together with --file")
		}
		return nil, nil, nil
	}

	defaultDockerfile := filepath.Join(root, "Dockerfile")
	if _, err := os.Stat(defaultDockerfile); err == nil {
		return nil, nil, nil
	}

	if !opts.auto {
		return nil, nil, fmt.Errorf("no Dockerfile found in %s; run `kforge init --detect --dir %s` or `kforge build --auto %s`", root, root, opts.contextPath)
	}

	detection, err := autoDetectProject(root, opts.autoFramework, opts.autoNode, opts.autoPython, opts.autoJava)
	if err != nil {
		return nil, nil, err
	}

	tmp, err := os.CreateTemp("", "kforge-auto-dockerfile-*.Dockerfile")
	if err != nil {
		return nil, nil, err
	}
	if _, err := tmp.WriteString(project.GenerateDockerfile(detection)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, nil, err
	}

	opts.dockerfile = tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmp.Name())
	}
	return &detection, cleanup, nil
}
