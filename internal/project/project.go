package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Framework string

const (
	FrameworkUnknown Framework = ""
	FrameworkNext    Framework = "next"
	FrameworkReact   Framework = "react"
	FrameworkVue     Framework = "vue"
	FrameworkNest    Framework = "nest"
	FrameworkHTML    Framework = "html"
	FrameworkNode    Framework = "node"
	FrameworkSpring  Framework = "spring"
	FrameworkFastAPI Framework = "fastapi"
	FrameworkFlask   Framework = "flask"
	FrameworkDjango  Framework = "django"
)

type PackageManager string

const (
	PackageManagerNone PackageManager = ""
	PackageManagerNPM  PackageManager = "npm"
	PackageManagerYarn PackageManager = "yarn"
	PackageManagerPNPM PackageManager = "pnpm"
)

type BuildTool string

const (
	BuildToolNone   BuildTool = ""
	BuildToolMaven  BuildTool = "maven"
	BuildToolGradle BuildTool = "gradle"
	BuildToolPip    BuildTool = "pip"
)

type Detection struct {
	Root              string
	Name              string
	Framework         Framework
	ConfigPath        string
	PackageManager    PackageManager
	BuildTool         BuildTool
	HasLockfile       bool
	NodeVersion       string
	NodeVersionFrom   string
	PythonVersion     string
	PythonVersionFrom string
	JavaVersion       string
	JavaVersionFrom   string
	BuildOutput       string
	Port              int
	HasDockerfile     bool
	HasPackageJSON    bool
	HasBuildScript    bool
	HasPublicDir      bool
	HasRequirements   bool
	HasPyProject      bool
	HasPomXML         bool
	HasGradleBuild    bool
	NextStandalone    bool
	AppModule         string
	SettingsModule    string
	HealthcheckPath   string
	StartCommand      []string
	EnvDefaults       map[string]string
	VerifyPath        string
	VerifyPort        int
	VerifyTimeout     int
	Warnings          []string

	scripts map[string]string
}

type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Engines         struct {
		Node string `json:"node"`
	} `json:"engines"`
}

func Detect(root, frameworkOverride, nodeOverride string) (Detection, error) {
	return DetectWithOverrides(root, frameworkOverride, nodeOverride, "", "")
}

func DetectWithOverrides(root, frameworkOverride, nodeOverride, pythonOverride, javaOverride string) (Detection, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Detection{}, err
	}

	cfg, cfgPath, err := LoadConfig(absRoot)
	if err != nil {
		return Detection{}, fmt.Errorf("load %s: %w", filepath.Base(cfgPath), err)
	}

	d := Detection{
		Root:            absRoot,
		ConfigPath:      cfgPath,
		HasDockerfile:   fileExists(filepath.Join(absRoot, "Dockerfile")),
		HasRequirements: fileExists(filepath.Join(absRoot, "requirements.txt")),
		HasPyProject:    fileExists(filepath.Join(absRoot, "pyproject.toml")),
		HasPomXML:       fileExists(filepath.Join(absRoot, "pom.xml")),
		HasGradleBuild:  fileExists(filepath.Join(absRoot, "build.gradle")) || fileExists(filepath.Join(absRoot, "build.gradle.kts")),
	}

	pkg, pkgExists, err := readPackageJSON(absRoot)
	if err != nil {
		return Detection{}, err
	}
	if pkgExists {
		d.HasPackageJSON = true
		d.Name = sanitizeName(pkg.Name)
		d.scripts = pkg.Scripts
		d.PackageManager, d.HasLockfile = detectPackageManager(absRoot)
		if d.PackageManager == PackageManagerNone {
			d.PackageManager = PackageManagerNPM
		}
	}

	if d.Name == "" {
		d.Name = detectProjectName(absRoot)
	}
	if d.Name == "" {
		d.Name = sanitizeName(filepath.Base(absRoot))
	}

	effectiveFramework := frameworkOverride
	if effectiveFramework == "" {
		effectiveFramework = cfg.Framework
	}
	effectiveNode := nodeOverride
	if effectiveNode == "" {
		effectiveNode = cfg.Node
	}
	effectivePython := pythonOverride
	if effectivePython == "" {
		effectivePython = cfg.Python
	}
	effectiveJava := javaOverride
	if effectiveJava == "" {
		effectiveJava = cfg.Java
	}

	if effectiveFramework != "" {
		d.Framework = Framework(strings.ToLower(effectiveFramework))
		if !isSupportedFramework(d.Framework) {
			return Detection{}, fmt.Errorf("unsupported framework %q", effectiveFramework)
		}
	} else {
		d.Framework = detectFramework(absRoot, pkg, pkgExists)
	}

	if d.Framework == FrameworkUnknown {
		return Detection{}, fmt.Errorf("could not detect project type in %s", absRoot)
	}

	switch d.Framework {
	case FrameworkHTML:
		d.Port = 80
		d.BuildOutput = "."
		d.StartCommand = []string{"nginx", "-g", "daemon off;"}
	case FrameworkNext, FrameworkReact, FrameworkVue, FrameworkNest, FrameworkNode:
		if !pkgExists {
			return Detection{}, fmt.Errorf("%s project detection requires package.json", d.Framework)
		}
		d.Port = 3000
		d.NodeVersion, d.NodeVersionFrom = detectNodeVersion(absRoot, pkg, effectiveNode)
		d.HasBuildScript = hasScript(pkg.Scripts, "build")
		if d.Framework == FrameworkNext {
			d.NextStandalone = detectNextStandalone(absRoot)
			d.HasPublicDir = fileExists(filepath.Join(absRoot, "public"))
		}
		d.StartCommand = detectNodeStartCommand(absRoot, d.Framework, pkg, d.NextStandalone)
		d.BuildOutput = detectNodeBuildOutput(d.Framework, pkg, d.NextStandalone)
	case FrameworkSpring:
		d.Port = 8080
		d.BuildTool = detectSpringBuildTool(absRoot)
		if d.BuildTool == BuildToolNone {
			return Detection{}, fmt.Errorf("could not determine Spring build tool in %s", absRoot)
		}
		d.JavaVersion, d.JavaVersionFrom = detectJavaVersion(absRoot, effectiveJava)
		d.BuildOutput = "app.jar"
		d.StartCommand = []string{"java", "-jar", "app.jar"}
		d.HealthcheckPath = detectSpringHealthcheck(absRoot)
	case FrameworkFastAPI, FrameworkFlask, FrameworkDjango:
		if !d.HasRequirements && !d.HasPyProject {
			return Detection{}, fmt.Errorf("%s project detection requires requirements.txt or pyproject.toml", d.Framework)
		}
		d.Port = 8000
		d.BuildTool = BuildToolPip
		d.PythonVersion, d.PythonVersionFrom = detectPythonVersion(absRoot, effectivePython)
		switch d.Framework {
		case FrameworkFastAPI:
			d.AppModule, d.HealthcheckPath = detectFastAPIApp(absRoot)
			if d.AppModule == "" {
				return Detection{}, fmt.Errorf("could not determine FastAPI application module in %s", absRoot)
			}
			d.StartCommand = []string{"uvicorn", d.AppModule, "--host", "0.0.0.0", "--port", "8000"}
		case FrameworkFlask:
			d.AppModule, d.HealthcheckPath = detectFlaskApp(absRoot)
			if d.AppModule == "" {
				return Detection{}, fmt.Errorf("could not determine Flask application module in %s", absRoot)
			}
			d.StartCommand = []string{"gunicorn", "--bind", "0.0.0.0:8000", d.AppModule}
		case FrameworkDjango:
			d.SettingsModule, d.AppModule, d.HealthcheckPath = detectDjangoApp(absRoot)
			if d.AppModule == "" {
				return Detection{}, fmt.Errorf("could not determine Django WSGI application in %s", absRoot)
			}
			d.StartCommand = []string{"gunicorn", "--bind", "0.0.0.0:8000", d.AppModule}
		}
		d.BuildOutput = "."
	default:
		return Detection{}, fmt.Errorf("unsupported framework %q", d.Framework)
	}

	applyConfig(&d, cfg)

	if len(d.StartCommand) == 0 && d.Framework != FrameworkHTML {
		return Detection{}, fmt.Errorf("could not determine how to start this %s project", d.Framework)
	}

	return d, nil
}

func (d Detection) SuggestedImageName() string {
	if d.Name == "" {
		return "myapp"
	}
	return d.Name
}

func (d Detection) DisplayFramework() string {
	switch d.Framework {
	case FrameworkSpring:
		return "spring-boot"
	case FrameworkFastAPI:
		return "fastapi"
	default:
		if d.Framework == FrameworkUnknown {
			return "unknown"
		}
		return string(d.Framework)
	}
}

func (d Detection) RuntimeDisplay() string {
	switch {
	case d.NodeVersion != "":
		return "Node " + d.NodeVersion
	case d.JavaVersion != "":
		return "Java " + d.JavaVersion
	case d.PythonVersion != "":
		return "Python " + d.PythonVersion
	default:
		return ""
	}
}

func (d Detection) RuntimeSource() string {
	switch {
	case d.NodeVersion != "":
		return d.NodeVersionFrom
	case d.JavaVersion != "":
		return d.JavaVersionFrom
	case d.PythonVersion != "":
		return d.PythonVersionFrom
	default:
		return ""
	}
}

func (d Detection) ToolchainDisplay() string {
	if d.PackageManager != PackageManagerNone {
		return string(d.PackageManager)
	}
	if d.BuildTool != BuildToolNone {
		return string(d.BuildTool)
	}
	return ""
}

func isSupportedFramework(f Framework) bool {
	switch f {
	case FrameworkNext, FrameworkReact, FrameworkVue, FrameworkNest, FrameworkHTML, FrameworkNode, FrameworkSpring, FrameworkFastAPI, FrameworkFlask, FrameworkDjango:
		return true
	default:
		return false
	}
}

func detectFramework(root string, pkg packageJSON, pkgExists bool) Framework {
	if pkgExists {
		switch {
		case hasPackage(pkg, "next"):
			return FrameworkNext
		case hasPackage(pkg, "@nestjs/core") || fileExists(filepath.Join(root, "nest-cli.json")):
			return FrameworkNest
		case hasPackage(pkg, "vue"):
			return FrameworkVue
		case hasPackage(pkg, "react") || hasPackage(pkg, "react-dom"):
			return FrameworkReact
		default:
			return FrameworkNode
		}
	}

	if isSpringProject(root) {
		return FrameworkSpring
	}
	if isFastAPIProject(root) {
		return FrameworkFastAPI
	}
	if isDjangoProject(root) {
		return FrameworkDjango
	}
	if isFlaskProject(root) {
		return FrameworkFlask
	}
	if fileExists(filepath.Join(root, "index.html")) {
		return FrameworkHTML
	}
	return FrameworkUnknown
}

func detectPackageManager(root string) (PackageManager, bool) {
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return PackageManagerPNPM, true
	case fileExists(filepath.Join(root, "yarn.lock")):
		return PackageManagerYarn, true
	case fileExists(filepath.Join(root, "package-lock.json")), fileExists(filepath.Join(root, "npm-shrinkwrap.json")):
		return PackageManagerNPM, true
	default:
		return PackageManagerNone, false
	}
}

func detectNodeVersion(root string, pkg packageJSON, override string) (string, string) {
	if major := resolveNodeMajor(override); major != "" {
		return major, "flag"
	}
	if major := resolveNodeMajor(pkg.Engines.Node); major != "" {
		return major, "package.json engines.node"
	}
	if data, ok := readOptionalFile(filepath.Join(root, ".nvmrc")); ok {
		if major := resolveNodeMajor(data); major != "" {
			return major, ".nvmrc"
		}
	}
	if data, ok := readOptionalFile(filepath.Join(root, ".node-version")); ok {
		if major := resolveNodeMajor(data); major != "" {
			return major, ".node-version"
		}
	}
	return "22", "default"
}

func detectPythonVersion(root, override string) (string, string) {
	if version := resolvePythonVersion(override); version != "" {
		return version, "flag"
	}
	if data, ok := readOptionalFile(filepath.Join(root, "pyproject.toml")); ok {
		re := regexp.MustCompile(`(?m)^\s*requires-python\s*=\s*["']([^"']+)["']`)
		if match := re.FindStringSubmatch(data); len(match) > 1 {
			if version := resolvePythonVersion(match[1]); version != "" {
				return version, "pyproject.toml requires-python"
			}
		}
	}
	if data, ok := readOptionalFile(filepath.Join(root, ".python-version")); ok {
		if version := resolvePythonVersion(data); version != "" {
			return version, ".python-version"
		}
	}
	if data, ok := readOptionalFile(filepath.Join(root, "runtime.txt")); ok {
		if version := resolvePythonVersion(data); version != "" {
			return version, "runtime.txt"
		}
	}
	return "3.12", "default"
}

func detectJavaVersion(root, override string) (string, string) {
	if version := resolveJavaMajor(override); version != "" {
		return version, "flag"
	}
	for _, candidate := range []struct {
		path    string
		pattern string
		label   string
	}{
		{filepath.Join(root, "pom.xml"), `(?s)<java\.version>\s*([^<]+)\s*</java\.version>`, "pom.xml java.version"},
		{filepath.Join(root, "pom.xml"), `(?s)<maven\.compiler\.release>\s*([^<]+)\s*</maven\.compiler\.release>`, "pom.xml maven.compiler.release"},
		{filepath.Join(root, "pom.xml"), `(?s)<maven\.compiler\.source>\s*([^<]+)\s*</maven\.compiler\.source>`, "pom.xml maven.compiler.source"},
		{filepath.Join(root, "build.gradle"), `JavaLanguageVersion\.of\((\d+)\)`, "build.gradle toolchain"},
		{filepath.Join(root, "build.gradle.kts"), `JavaLanguageVersion\.of\((\d+)\)`, "build.gradle.kts toolchain"},
		{filepath.Join(root, "build.gradle"), `sourceCompatibility\s*=\s*JavaVersion\.VERSION_(\d+)`, "build.gradle sourceCompatibility"},
		{filepath.Join(root, "build.gradle.kts"), `sourceCompatibility\s*=\s*JavaVersion\.VERSION_(\d+)`, "build.gradle.kts sourceCompatibility"},
	} {
		if data, ok := readOptionalFile(candidate.path); ok {
			re := regexp.MustCompile(candidate.pattern)
			if match := re.FindStringSubmatch(data); len(match) > 1 {
				if version := resolveJavaMajor(match[1]); version != "" {
					return version, candidate.label
				}
			}
		}
	}
	return "21", "default"
}

func resolveNodeMajor(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if raw == "" {
		return ""
	}
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(raw, -1)
	for _, match := range matches {
		n, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		if n >= 14 && n <= 30 {
			return strconv.Itoa(n)
		}
	}
	return ""
}

func resolveJavaMajor(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if raw == "" {
		return ""
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(raw)
	if match == "" {
		return ""
	}
	n, err := strconv.Atoi(match)
	if err != nil || n < 8 || n > 30 {
		return ""
	}
	return strconv.Itoa(n)
}

func resolvePythonVersion(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "python-"))
	if raw == "" {
		return ""
	}
	re := regexp.MustCompile(`\d+\.\d+|\d+`)
	match := re.FindString(raw)
	if match == "" {
		return ""
	}
	if strings.Count(match, ".") == 0 {
		if match == "3" {
			return "3.12"
		}
		return ""
	}
	parts := strings.Split(match, ".")
	if len(parts) < 2 {
		return ""
	}
	if parts[0] != "3" {
		return ""
	}
	return parts[0] + "." + parts[1]
}

func detectNodeStartCommand(root string, framework Framework, pkg packageJSON, nextStandalone bool) []string {
	switch framework {
	case FrameworkNext:
		if nextStandalone {
			return []string{"node", "server.js"}
		}
		if hasScript(pkg.Scripts, "start") {
			pm, _ := detectPackageManager(root)
			return packageManagerScriptCmd(pm, "start")
		}
		return nil
	case FrameworkNest:
		return []string{"node", "dist/main.js"}
	case FrameworkReact, FrameworkVue:
		return []string{"nginx", "-g", "daemon off;"}
	case FrameworkNode:
		if hasScript(pkg.Scripts, "start") {
			pm, _ := detectPackageManager(root)
			return packageManagerScriptCmd(pm, "start")
		}
		switch {
		case fileExists(filepath.Join(root, "server.js")):
			return []string{"node", "server.js"}
		case fileExists(filepath.Join(root, "index.js")):
			return []string{"node", "index.js"}
		case fileExists(filepath.Join(root, "dist", "main.js")):
			return []string{"node", "dist/main.js"}
		case fileExists(filepath.Join(root, "dist", "index.js")):
			return []string{"node", "dist/index.js"}
		default:
			return []string{"node", "index.js"}
		}
	default:
		return nil
	}
}

func detectNodeBuildOutput(framework Framework, pkg packageJSON, nextStandalone bool) string {
	switch framework {
	case FrameworkReact:
		if script := pkg.Scripts["build"]; strings.Contains(script, "react-scripts") {
			return "build"
		}
		return "dist"
	case FrameworkVue:
		return "dist"
	case FrameworkNext:
		if nextStandalone {
			return ".next/standalone"
		}
		return ".next"
	case FrameworkNest:
		return "dist"
	case FrameworkNode:
		if hasScript(pkg.Scripts, "build") {
			return "dist"
		}
		return "."
	default:
		return "."
	}
}

func detectNextStandalone(root string) bool {
	re := regexp.MustCompile(`output\s*:\s*['"]standalone['"]`)
	for _, name := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && re.MatchString(data) {
			return true
		}
	}
	return false
}

func isSpringProject(root string) bool {
	for _, name := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && strings.Contains(strings.ToLower(data), "spring-boot") {
			return true
		}
	}
	return sourceTreeContains(root, func(path, data string) bool {
		if !strings.HasSuffix(path, ".java") && !strings.HasSuffix(path, ".kt") {
			return false
		}
		return strings.Contains(data, "@SpringBootApplication")
	})
}

func isFastAPIProject(root string) bool {
	for _, name := range []string{"requirements.txt", "pyproject.toml"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && strings.Contains(strings.ToLower(data), "fastapi") {
			return true
		}
	}
	module, _ := detectFastAPIApp(root)
	return module != ""
}

func isFlaskProject(root string) bool {
	for _, name := range []string{"requirements.txt", "pyproject.toml"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && strings.Contains(strings.ToLower(data), "flask") {
			return true
		}
	}
	module, _ := detectFlaskApp(root)
	return module != ""
}

func isDjangoProject(root string) bool {
	if fileExists(filepath.Join(root, "manage.py")) {
		return true
	}
	for _, name := range []string{"requirements.txt", "pyproject.toml"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && strings.Contains(strings.ToLower(data), "django") {
			return true
		}
	}
	_, appModule, _ := detectDjangoApp(root)
	return appModule != ""
}

func detectSpringBuildTool(root string) BuildTool {
	switch {
	case fileExists(filepath.Join(root, "pom.xml")):
		return BuildToolMaven
	case fileExists(filepath.Join(root, "build.gradle")), fileExists(filepath.Join(root, "build.gradle.kts")):
		return BuildToolGradle
	default:
		return BuildToolNone
	}
}

func detectSpringHealthcheck(root string) string {
	for _, name := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if data, ok := readOptionalFile(filepath.Join(root, name)); ok && strings.Contains(strings.ToLower(data), "spring-boot-starter-actuator") {
			return "/actuator/health"
		}
	}
	if path := detectMappedPath(root, []string{".java", ".kt"}, []string{"/health", "/healthz"}); path != "" {
		return path
	}
	return ""
}

func detectFastAPIApp(root string) (string, string) {
	for _, rel := range []string{"main.py", "app.py", filepath.Join("app", "main.py"), filepath.Join("src", "main.py")} {
		path := filepath.Join(root, rel)
		if data, ok := readOptionalFile(path); ok {
			if module, health := fastAPIModuleFromFile(root, path, data); module != "" {
				return module, health
			}
		}
	}

	var module string
	var health string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "__pycache__", ".venv", "venv", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		data, ok := readOptionalFile(path)
		if !ok {
			return nil
		}
		module, health = fastAPIModuleFromFile(root, path, data)
		if module != "" {
			return fs.SkipAll
		}
		return nil
	})
	return module, health
}

func detectFlaskApp(root string) (string, string) {
	for _, rel := range []string{"app.py", "main.py", filepath.Join("src", "app.py"), filepath.Join("src", "main.py")} {
		path := filepath.Join(root, rel)
		if data, ok := readOptionalFile(path); ok {
			if module, health := flaskModuleFromFile(root, path, data); module != "" {
				return module, health
			}
		}
	}

	var module string
	var health string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "__pycache__", ".venv", "venv", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		data, ok := readOptionalFile(path)
		if !ok {
			return nil
		}
		module, health = flaskModuleFromFile(root, path, data)
		if module != "" {
			return fs.SkipAll
		}
		return nil
	})
	return module, health
}

func detectDjangoApp(root string) (string, string, string) {
	settingsModule := detectDjangoSettingsModule(root)
	if settingsModule == "" {
		return "", "", ""
	}
	wsgiModule := strings.TrimSuffix(settingsModule, ".settings") + ".wsgi:application"
	health := "/"
	if path := detectDjangoHealthcheck(root); path != "" {
		health = path
	}
	return settingsModule, wsgiModule, health
}

func flaskModuleFromFile(root, path, data string) (string, string) {
	if !strings.Contains(data, "Flask(") {
		return "", ""
	}
	varName := "app"
	re := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*Flask\(`)
	if match := re.FindStringSubmatch(data); len(match) > 1 {
		varName = match[1]
	}
	module, err := moduleNameFromPath(root, path)
	if err != nil {
		return "", ""
	}
	health := detectPythonRoute(data, []string{"/health", "/healthz", "/ready", "/"})
	return module + ":" + varName, health
}

func detectDjangoSettingsModule(root string) string {
	if data, ok := readOptionalFile(filepath.Join(root, "manage.py")); ok {
		re := regexp.MustCompile(`DJANGO_SETTINGS_MODULE["']?\s*,\s*["']([^"']+)["']`)
		if match := re.FindStringSubmatch(data); len(match) > 1 {
			return match[1]
		}
	}

	var module string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "__pycache__", ".venv", "venv", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "settings.py" {
			return nil
		}
		mod, err := moduleNameFromPath(root, path)
		if err != nil {
			return nil
		}
		module = mod
		return fs.SkipAll
	})
	return module
}

func detectDjangoHealthcheck(root string) string {
	var found string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "__pycache__", ".venv", "venv", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "urls.py" {
			return nil
		}
		data, ok := readOptionalFile(path)
		if !ok {
			return nil
		}
		lowered := strings.ToLower(data)
		switch {
		case strings.Contains(lowered, `"health/"`) || strings.Contains(lowered, `'health/'`):
			found = "/health/"
		case strings.Contains(lowered, `"health"`) || strings.Contains(lowered, `'health'`):
			found = "/health"
		case strings.Contains(lowered, `"healthz/"`) || strings.Contains(lowered, `'healthz/'`):
			found = "/healthz/"
		}
		if found != "" {
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func fastAPIModuleFromFile(root, path, data string) (string, string) {
	if !strings.Contains(data, "FastAPI(") {
		return "", ""
	}
	varName := "app"
	re := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*FastAPI\(`)
	if match := re.FindStringSubmatch(data); len(match) > 1 {
		varName = match[1]
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", ""
	}
	module := strings.TrimSuffix(rel, ".py")
	module = filepath.ToSlash(module)
	module = strings.ReplaceAll(module, "/", ".")
	module = strings.TrimSuffix(module, ".__init__")
	health := detectPythonRoute(data, []string{"/health", "/healthz", "/ready"})
	return module + ":" + varName, health
}

func detectPythonRoute(data string, candidates []string) string {
	lowered := strings.ToLower(data)
	for _, candidate := range candidates {
		if strings.Contains(lowered, `"`+strings.ToLower(candidate)+`"`) || strings.Contains(lowered, `'`+strings.ToLower(candidate)+`'`) {
			return candidate
		}
	}
	return ""
}

func detectMappedPath(root string, suffixes, candidates []string) string {
	var found string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "target", "build", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		okSuffix := false
		for _, suffix := range suffixes {
			if strings.HasSuffix(path, suffix) {
				okSuffix = true
				break
			}
		}
		if !okSuffix {
			return nil
		}
		data, ok := readOptionalFile(path)
		if !ok {
			return nil
		}
		lowered := strings.ToLower(data)
		for _, candidate := range candidates {
			if strings.Contains(lowered, strings.ToLower(candidate)) {
				found = candidate
				return fs.SkipAll
			}
		}
		return nil
	})
	return found
}

func sourceTreeContains(root string, match func(path, data string) bool) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "target", "build", "__pycache__", ".venv", "venv":
				return filepath.SkipDir
			}
			return nil
		}
		data, ok := readOptionalFile(path)
		if !ok {
			return nil
		}
		if match(path, data) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func detectProjectName(root string) string {
	if data, ok := readOptionalFile(filepath.Join(root, "pyproject.toml")); ok {
		re := regexp.MustCompile(`(?m)^\s*name\s*=\s*["']([^"']+)["']`)
		if match := re.FindStringSubmatch(data); len(match) > 1 {
			if name := sanitizeName(match[1]); name != "" {
				return name
			}
		}
	}
	if data, ok := readOptionalFile(filepath.Join(root, "pom.xml")); ok {
		parentRe := regexp.MustCompile(`(?s)<parent>.*?</parent>`)
		data = parentRe.ReplaceAllString(data, "")
		re := regexp.MustCompile(`(?s)<artifactId>\s*([^<]+)\s*</artifactId>`)
		if match := re.FindStringSubmatch(data); len(match) > 1 {
			if name := sanitizeName(match[1]); name != "" {
				return name
			}
		}
	}
	return ""
}

func applyConfig(d *Detection, cfg Config) {
	if cfg.Image != "" {
		d.Name = sanitizeName(cfg.Image)
	}
	if cfg.Port > 0 {
		d.Port = cfg.Port
	}
	if cfg.Healthcheck != "" {
		d.HealthcheckPath = cfg.Healthcheck
	}
	if cfg.AppModule != "" {
		d.AppModule = cfg.AppModule
	}
	if cfg.SettingsModule != "" {
		d.SettingsModule = cfg.SettingsModule
	}
	if len(cfg.StartCommand) > 0 {
		d.StartCommand = append([]string(nil), cfg.StartCommand...)
	}
	if len(cfg.Env) > 0 {
		d.EnvDefaults = cloneStringMap(cfg.Env)
	}
	if cfg.Verify.Path != "" {
		d.VerifyPath = cfg.Verify.Path
	} else {
		d.VerifyPath = d.HealthcheckPath
	}
	if cfg.Verify.Port > 0 {
		d.VerifyPort = cfg.Verify.Port
	} else {
		d.VerifyPort = d.Port
	}
	if cfg.Verify.TimeoutSeconds > 0 {
		d.VerifyTimeout = cfg.Verify.TimeoutSeconds
	}
	if len(cfg.Verify.Env) > 0 {
		d.EnvDefaults = mergeStringMaps(d.EnvDefaults, cfg.Verify.Env)
	}

	switch d.Framework {
	case FrameworkFastAPI:
		if d.AppModule != "" && len(cfg.StartCommand) == 0 {
			d.StartCommand = []string{"uvicorn", d.AppModule, "--host", "0.0.0.0", "--port", strconv.Itoa(d.Port)}
		}
	case FrameworkFlask:
		if d.AppModule != "" && len(cfg.StartCommand) == 0 {
			d.StartCommand = []string{"gunicorn", "--bind", "0.0.0.0:" + strconv.Itoa(d.Port), d.AppModule}
		}
	case FrameworkDjango:
		if d.SettingsModule != "" && d.AppModule == "" {
			d.AppModule = strings.TrimSuffix(d.SettingsModule, ".settings") + ".wsgi:application"
		}
		if d.AppModule != "" && len(cfg.StartCommand) == 0 {
			d.StartCommand = []string{"gunicorn", "--bind", "0.0.0.0:" + strconv.Itoa(d.Port), d.AppModule}
		}
	}

	if d.VerifyPath == "" {
		d.VerifyPath = d.HealthcheckPath
	}
	if d.VerifyPort == 0 {
		d.VerifyPort = d.Port
	}
}

func moduleNameFromPath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	module := strings.TrimSuffix(rel, ".py")
	module = filepath.ToSlash(module)
	module = strings.ReplaceAll(module, "/", ".")
	module = strings.TrimSuffix(module, ".__init__")
	return module, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeStringMaps(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := cloneStringMap(base)
	if out == nil {
		out = map[string]string{}
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func packageManagerScriptCmd(pm PackageManager, script string) []string {
	switch pm {
	case PackageManagerPNPM:
		return []string{"pnpm", script}
	case PackageManagerYarn:
		return []string{"yarn", script}
	default:
		return []string{"npm", "run", script}
	}
}

func hasPackage(pkg packageJSON, name string) bool {
	if _, ok := pkg.Dependencies[name]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[name]; ok {
		return true
	}
	return false
}

func hasScript(scripts map[string]string, name string) bool {
	if scripts == nil {
		return false
	}
	_, ok := scripts[name]
	return ok
}

func readPackageJSON(root string) (packageJSON, bool, error) {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return packageJSON{}, false, nil
		}
		return packageJSON{}, false, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return packageJSON{}, false, fmt.Errorf("parse package.json: %w", err)
	}
	return pkg, true, nil
}

func readOptionalFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "@")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if name == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	name = re.ReplaceAllString(strings.ToLower(name), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "myapp"
	}
	return name
}
