package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/MuyleangIng/kforge/internal/project"
	"github.com/spf13/cobra"
)

func InitCmd() *cobra.Command {
	var (
		dir             string
		name            string
		force           bool
		detect          bool
		framework       string
		nodeVersion     string
		pythonVersion   string
		javaVersion     string
		printDockerfile bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create starter Dockerfile, bake file, and demo HTML app",
		Example: `  kforge init
  kforge init --name myapp
  kforge init --detect
  kforge init --detect --framework next
  kforge init --detect --framework fastapi --python 3.12
  kforge init --detect --framework spring --java 21
  kforge init --dir ./demo --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detect || framework != "" {
				return runDetectedInit(dir, name, framework, nodeVersion, pythonVersion, javaVersion, force, printDockerfile)
			}
			return runInit(dir, name, force)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&dir, "dir", ".", "Directory to initialize")
	flags.StringVar(&name, "name", "", "Starter image/app name")
	flags.BoolVar(&force, "force", false, "Overwrite existing files")
	flags.BoolVar(&detect, "detect", false, "Detect the existing project and generate a Dockerfile for it")
	flags.StringVar(&framework, "framework", "", "Force framework: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&nodeVersion, "node", "", "Override detected Node.js major version")
	flags.StringVar(&pythonVersion, "python", "", "Override detected Python version for FastAPI, Django, or Flask projects")
	flags.StringVar(&javaVersion, "java", "", "Override detected Java version for Spring projects")
	flags.BoolVar(&printDockerfile, "print-dockerfile", false, "Print the detected Dockerfile instead of writing files")

	return cmd
}

func runInit(dir, name string, force bool) error {
	if dir == "" {
		dir = "."
	}
	if name == "" {
		name = "myapp"
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	files := map[string]string{
		"Dockerfile":    initDockerfile(),
		".dockerignore": ".DS_Store\n.git\n.gitignore\n",
		"index.html":    initIndexHTML(name),
		"kforge.hcl":    initBakeFile(name),
	}

	created, err := writeGeneratedProjectFiles(dir, files, force)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(cGreen + cBold + "Initialized starter project" + cReset)
	for _, path := range created {
		fmt.Printf("  %s\n", path)
	}
	fmt.Println()
	fmt.Printf("Next: %s$ kforge build --load -t %s:dev %s%s\n",
		cCyan, name, dir, cReset)
	return nil
}

func runDetectedInit(dir, name, framework, nodeVersion, pythonVersion, javaVersion string, force, printDockerfile bool) error {
	detection, err := autoDetectProject(dir, framework, nodeVersion, pythonVersion, javaVersion)
	if err != nil {
		return err
	}
	if name != "" {
		detection.Name = name
	}

	if printDockerfile {
		fmt.Println(project.GenerateDockerfile(detection))
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	files := generatedProjectFiles(detection, detection.Name)
	created, err := writeGeneratedProjectFiles(dir, files, force)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(cGreen + cBold + "Initialized Docker assets from detected project" + cReset)
	for _, path := range created {
		fmt.Printf("  %s\n", path)
	}
	fmt.Println()
	printDetectedProject(detection)
	fmt.Println()
	fmt.Printf("Next: %s$ kforge build --load -t %s:dev %s%s\n",
		cCyan, detection.SuggestedImageName(), dir, cReset)
	return nil
}

func initDockerfile() string {
	return strings.TrimSpace(`FROM nginx:1.27-alpine

COPY index.html /usr/share/nginx/html/index.html
`) + "\n"
}

func initIndexHTML(name string) string {
	return strings.TrimSpace(fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body style="font-family: sans-serif; padding: 2rem;">
  <h1>%s</h1>
  <p>Built with kforge.</p>
</body>
</html>
`, name, name)) + "\n"
}

func initBakeFile(name string) string {
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
`, name)) + "\n"
}
