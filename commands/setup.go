package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// ANSI helpers (reuse from build context, no external dep needed)
const (
	sReset  = "\033[0m"
	sBold   = "\033[1m"
	sDim    = "\033[2m"
	sRed    = "\033[31m"
	sGreen  = "\033[32m"
	sYellow = "\033[33m"
	sBlue   = "\033[34m"
	sCyan   = "\033[36m"
	sWhite  = "\033[97m"
	sGray   = "\033[90m"
)

// SetupCmd returns the `kforge setup` interactive wizard command.
func SetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard to set up multi-platform builders",
		Long: `Interactive setup wizard for kforge multi-platform building.

Guides you through:
  1. QEMU emulation   — build all platforms on one machine using QEMU
  2. Multi-node        — use native nodes for each platform (faster, no emulation)
  3. Both              — QEMU as fallback + native nodes for speed`,
		Example: `  kforge setup
  docker kforge setup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup()
		},
	}
}

// ─── WIZARD ──────────────────────────────────────────────────────────────────

func runSetup() error {
	printBanner()

	fmt.Println(sBold + "Welcome to kforge setup!" + sReset)
	fmt.Println(sDim + "This wizard configures multi-platform image building." + sReset)
	fmt.Println()

	mode := promptMenu("Choose your build strategy:", []menuItem{
		{Key: "1", Label: "QEMU emulation",
			Desc: "Build all platforms on this machine using QEMU (easiest, slower for ARM)"},
		{Key: "2", Label: "Multiple native nodes",
			Desc: "Use separate machines/contexts per platform (fastest, needs remote nodes)"},
		{Key: "3", Label: "Both (recommended)",
			Desc: "Native nodes first, QEMU as fallback for remaining platforms"},
		{Key: "q", Label: "Quit", Desc: "Exit without making changes"},
	})

	switch mode {
	case "1":
		return setupQEMU()
	case "2":
		return setupMultiNode()
	case "3":
		if err := setupQEMU(); err != nil {
			return err
		}
		return setupMultiNode()
	case "q":
		fmt.Println(sGray + "Exited without changes." + sReset)
		return nil
	}
	return nil
}

// ─── QEMU SETUP ──────────────────────────────────────────────────────────────

func setupQEMU() error {
	fmt.Println()
	printSection("Step: Install QEMU emulators")
	fmt.Println("This runs a privileged Docker container that installs QEMU binaries")
	fmt.Println("and registers them with binfmt_misc so non-native binaries run transparently.")
	fmt.Println()
	fmt.Printf("  %s$ docker run --privileged --rm tonistiigi/binfmt --install all%s\n", sCyan, sReset)
	fmt.Println()

	if !promptYesNo("Install QEMU now?") {
		fmt.Println(sYellow + "⚠  Skipped QEMU installation." + sReset)
		return nil
	}

	fmt.Println(sCyan + "⠋ Installing QEMU..." + sReset)
	if err := runLive("docker", "run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all"); err != nil {
		return fmt.Errorf("QEMU install failed: %w", err)
	}
	fmt.Println(sGreen + "✓ QEMU installed" + sReset)

	// Verify
	fmt.Println()
	fmt.Println(sDim + "Registered binfmt entries:" + sReset)
	_ = runLive("sh", "-c", "ls /proc/sys/fs/binfmt_misc/qemu-* 2>/dev/null | sed 's|.*/||' | tr '\\n' ' '")
	fmt.Println()

	// Create a QEMU-backed kforge builder
	fmt.Println()
	printSection("Step: Create kforge builder (QEMU-backed)")
	builderName := promptInput("Builder name:", "kforge-qemu")

	fmt.Printf(sCyan+"⠋ Creating builder %q..."+sReset+"\n", builderName)
	if err := runLive("docker", "buildx", "create",
		"--name", builderName,
		"--driver", "docker-container",
		"--bootstrap",
		"--use",
	); err != nil {
		// Builder may already exist — try setting it as active
		fmt.Println(sYellow + "⚠  Builder may already exist, setting as active..." + sReset)
		_ = runLive("docker", "buildx", "use", builderName)
	}

	// Register in kforge store too
	_ = runSilent("kforge", "builder", "create", "--name", builderName, "--driver", "docker-container")
	_ = runSilent("kforge", "builder", "use", builderName)

	fmt.Println(sGreen + "✓ Builder created" + sReset)
	printVerify(builderName)
	return nil
}

// ─── MULTI-NODE SETUP ────────────────────────────────────────────────────────

func setupMultiNode() error {
	fmt.Println()
	printSection("Step: Multi-node builder setup")
	fmt.Println("A multi-node builder connects native machines for each platform.")
	fmt.Println("Each node must be a Docker context (run `docker context ls` to see available contexts).")
	fmt.Println()

	// Show existing contexts
	fmt.Println(sDim + "Your Docker contexts:" + sReset)
	_ = runLive("docker", "context", "ls")
	fmt.Println()

	builderName := promptInput("Builder name:", "kforge-multinode")

	// First node
	fmt.Println()
	fmt.Println(sBold + "Node 1 (primary)" + sReset)
	node1Context := promptInput("Docker context name for node 1 (e.g. node-amd64):", "")
	node1Platform := promptInput("Platform for node 1 (e.g. linux/amd64):", "linux/amd64")

	if node1Context == "" {
		fmt.Println(sRed + "✗ Context name is required." + sReset)
		return nil
	}

	// Create builder with first node
	fmt.Println()
	fmt.Printf(sCyan+"⠋ Creating builder %q with node 1 (%s)..."+sReset+"\n", builderName, node1Context)
	cmd1 := []string{
		"buildx", "create",
		"--name", builderName,
		"--node", builderName + "-" + node1Platform[strings.LastIndex(node1Platform, "/")+1:],
		"--platform", node1Platform,
		"--driver", "docker-container",
		"--bootstrap",
		"--use",
		node1Context,
	}
	if err := runLive("docker", cmd1...); err != nil {
		fmt.Printf(sYellow+"⚠  Could not create builder with context %q (is it reachable?)\n"+sReset, node1Context)
		fmt.Println(sDim + "Continuing anyway..." + sReset)
	} else {
		fmt.Println(sGreen + "✓ Node 1 added" + sReset)
	}

	// Additional nodes
	nodeNum := 2
	for {
		fmt.Println()
		if !promptYesNo(fmt.Sprintf("Add node %d?", nodeNum)) {
			break
		}

		fmt.Printf(sBold+"Node %d"+sReset+"\n", nodeNum)
		nodeContext := promptInput(fmt.Sprintf("Docker context name for node %d:", nodeNum), "")
		nodePlatform := promptInput(fmt.Sprintf("Platform for node %d (e.g. linux/arm64):", nodeNum), "linux/arm64")

		if nodeContext == "" {
			fmt.Println(sRed + "✗ Skipping — context name is required." + sReset)
			nodeNum++
			continue
		}

		fmt.Printf(sCyan+"⠋ Appending node %d (%s)..."+sReset+"\n", nodeNum, nodeContext)
		cmdN := []string{
			"buildx", "create",
			"--append",
			"--name", builderName,
			"--node", builderName + "-" + nodePlatform[strings.LastIndex(nodePlatform, "/")+1:],
			"--platform", nodePlatform,
			nodeContext,
		}
		if err := runLive("docker", cmdN...); err != nil {
			fmt.Printf(sYellow+"⚠  Could not append context %q\n"+sReset, nodeContext)
		} else {
			fmt.Printf(sGreen+"✓ Node %d added (%s)\n"+sReset, nodeNum, nodePlatform)
		}
		nodeNum++
	}

	// Register in kforge store
	_ = runSilent("kforge", "builder", "create", "--name", builderName, "--driver", "docker-container")
	_ = runSilent("kforge", "builder", "use", builderName)

	printVerify(builderName)
	return nil
}

// ─── HELPERS ─────────────────────────────────────────────────────────────────

type menuItem struct {
	Key   string
	Label string
	Desc  string
}

func printBanner() {
	lines := []string{
		"  ██╗  ██╗███████╗ ██████╗ ██████╗  ██████╗ ███████╗",
		"  ██║ ██╔╝██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝",
		"  █████╔╝ █████╗  ██║   ██║██████╔╝██║  ███╗█████╗  ",
		"  ██╔═██╗ ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  ",
		"  ██║  ██╗██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗",
		"  ╚═╝  ╚═╝╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝",
	}
	fmt.Println()
	for _, l := range lines {
		fmt.Println(sCyan + l + sReset)
	}
	fmt.Println(sDim + "  v0.1.0 · KhmerStack · Ing Muyleang" + sReset)
	fmt.Println()
}

func printSection(title string) {
	width := 52
	bar := strings.Repeat("─", width)
	fmt.Printf("%s%s%s\n", sCyan, bar, sReset)
	fmt.Printf(" %s%s%s\n", sBold, title, sReset)
	fmt.Printf("%s%s%s\n", sCyan, bar, sReset)
}

func printVerify(builderName string) {
	fmt.Println()
	printSection("Verify your setup")
	fmt.Printf("  %s$ docker buildx inspect %s%s\n", sCyan, builderName, sReset)
	fmt.Printf("  %s$ kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/myapp:latest .%s\n", sCyan, sReset)
	fmt.Printf("  %s$ docker kforge build --platform linux/amd64,linux/arm64 --push -t muyleangin/myapp:latest .%s\n", sCyan, sReset)
	fmt.Println()
	fmt.Println(sGreen + sBold + "✦ Setup complete!" + sReset)
}

func promptMenu(title string, items []menuItem) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println(sBold + title + sReset)
		for _, item := range items {
			fmt.Printf("  %s%s)%s %-28s %s%s%s\n",
				sCyan, item.Key, sReset,
				sBold+item.Label+sReset,
				sDim, item.Desc, sReset)
		}
		fmt.Printf("\n%sYour choice: %s", sYellow, sReset)
		line, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(line)
		for _, item := range items {
			if choice == item.Key {
				return choice
			}
		}
		fmt.Println(sRed + "Invalid choice. Try again." + sReset)
		fmt.Println()
	}
}

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s%s%s %s[y/n]%s: ", sBold, question, sReset, sCyan, sReset)
		line, _ := reader.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans == "y" || ans == "yes" {
			return true
		}
		if ans == "n" || ans == "no" {
			return false
		}
		fmt.Println(sRed + "Please enter y or n." + sReset)
	}
}

func promptInput(question, defaultVal string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultVal != "" {
		fmt.Printf("%s %s[default: %s]%s: ", question, sDim, defaultVal, sReset)
	} else {
		fmt.Printf("%s: ", question)
	}
	line, _ := reader.ReadString('\n')
	val := strings.TrimSpace(line)
	if val == "" {
		return defaultVal
	}
	return val
}

// runLive runs a command and streams stdout/stderr to the terminal.
func runLive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runSilent runs a command and discards all output.
func runSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
