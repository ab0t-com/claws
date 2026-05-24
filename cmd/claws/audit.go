package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// cmdAudit runs the security audit. Prefers the bundled script; falls back to built-in checks.
func cmdAudit(args []string) error {
	paths := resolvePaths()

	// Try to find the audit script
	scriptPaths := []string{}
	exe, _ := os.Executable()
	if exe != "" {
		scriptPaths = append(scriptPaths, filepath.Join(filepath.Dir(exe), "scripts", "security-audit.sh"))
	}
	cwd, _ := os.Getwd()
	if cwd != "" {
		scriptPaths = append(scriptPaths, filepath.Join(cwd, "scripts", "security-audit.sh"))
	}

	for _, sp := range scriptPaths {
		if _, err := os.Stat(sp); err == nil {
			cmd := exec.Command("bash", sp, paths.Root)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	// Fallback: run policy validate + doctor as a minimal audit
	fmt.Println("security-audit.sh not found — running built-in checks")
	fmt.Println()
	cmdDoctor(nil)
	fmt.Println()
	if policyExists(paths) {
		cmdPolicyValidate(nil)
	} else {
		warn("No admin policy configured. Run: claws policy init")
	}
	return nil
}
