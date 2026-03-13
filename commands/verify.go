package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type verifyOptions struct {
	contextPath   string
	tag           string
	progress      string
	builderName   string
	auto          bool
	autoFramework string
	autoNode      string
	autoPython    string
	autoJava      string
	env           []string
	envFile       string
	checkPath     string
	containerPort int
	timeout       time.Duration
	keepRunning   bool
}

func VerifyCmd() *cobra.Command {
	opts := &verifyOptions{}

	cmd := &cobra.Command{
		Use:   "verify [PATH]",
		Short: "Build, run, and HTTP-check an image locally",
		Example: `  kforge verify
  kforge verify ./examples/fastapi-auto
  kforge verify --path /health --env APP_ENV=staging .
  kforge verify --auto=false --tag myapp:check .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.contextPath = args[0]
			} else {
				opts.contextPath = "."
			}
			return runVerify(cmd.Context(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.tag, "tag", "t", "", "Image tag to build and verify")
	flags.StringVar(&opts.progress, "progress", "auto", "Build progress style")
	flags.StringVar(&opts.builderName, "builder", "", "Builder to use")
	flags.BoolVar(&opts.auto, "auto", true, "Generate a temporary Dockerfile when none exists")
	flags.StringVar(&opts.autoFramework, "framework", "", "Force project type: next|react|vue|nest|html|node|spring|fastapi|django|flask")
	flags.StringVar(&opts.autoNode, "node", "", "Override detected Node.js major version")
	flags.StringVar(&opts.autoPython, "python", "", "Override detected Python version")
	flags.StringVar(&opts.autoJava, "java", "", "Override detected Java version")
	flags.StringArrayVar(&opts.env, "env", nil, "Runtime environment variable KEY=VALUE")
	flags.StringVar(&opts.envFile, "env-file", "", "Read runtime environment variables from file")
	flags.StringVar(&opts.checkPath, "path", "", "HTTP path to verify (defaults to detected healthcheck)")
	flags.IntVar(&opts.containerPort, "port", 0, "Container port to expose and check")
	flags.DurationVar(&opts.timeout, "timeout", 0, "How long to wait for the HTTP check (defaults to .kforge.yml or 30s)")
	flags.BoolVar(&opts.keepRunning, "keep-running", false, "Keep the container running after verify succeeds")

	return cmd
}

func runVerify(ctx context.Context, opts *verifyOptions) error {
	detection, err := autoDetectProject(opts.contextPath, opts.autoFramework, opts.autoNode, opts.autoPython, opts.autoJava)
	if err != nil {
		return err
	}

	tag := opts.tag
	if tag == "" {
		tag = detection.SuggestedImageName() + ":verify"
	}

	buildOpts := &buildOptions{
		contextPath:   opts.contextPath,
		tags:          []string{tag},
		load:          true,
		progress:      opts.progress,
		builderName:   opts.builderName,
		auto:          opts.auto,
		autoFramework: opts.autoFramework,
		autoNode:      opts.autoNode,
		autoPython:    opts.autoPython,
		autoJava:      opts.autoJava,
	}
	if err := runBuild(ctx, buildOpts); err != nil {
		return err
	}

	containerPort := detection.VerifyPort
	if containerPort == 0 {
		containerPort = detection.Port
	}
	if opts.containerPort > 0 {
		containerPort = opts.containerPort
	}
	if containerPort == 0 {
		containerPort = 80
	}

	checkPath := detection.VerifyPath
	if checkPath == "" {
		checkPath = detection.HealthcheckPath
	}
	if checkPath == "" {
		checkPath = "/"
	}
	if opts.checkPath != "" {
		checkPath = opts.checkPath
	}

	hostPort, err := reserveLocalPort()
	if err != nil {
		return err
	}

	containerName := fmt.Sprintf("kforge-verify-%d", time.Now().UnixNano())
	runArgs := []string{"run", "-d", "--rm", "--name", containerName, "-p", fmt.Sprintf("%d:%d", hostPort, containerPort)}
	if opts.envFile != "" {
		runArgs = append(runArgs, "--env-file", opts.envFile)
	}
	for _, envArg := range formatEnvArgs(mergeEnvMaps(detection.EnvDefaults, parseEnvPairs(opts.env))) {
		runArgs = append(runArgs, "-e", envArg)
	}
	runArgs = append(runArgs, tag)

	if _, err := runDockerOutput(ctx, runArgs...); err != nil {
		return err
	}

	stopContainer := func() {
		_, _ = runDockerOutput(ctx, "stop", containerName)
	}
	if !opts.keepRunning {
		defer stopContainer()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", hostPort, checkPath)
	timeout := opts.timeout
	if timeout == 0 {
		if detection.VerifyTimeout > 0 {
			timeout = time.Duration(detection.VerifyTimeout) * time.Second
		} else {
			timeout = 30 * time.Second
		}
	}

	body, err := waitForHTTP(ctx, url, timeout)
	if err != nil {
		logs, _ := runDockerOutput(ctx, "logs", containerName)
		if logs != "" {
			return fmt.Errorf("verify failed for %s: %w\n%s", url, err, logs)
		}
		return fmt.Errorf("verify failed for %s: %w", url, err)
	}

	fmt.Println()
	fmt.Printf("%s%s VERIFY OK%s  %s%s%s\n", cGreen, cBold, cReset, cDim, tag, cReset)
	fmt.Printf("  URL:        %s\n", url)
	fmt.Printf("  Framework:  %s\n", detection.DisplayFramework())
	fmt.Printf("  Response:   %s\n", summarizeBody(body))
	if opts.keepRunning {
		fmt.Printf("  Container:  %s\n", containerName)
		fmt.Printf("  Stop:       docker stop %s\n", containerName)
	}
	fmt.Println()
	return nil
}

func parseEnvPairs(values []string) map[string]string {
	env := map[string]string{}
	for _, raw := range values {
		key, value, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		env[strings.TrimSpace(key)] = value
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func formatEnvArgs(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, key+"="+env[key])
	}
	return args
}

func mergeEnvMaps(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func reserveLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForHTTP(ctx context.Context, url string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return strings.TrimSpace(string(body)), nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
			}
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("timeout")
	}
	return "", lastErr
}

func summarizeBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "<empty>"
	}
	if len(body) > 120 {
		return body[:117] + "..."
	}
	return body
}

func runDockerOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(string(out)), nil
}
