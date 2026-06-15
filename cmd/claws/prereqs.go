package main

import (
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// prereqs — friendly errors when claws's external dependencies are missing
// ---------------------------------------------------------------------------
//
// Background: a fresh box without docker would hit a docker-using command
// (create, start, apply, etc.) and get an opaque "exec: \"docker\": executable
// file not found in $PATH" error mid-flow. That tells a non-technical user
// exactly nothing about what to do.
//
// This file centralises the prereq guard. Commands that need docker call
// requireDocker() at the start; the dispatch loop in main.go also calls it
// for the well-known docker-using commands as a safety net.
//
// The error messages point at the bash installer scripts under
// scripts/prereqs/ — both as a curl one-liner (works on a fresh box) and
// the manual provider docs (for users who'd rather not curl|bash).

const prereqsRepoBase = "https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs"

// commandsNotNeedingDocker lists the commands that can safely run without
// docker on PATH or the daemon reachable. Everything else gets the
// requireDocker preflight.
//
// Conservative on purpose — we'd rather force a docker check on a command
// that doesn't strictly need it than skip the check on a command that does.
var commandsNotNeedingDocker = map[string]bool{
	"":             true, // no-args invocation prints the welcome screen
	"version":      true,
	"--version":    true,
	"-v":           true,
	"help":         true,
	"--help":       true,
	"-h":           true,
	"update":       true,
	"self-update":  true,
	"doctor":       true, // doctor diagnoses; it shouldn't refuse to run because the thing it diagnoses is broken
	"init":         true, // first-time host setup (writes docker-compose.yml)
	"paste-secret": true, // ephemeral HTTP listener; no docker needed
}

// requireDocker checks that docker is installed and the daemon is reachable.
// Returns a verbose, actionable error if either check fails — the kind of
// error a non-technical user can act on without googling.
//
// Order matters: we check for the docker binary FIRST (cheap, no syscall
// past PATH lookup), THEN check the daemon (a docker info call). The two
// failure modes have different remediations, so we surface them separately.
func requireDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return missingPrereqError("docker", "manages the agent containers (claws creates one container per agent)")
	}

	// Compose plugin — modern docker installs ship with `docker compose`
	// as a subcommand. Detect by running it; failure here means an older
	// docker-only install without the v2 plugin.
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return errorf(`docker is installed but the 'docker compose' plugin is missing.

  claws uses 'docker compose' (v2 plugin form, not legacy 'docker-compose').

  Fix:
    %s
    curl -fsSL %s/install-docker.sh | bash
    %s

  Or install the compose plugin per Docker's docs:
    https://docs.docker.com/compose/install/linux/`,
			"\033[0;90m", prereqsRepoBase, "\033[0m")
	}

	// Daemon reachable? `docker info` is the canonical "is the daemon up
	// and am I authorised to talk to it" check.
	if err := exec.Command("docker", "info").Run(); err != nil {
		return errorf(`docker is installed but the daemon is not reachable.

  This usually means one of:

    1. The daemon isn't running. Start it:
         %ssudo systemctl start docker%s     (Linux)
         %sopen -a Docker%s                  (macOS Docker Desktop)

    2. Your user isn't in the 'docker' group. Add yourself:
         %ssudo usermod -aG docker $USER%s
       Then log out and back in (or run 'newgrp docker').

    3. Some other docker config issue. Investigate:
         %ssudo journalctl -u docker -n 50%s

  After fixing, re-run your claws command.`,
			"\033[0;90m", "\033[0m", "\033[0;90m", "\033[0m",
			"\033[0;90m", "\033[0m", "\033[0;90m", "\033[0m")
	}

	return nil
}

// commandNeedsDocker returns true if the given command requires docker
// to function.
func commandNeedsDocker(cmd string) bool {
	if commandsNotNeedingDocker[cmd] {
		return false
	}
	return true
}

// missingPrereqError formats a friendly error for a missing CLI tool,
// pointing at both the universal installer and the per-tool installer.
func missingPrereqError(tool, role string) error {
	dim := "\033[0;90m"
	nc := "\033[0m"
	bold := "\033[1m"

	var perToolURL string
	switch tool {
	case "docker":
		perToolURL = prereqsRepoBase + "/install-docker.sh"
	case "git":
		perToolURL = prereqsRepoBase + "/install-git.sh"
	case "curl":
		perToolURL = prereqsRepoBase + "/install-curl.sh"
	default:
		perToolURL = prereqsRepoBase + "/install-all.sh"
	}

	return errorf(`%s is not installed (%s).

  %sQuick install — auto-detects your OS%s:
    %scurl -fsSL %s | bash%s

  %sOr install everything claws needs in one shot%s:
    %scurl -fsSL %s/install-all.sh | bash%s

  Verify after install:
    %sclaws doctor%s`,
		tool, role,
		bold, nc, dim, perToolURL, nc,
		bold, nc, dim, prereqsRepoBase, nc,
		dim, nc)
}

// validatePrereqsForCommand is the dispatch-level entry point. main.go
// calls this with the parsed command name before running it. If the
// command needs docker, we verify; otherwise we no-op.
func validatePrereqsForCommand(cmd string) error {
	// Strip leading colour codes if any caller pre-formatted the cmd.
	cmd = strings.TrimSpace(cmd)

	if !commandNeedsDocker(cmd) {
		return nil
	}
	return requireDocker()
}
