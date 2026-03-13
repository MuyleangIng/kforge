package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MuyleangIng/kforge/builder"
	"github.com/MuyleangIng/kforge/internal/meta"
	"github.com/spf13/cobra"
)

type doctorCheck struct {
	name     string
	ok       bool
	details  string
	critical bool
}

func DoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Docker, Buildx, and kforge environment health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context())
		},
	}
}

func runDoctor(ctx context.Context) error {
	checks := []doctorCheck{}
	issues := 0

	if path, err := exec.LookPath("docker"); err != nil {
		checks = append(checks, doctorCheck{
			name:     "docker executable",
			ok:       false,
			details:  err.Error(),
			critical: true,
		})
		issues++
		renderDoctor(checks)
		return fmt.Errorf("doctor found %d critical issue(s)", issues)
	} else {
		checks = append(checks, doctorCheck{name: "docker executable", ok: true, details: path, critical: true})
	}

	if out, err := runDoctorCmd(ctx, "docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		checks = append(checks, doctorCheck{
			name:     "docker daemon",
			ok:       false,
			details:  err.Error(),
			critical: true,
		})
		issues++
	} else {
		checks = append(checks, doctorCheck{name: "docker daemon", ok: true, details: out, critical: true})
	}

	if out, err := runDoctorCmd(ctx, "docker", "buildx", "version"); err != nil {
		checks = append(checks, doctorCheck{
			name:     "docker buildx",
			ok:       false,
			details:  err.Error(),
			critical: true,
		})
		issues++
	} else {
		checks = append(checks, doctorCheck{name: "docker buildx", ok: true, details: firstLine(out), critical: true})
	}

	if out, err := runDoctorCmd(ctx, "docker", "context", "show"); err != nil {
		checks = append(checks, doctorCheck{name: "docker context", ok: false, details: err.Error()})
	} else {
		checks = append(checks, doctorCheck{name: "docker context", ok: true, details: out})
	}

	currentBuilder := builder.Current()
	checks = append(checks, doctorCheck{name: "kforge current builder", ok: true, details: currentBuilder})

	cfgs, err := builder.List()
	if err != nil {
		checks = append(checks, doctorCheck{name: "saved kforge builders", ok: false, details: err.Error()})
	} else {
		checks = append(checks, doctorCheck{
			name:    "saved kforge builders",
			ok:      true,
			details: fmt.Sprintf("%d builder(s)", len(cfgs)),
		})
		if currentBuilder != "default" {
			found := false
			for _, cfg := range cfgs {
				if cfg.Name == currentBuilder {
					found = true
					break
				}
			}
			if !found {
				checks = append(checks, doctorCheck{
					name:    "builder/config sync",
					ok:      false,
					details: fmt.Sprintf("current builder %q is not present in ~/.kforge/builders", currentBuilder),
				})
			}
		}
	}

	renderDoctor(checks)
	if issues > 0 {
		return fmt.Errorf("doctor found %d critical issue(s)", issues)
	}
	return nil
}

func renderDoctor(checks []doctorCheck) {
	fmt.Println()
	fmt.Printf("%s%s DOCTOR%s  %s%s%s\n\n", cBold, meta.ToolName, cReset, cDim, meta.DisplayVersion(), cReset)
	for _, check := range checks {
		icon := cGreen + "✓" + cReset
		if !check.ok {
			icon = cYellow + "!" + cReset
			if check.critical {
				icon = cRed + "✗" + cReset
			}
		}
		fmt.Printf("  %s %-22s %s\n", icon, check.name, check.details)
	}
	fmt.Println()
}

func runDoctorCmd(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
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

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
