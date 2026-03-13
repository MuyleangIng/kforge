package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MuyleangIng/kforge/internal/project"
	"github.com/spf13/cobra"
)

func CICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Generate CI/CD bootstrap files for GitHub Actions and GitLab CI",
	}
	cmd.AddCommand(ciInitCmd())
	return cmd
}

func ciInitCmd() *cobra.Command {
	var (
		framework    string
		node         string
		python       string
		java         string
		targets      []string
		deployTarget string
		force        bool
		printOnly    bool
	)

	cmd := &cobra.Command{
		Use:   "init [PATH]",
		Short: "Generate GitHub Actions and GitLab CI pipeline files",
		Example: `  kforge ci init
  kforge ci init ./examples/flask-auto
  kforge ci init --target github
  kforge ci init --deploy compose --print ./examples/django-auto`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			return runCIInit(root, targets, framework, node, python, java, deployTarget, force, printOnly)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&framework, "framework", "", "Force framework: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&node, "node", "", "Override detected Node.js major version")
	flags.StringVar(&python, "python", "", "Override detected Python version")
	flags.StringVar(&java, "java", "", "Override detected Java version")
	flags.StringSliceVar(&targets, "target", []string{"github", "gitlab"}, "CI files to generate: github, gitlab, or all")
	flags.StringVar(&deployTarget, "deploy", "", "Optional deploy stage: none|compose|render|fly")
	flags.BoolVar(&force, "force", false, "Overwrite existing CI files")
	flags.BoolVar(&printOnly, "print", false, "Print generated CI files instead of writing them")

	return cmd
}

func runCIInit(root string, targets []string, framework, node, python, java, deployTarget string, force, printOnly bool) error {
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
	if deployTarget != "" {
		normalized, err := normalizeCIDeployTarget(deployTarget)
		if err != nil {
			return err
		}
		cfg.CI.Deploy = normalized
	}

	selected, err := normalizeCITargets(targets)
	if err != nil {
		return err
	}
	ciFiles := generatedCIFiles(detection, cfg, selected)
	if printOnly {
		printGeneratedFiles(ciFiles)
		return nil
	}

	createdAssets, err := writeMissingProjectFiles(absRoot, generatedProjectFiles(detection, detection.Name))
	if err != nil {
		return err
	}

	createdDeploy := []string(nil)
	if cfg.CI.Deploy != "" && cfg.CI.Deploy != "none" {
		deployFiles := generatedDeployFiles(detection, cfg, []string{cfg.CI.Deploy})
		createdDeploy, err = writeMissingProjectFiles(absRoot, deployFiles)
		if err != nil {
			return err
		}
	}

	createdCI, err := writeGeneratedProjectFiles(absRoot, ciFiles, force)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(cGreen + cBold + "Initialized CI/CD files" + cReset)
	for _, path := range createdCI {
		fmt.Printf("  %s\n", path)
	}
	if len(createdDeploy) > 0 {
		fmt.Println()
		fmt.Println(cCyan + "Generated missing deploy files" + cReset)
		for _, path := range createdDeploy {
			fmt.Printf("  %s\n", path)
		}
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

func normalizeCITargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		return []string{"github", "gitlab"}, nil
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
				return []string{"github", "gitlab"}, nil
			}
			switch target {
			case "github", "gitlab":
				if !seen[target] {
					seen[target] = true
					out = append(out, target)
				}
			default:
				return nil, fmt.Errorf("unsupported CI target %q", target)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CI targets selected")
	}
	return out, nil
}

func normalizeCIDeployTarget(target string) (string, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	switch target {
	case "", "none", "compose", "render", "fly":
		return target, nil
	default:
		return "", fmt.Errorf("unsupported CI deploy target %q", target)
	}
}

func generatedCIFiles(detection project.Detection, cfg project.Config, targets []string) map[string]string {
	files := map[string]string{}
	for _, target := range targets {
		switch target {
		case "github":
			spec := project.ResolveCISpec(detection, cfg, "github")
			name := spec.GitHubWorkflowFile
			if !strings.Contains(name, "/") {
				name = filepath.Join(".github", "workflows", name)
			}
			if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
				name += ".yml"
			}
			files[name] = project.GenerateGitHubActionsWorkflow(detection, cfg)
		case "gitlab":
			spec := project.ResolveCISpec(detection, cfg, "gitlab")
			name := spec.GitLabFile
			if name == "" {
				name = ".gitlab-ci.yml"
			}
			files[name] = project.GenerateGitLabCI(detection, cfg)
		}
	}
	return files
}
