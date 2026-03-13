package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MuyleangIng/kforge/internal/project"
	"github.com/spf13/cobra"
)

func DetectCmd() *cobra.Command {
	var (
		framework       string
		nodeVersion     string
		pythonVersion   string
		javaVersion     string
		printDockerfile bool
		jsonOutput      bool
	)

	cmd := &cobra.Command{
		Use:   "detect [PATH]",
		Short: "Detect project type and generate a suitable Dockerfile template",
		Example: `  kforge detect
  kforge detect ./apps/web
  kforge detect --print-dockerfile
  kforge detect --framework next --node 20
  kforge detect --framework fastapi --python 3.12
  kforge detect --framework spring --java 21
  kforge detect --framework django --python 3.12
  kforge detect --framework flask --python 3.12`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}

			detection, err := project.DetectWithOverrides(root, framework, nodeVersion, pythonVersion, javaVersion)
			if err != nil {
				return err
			}

			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(detection)
			}

			printDetectedProject(detection)
			if printDockerfile {
				fmt.Println()
				fmt.Println(project.GenerateDockerfile(detection))
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&framework, "framework", "", "Force framework: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&nodeVersion, "node", "", "Override detected Node.js major version")
	flags.StringVar(&pythonVersion, "python", "", "Override detected Python version for FastAPI, Django, or Flask projects")
	flags.StringVar(&javaVersion, "java", "", "Override detected Java version for Spring projects")
	flags.BoolVar(&printDockerfile, "print-dockerfile", false, "Print the generated Dockerfile template")
	flags.BoolVar(&jsonOutput, "json", false, "Print detection result as JSON")

	return cmd
}

func printDetectedProject(d project.Detection) {
	fmt.Println()
	fmt.Println(cBold + "Detected project" + cReset)
	fmt.Printf("  Framework:       %s\n", d.DisplayFramework())
	fmt.Printf("  Root:            %s\n", d.Root)
	if d.ConfigPath != "" {
		fmt.Printf("  Config:          %s\n", d.ConfigPath)
	}
	if d.HasPackageJSON {
		fmt.Printf("  Package manager: %s\n", d.PackageManager)
	}
	if d.BuildTool != "" {
		fmt.Printf("  Build tool:      %s\n", d.BuildTool)
	}
	if d.NodeVersion != "" {
		fmt.Printf("  Node version:    %s (%s)\n", d.NodeVersion, d.NodeVersionFrom)
	}
	if d.JavaVersion != "" {
		fmt.Printf("  Java version:    %s (%s)\n", d.JavaVersion, d.JavaVersionFrom)
	}
	if d.PythonVersion != "" {
		fmt.Printf("  Python version:  %s (%s)\n", d.PythonVersion, d.PythonVersionFrom)
	}
	fmt.Printf("  Suggested image: %s\n", d.SuggestedImageName())
	if d.BuildOutput != "" && d.BuildOutput != "." {
		fmt.Printf("  Build output:    %s\n", d.BuildOutput)
	}
	if d.AppModule != "" {
		fmt.Printf("  App module:      %s\n", d.AppModule)
	}
	if d.SettingsModule != "" {
		fmt.Printf("  Settings module: %s\n", d.SettingsModule)
	}
	if d.HealthcheckPath != "" {
		fmt.Printf("  Healthcheck:     %s\n", d.HealthcheckPath)
	}
	if len(d.EnvDefaults) > 0 {
		fmt.Printf("  Env defaults:    %d value(s)\n", len(d.EnvDefaults))
	}
	if len(d.StartCommand) > 0 {
		fmt.Printf("  Start command:   %s\n", strings.Join(d.StartCommand, " "))
	}
	fmt.Printf("  Has Dockerfile:  %t\n", d.HasDockerfile)
	if len(d.Warnings) > 0 {
		fmt.Println("  Warnings:")
		for _, warning := range d.Warnings {
			fmt.Printf("    - %s\n", warning)
		}
	}
}

func autoDetectProject(root, framework, nodeVersion, pythonVersion, javaVersion string) (project.Detection, error) {
	return project.DetectWithOverrides(root, framework, nodeVersion, pythonVersion, javaVersion)
}

func generatedProjectFiles(d project.Detection, nameOverride string) map[string]string {
	if nameOverride != "" {
		d.Name = nameOverride
	}
	return map[string]string{
		"Dockerfile":    project.GenerateDockerfile(d),
		".dockerignore": project.GenerateDockerignore(d),
		"kforge.hcl":    project.GenerateBakeFile(d),
	}
}

func writeGeneratedProjectFiles(dir string, files map[string]string, force bool) ([]string, error) {
	created := []string{}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if !force {
			if _, err := os.Stat(path); err == nil {
				return nil, fmt.Errorf("%s already exists (use --force to overwrite)", path)
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}
