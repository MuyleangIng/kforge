package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MuyleangIng/kforge/internal/project"
	"github.com/spf13/cobra"
)

func DeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Generate deploy bootstrap files from detected project settings",
	}
	cmd.AddCommand(deployInitCmd())
	return cmd
}

func deployInitCmd() *cobra.Command {
	var (
		framework string
		node      string
		python    string
		java      string
		targets   []string
		force     bool
		printOnly bool
	)

	cmd := &cobra.Command{
		Use:   "init [PATH]",
		Short: "Generate docker-compose.yml, render.yaml, and fly.toml",
		Example: `  kforge deploy init
  kforge deploy init ./examples/flask-auto
  kforge deploy init --target compose
  kforge deploy init --print ./examples/django-auto`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			return runDeployInit(root, targets, framework, node, python, java, force, printOnly)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&framework, "framework", "", "Force framework: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&node, "node", "", "Override detected Node.js major version")
	flags.StringVar(&python, "python", "", "Override detected Python version")
	flags.StringVar(&java, "java", "", "Override detected Java version")
	flags.StringSliceVar(&targets, "target", []string{"compose", "render", "fly"}, "Deploy files to generate: compose, render, fly, or all")
	flags.BoolVar(&force, "force", false, "Overwrite existing deploy files")
	flags.BoolVar(&printOnly, "print", false, "Print generated deploy files instead of writing them")

	return cmd
}

func runDeployInit(root string, targets []string, framework, node, python, java string, force, printOnly bool) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	detection, err := autoDetectProject(absRoot, framework, node, python, java)
	if err != nil {
		return err
	}
	cfg, _, err := project.LoadConfig(absRoot)
	if err != nil {
		return err
	}

	selected, err := normalizeDeployTargets(targets)
	if err != nil {
		return err
	}
	deployFiles := generatedDeployFiles(detection, cfg, selected)
	if printOnly {
		printGeneratedFiles(deployFiles)
		return nil
	}

	createdAssets, err := writeMissingProjectFiles(absRoot, generatedProjectFiles(detection, detection.Name))
	if err != nil {
		return err
	}
	createdDeploy, err := writeGeneratedProjectFiles(absRoot, deployFiles, force)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(cGreen + cBold + "Initialized deploy files" + cReset)
	for _, path := range createdDeploy {
		fmt.Printf("  %s\n", path)
	}
	if len(createdAssets) > 0 {
		fmt.Println()
		fmt.Println(cCyan + "Generated missing Docker assets" + cReset)
		for _, path := range createdAssets {
			fmt.Printf("  %s\n", path)
		}
	}
	fmt.Println()
	fmt.Printf("Next: %s$ kforge verify %s%s\n", cCyan, absRoot, cReset)
	return nil
}

func normalizeDeployTargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		return []string{"compose", "render", "fly"}, nil
	}

	seen := map[string]bool{}
	out := []string{}
	for _, raw := range targets {
		for _, part := range strings.Split(raw, ",") {
			target := strings.ToLower(strings.TrimSpace(part))
			if target == "" {
				continue
			}
			if target == "all" {
				return []string{"compose", "render", "fly"}, nil
			}
			switch target {
			case "compose", "render", "fly":
				if !seen[target] {
					seen[target] = true
					out = append(out, target)
				}
			default:
				return nil, fmt.Errorf("unsupported deploy target %q", target)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no deploy targets selected")
	}
	return out, nil
}

func generatedDeployFiles(detection project.Detection, cfg project.Config, targets []string) map[string]string {
	files := map[string]string{}
	for _, target := range targets {
		switch target {
		case "compose":
			files["docker-compose.yml"] = project.GenerateComposeFile(detection, cfg)
		case "render":
			files["render.yaml"] = project.GenerateRenderFile(detection, cfg)
		case "fly":
			files["fly.toml"] = project.GenerateFlyFile(detection, cfg)
		}
	}
	return files
}

func writeMissingProjectFiles(dir string, files map[string]string) ([]string, error) {
	missing := map[string]string{}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing[name] = content
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}
	return writeGeneratedProjectFiles(dir, missing, false)
}

func printGeneratedFiles(files map[string]string) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(cBold + name + cReset)
		fmt.Println(files[name])
	}
}
