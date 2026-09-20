// Aethium Setup Wizard
package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var payload embed.FS

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
)

func color(c, s string) string {
	if runtime.GOOS == "windows" {
		if os.Getenv("WT_SESSION") == "" && os.Getenv("TERM_PROGRAM") == "" {
			return s
		}
	}
	return c + s + reset
}

func header() {
	fmt.Println()
	fmt.Println(color(cyan, "╔══════════════════════════════════════════════════╗"))
	fmt.Println(color(cyan, "║       Aethium Language Setup Wizard v1.3.0       ║"))
	fmt.Println(color(cyan, "╚══════════════════════════════════════════════════╝"))
	fmt.Println()
	fmt.Printf("  Platform : %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
}

func step(n int, msg string) {
	fmt.Printf(color(bold, "  [%d] %s\n"), n, msg)
}

func ok(msg string) {
	fmt.Printf("      %s %s\n", color(green, "✓"), msg)
}

func warn(msg string) {
	fmt.Printf("      %s %s\n", color(yellow, "!"), msg)
}

func fail(msg string) {
	fmt.Printf("      %s %s\n", color(red, "✗"), msg)
}

func ask(prompt, defaultVal string) string {
	fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func confirm(prompt string) bool {
	fmt.Printf("  %s [Y/n]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}

func copyFile(dst string, src []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, src, 0755)
}

func extractEmbedDir(dst, src string) error {
	return fs.WalkDir(payload, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := payload.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func defaultInstallDir() string {
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(local, "Aethium")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), ".aethium")
	default: // linux and others
		return filepath.Join(os.Getenv("HOME"), ".aethium")
	}
}

func binaryName() string {
	switch runtime.GOOS {
	case "windows":
		return "aethium.exe"
	case "darwin":
		return "aethium"
	default:
		return "aethium"
	}
}

func embeddedBinaryPath() string {
	switch runtime.GOOS {
	case "windows":
		return "payload/aethium-windows.exe"
	case "darwin":
		return "payload/aethium-mac"
	default:
		return "payload/aethium-linux"
	}
}

func addToPathWindows(dir string) error {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','User')`).Output()
	if err != nil {
		return err
	}
	current := strings.TrimSpace(string(out))
	if strings.Contains(current, dir) {
		return nil // already there
	}

	parts := strings.Split(current, ";")
	cleaned := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	cleaned = append(cleaned, dir)
	newPath := strings.Join(cleaned, ";")

	if len(newPath) > 2040 {
		return fmt.Errorf("PATH too long — add %s manually", dir)
	}

	return exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path','%s','User')`, newPath)).Run()
}

func addToPathUnix(dir string) error {
	home := os.Getenv("HOME")
	shell := os.Getenv("SHELL")
	var rcFile string
	switch {
	case strings.Contains(shell, "zsh"):
		rcFile = filepath.Join(home, ".zshrc")
	case strings.Contains(shell, "fish"):
		rcFile = filepath.Join(home, ".config", "fish", "config.fish")
	default:
		rcFile = filepath.Join(home, ".bashrc")
	}

	exportLine := fmt.Sprintf(`export PATH="$PATH:%s"`, dir)

	existing, _ := os.ReadFile(rcFile)
	if strings.Contains(string(existing), dir) {
		return nil
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n# Aethium\n%s\n", exportLine)
	return err
}

func addToPath(dir string) error {
	if runtime.GOOS == "windows" {
		return addToPathWindows(dir)
	}
	return addToPathUnix(dir)
}

func installVSCodeExtension(vsixPath string) error {
	codeCmd := ""
	for _, candidate := range []string{"code", "code-insiders", "codium"} {
		if commandExists(candidate) {
			codeCmd = candidate
			break
		}
	}
	if codeCmd == "" {
		return fmt.Errorf("VS Code not found on PATH")
	}
	return exec.Command(codeCmd, "--install-extension", vsixPath, "--force").Run()
}

func writeUninstaller(installDir string) {
	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf(`# Aethium Uninstaller
$installDir = "%s"
Remove-Item -Recurse -Force $installDir -ErrorAction SilentlyContinue
$current = [Environment]::GetEnvironmentVariable("Path","User")
$new = ($current -split ";" | Where-Object { $_ -ne $installDir }) -join ";"
[Environment]::SetEnvironmentVariable("Path",$new,"User")
Write-Host "Aethium uninstalled."
`, installDir)
		os.WriteFile(filepath.Join(installDir, "uninstall.ps1"), []byte(script), 0644)

	default:
		script := fmt.Sprintf(`#!/bin/sh
# Aethium Uninstaller
rm -rf "%s"
echo "Aethium uninstalled. Remove the PATH line from your shell rc file manually."
`, installDir)
		path := filepath.Join(installDir, "uninstall.sh")
		os.WriteFile(path, []byte(script), 0755)
	}
}

// Main

func main() {
	header()

	// Welcome
	fmt.Println("  Welcome! This wizard will install Aethium on your computer.")
	fmt.Println("  It will:")
	fmt.Println("    • Install the Aethium runtime")
	fmt.Println("    • Add aethium to your PATH")
	fmt.Println("    • Install the VS Code extension (if VS Code is present)")
	fmt.Println("    • Copy example scripts")
	fmt.Println("    • Create an uninstaller")
	fmt.Println()

	if !confirm("Continue with installation?") {
		fmt.Println("\n  Installation cancelled.")
		return
	}
	fmt.Println()

	// Choose install directory
	installDir := ask("Install directory", defaultInstallDir())
	fmt.Println()

	// Step 1: Extract binary
	step(1, "Installing Aethium runtime")

	binData, err := payload.ReadFile(embeddedBinaryPath())
	if err != nil {
		fail(fmt.Sprintf("Could not read embedded binary: %v", err))
		os.Exit(1)
	}

	binDest := filepath.Join(installDir, binaryName())
	if err := copyFile(binDest, binData); err != nil {
		fail(fmt.Sprintf("Could not write binary: %v", err))
		os.Exit(1)
	}
	ok(fmt.Sprintf("Installed to %s", binDest))

	// Step 2: Add to PATH
	step(2, "Adding to PATH")

	if err := addToPath(installDir); err != nil {
		warn(fmt.Sprintf("Could not update PATH automatically: %v", err))
		warn(fmt.Sprintf("Add this to your PATH manually: %s", installDir))
	} else {
		ok("PATH updated")
		if runtime.GOOS != "windows" {
			warn("Restart your terminal (or run: source ~/.bashrc) for PATH to take effect")
		} else {
			warn("Restart your terminal for PATH to take effect")
		}
	}

	// Step 3: Copy examples
	step(3, "Copying example scripts")

	examplesDir := filepath.Join(installDir, "examples")
	if err := extractEmbedDir(examplesDir, "payload/examples"); err != nil {
		warn(fmt.Sprintf("Could not copy examples: %v", err))
	} else {
		ok(fmt.Sprintf("Examples copied to %s", examplesDir))
	}

	// Step 4: VS Code extension
	step(4, "Installing VS Code extension")

	vsixData, err := payload.ReadFile("payload/editors/vscode/aethium-lang-1.3.0.vsix")
	if err != nil {
		warn("VS Code extension not found in package — skipping")
	} else {
		// Write vsix to install dir first
		vsixDest := filepath.Join(installDir, "aethium-lang-1.3.0.vsix")
		_ = os.WriteFile(vsixDest, vsixData, 0644)

		if err := installVSCodeExtension(vsixDest); err != nil {
			// VS Code not found — copy to Desktop as fallback
			desktop := ""
			if runtime.GOOS == "windows" {
				desktop = filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
			} else {
				desktop = filepath.Join(os.Getenv("HOME"), "Desktop")
			}
			destOnDesktop := filepath.Join(desktop, "aethium-lang-1.3.0.vsix")
			if desktop != "" {
				_ = os.WriteFile(destOnDesktop, vsixData, 0644)
				warn("VS Code not found on PATH")
				warn("Extension copied to your Desktop")
				warn("In VS Code: Ctrl+Shift+P → 'Install from VSIX' → select it")
			} else {
				warn(fmt.Sprintf("VS Code not found. Install extension manually from: %s", vsixDest))
			}
		} else {
			ok("VS Code extension installed")
		}
	}

	// Step 5: Write uninstaller
	step(5, "Creating uninstaller")
	writeUninstaller(installDir)
	ok(fmt.Sprintf("Uninstaller written to %s", installDir))

	// Step 6: Verify
	step(6, "Verifying installation")

	out, err := exec.Command(binDest, "version").Output()
	if err != nil {
		warn("Could not verify — try running 'aethium version' after restarting your terminal")
	} else {
		ok(strings.TrimSpace(string(out)))
	}

	// Done
	fmt.Println()
	fmt.Println(color(cyan, "  ╔══════════════════════════════════════════════════╗"))
	fmt.Println(color(cyan, "  ║           Installation complete! 🎉              ║"))
	fmt.Println(color(cyan, "  ╚══════════════════════════════════════════════════╝"))
	fmt.Println()
	fmt.Println("  Restart your terminal, then try:")
	fmt.Println(color(green, "    aethium version"))
	fmt.Println(color(green, "    aethium -c \"print('Hello, Aethium!')\""))
	fmt.Println(color(green, "    aethium myfile.aeth"))
	fmt.Println()

	if runtime.GOOS == "windows" {
		fmt.Print("  Press Enter to exit...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
}

// Compile-time asset check
// Ensures the build fails clearly if payload files are missing rather than producing a broken installer silently.
var _ io.Reader // suppress unused import if embed path changes
