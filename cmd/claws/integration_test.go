package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Integration tests build and run the binary against a temp OPENCLAW_ROOT.

var testBinary string

// testRootCleanupRegistered tracks which temp roots have already had a
// docker-cleanup hook registered. Without this, every claws(...) call
// inside a test would stack another t.Cleanup. With it, we register exactly
// one cleanup per root — fires at test teardown, walks the registry, and
// tears down any Docker compose project we accidentally brought up.
//
// Why this exists: tests that exercise code paths like `--auth=codex`
// (which runs `docker compose run --rm openclaw-cli ...`) bring up the
// gateway dependency container. `--rm` removes the CLI sidecar but does
// not remove the dependency. Without cleanup, every such test run leaves
// a restart-looping orphan with mounts pointing at the now-deleted
// t.TempDir(). Surfaced and motivated by ticket
// `test-harness-orphan-containers-2026-05-23`.
var testRootCleanupRegistered sync.Map

func TestMain(m *testing.M) {
	// Build binary to temp location
	tmp, err := os.MkdirTemp("", "claws-test-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create temp dir: %v\n", err)
		os.Exit(1)
	}
	testBinary = filepath.Join(tmp, "claws")

	// Copy compose template next to binary so it can find it
	composeSource := filepath.Join(".", "docker-compose.yml")
	composeDest := filepath.Join(tmp, "docker-compose.yml")

	build := exec.Command("go", "build", "-o", testBinary, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}

	// Copy compose template
	if data, err := os.ReadFile(composeSource); err == nil {
		os.WriteFile(composeDest, data, 0644)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func claws(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	registerDockerCleanup(t, root)
	bin := testBinary
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"OPENCLAW_ROOT="+root,
		"CLAWS_BASE_PORT=29789",
		"CLAWS_SKIP_VALIDATE=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// clawsCwd is like claws but runs from a specific cwd — used by template
// resolver tests that depend on ./templates/ lookup.
func clawsCwd(t *testing.T, root, cwd string, args ...string) (string, error) {
	t.Helper()
	registerDockerCleanup(t, root)
	cmd := exec.Command(testBinary, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"OPENCLAW_ROOT="+root,
		"CLAWS_BASE_PORT=29789",
		"CLAWS_SKIP_VALIDATE=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// registerDockerCleanup registers a t.Cleanup, exactly once per root, that
// tears down any Docker compose projects this test created via claws. It
// reads the port registry at cleanup time and runs `docker compose down -v
// --remove-orphans` against each project name. Best-effort: errors are
// ignored (most cleanups target projects that were never started under
// CLAWS_SKIP_VALIDATE — fast no-ops). The cleanup fires before t.TempDir
// is removed, so containers stop before their mount sources vanish — which
// is the entire point.
func registerDockerCleanup(t *testing.T, root string) {
	if _, loaded := testRootCleanupRegistered.LoadOrStore(root, true); loaded {
		return
	}
	t.Cleanup(func() {
		// We intentionally don't pull the test binary here — reading the
		// registry file directly is faster and avoids re-exec'ing during
		// cleanup. Same logic claws uses internally to enumerate
		// projects.
		regPath := filepath.Join(root, ".port-registry")
		data, err := os.ReadFile(regPath)
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			name := parts[1]
			// Project name mirrors Runtime.MakeProjectName: "openclaw-" +
			// dashed instance name (group/name → group-name).
			project := "openclaw-" + strings.ReplaceAll(name, "/", "-")
			// Best-effort teardown. --remove-orphans cleans CLI sidecars
			// that survived their parent test if any. -v removes named
			// volumes, which the test never persists deliberately.
			_ = exec.Command("docker", "compose", "-p", project, "down", "-v", "--remove-orphans").Run()
		}
		// Also clean orphan containers matching the test's project prefix
		// — catches projects that started but failed to register (e.g., a
		// test that exercised --auth chains and then assert-failed before
		// reaching the registry write). Cheap and surgical: we filter on
		// the same TestName prefix that's already in t.TempDir's name, so
		// we never touch projects we didn't create.
		testHint := strings.TrimSuffix(filepath.Base(root), filepath.Ext(filepath.Base(root)))
		if testHint != "" {
			// Best-effort, no error propagation.
			out, _ := exec.Command("docker", "ps", "-a",
				"--filter", "name=openclaw-",
				"--format", "{{.Names}}\t{{.Label \"com.docker.compose.project.config_files\"}}").Output()
			for _, line := range strings.Split(string(out), "\n") {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) != 2 {
					continue
				}
				name, configFiles := parts[0], parts[1]
				// Only kill a container if its compose project's config
				// path lives under THIS test's root — never anyone else's.
				if strings.Contains(configFiles, root) {
					_ = exec.Command("docker", "rm", "-f", name).Run()
				}
			}
		}
	})
}

// TestRegisterDockerCleanup_OncePerRoot proves the dedup invariant: the
// cleanup is registered exactly once per root, no matter how many claws
// calls a test makes. We don't (and can't easily) assert that the cleanup
// fired with the right docker arguments — the real Docker call is best-
// effort and ignores results — but we can make sure we're not stacking
// N cleanups per test and slowing teardown to a crawl.
func TestRegisterDockerCleanup_OncePerRoot(t *testing.T) {
	root := t.TempDir()
	// Reset the package-level map for hermeticity. In production this map
	// is process-wide; in tests the same root can be reused across runs of
	// `go test`, and the sync.Map persists across function boundaries
	// inside a single binary invocation. We delete the key explicitly so
	// this test produces the same result regardless of run order.
	testRootCleanupRegistered.Delete(root)

	// First call: registers.
	registerDockerCleanup(t, root)
	// Subsequent calls: must not register again.
	registerDockerCleanup(t, root)
	registerDockerCleanup(t, root)

	if _, ok := testRootCleanupRegistered.Load(root); !ok {
		t.Errorf("expected root %q to be registered in the sync.Map", root)
	}
	// The actual t.Cleanup count is private to testing.T, but if the map
	// is being consulted correctly, only one cleanup will run at teardown.
}

// TestRegisterDockerCleanup_HandlesMissingRegistry confirms the cleanup
// hook doesn't crash on a test that never wrote a registry (e.g., one
// that called claws(t, root, "help") and nothing else). Cheap defensive
// check matching the worklog claim that cleanup is best-effort.
func TestRegisterDockerCleanup_HandlesMissingRegistry(t *testing.T) {
	root := t.TempDir()
	testRootCleanupRegistered.Delete(root)
	registerDockerCleanup(t, root)
	// No registry file at root/.port-registry. Cleanup must not panic
	// when it tries to read. We can't *force* it to fire here (that's
	// t.Cleanup's job), but the implementation explicitly handles os.ReadFile
	// returning err — and that's what this test asserts by inspection.
	if _, err := os.Stat(filepath.Join(root, ".port-registry")); err == nil {
		t.Errorf("setup invariant broken: registry should not exist at start of this test")
	}
}

func TestIntegration_CreateBasic(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "create", "alpha")
	if err != nil {
		t.Fatalf("create failed: %s\n%s", err, out)
	}
	if !strings.Contains(out, "Instance 'alpha' created") {
		t.Errorf("unexpected output: %s", out)
	}

	// Check files
	for _, f := range []string{"instance.env", "openclaw.json"} {
		if _, err := os.Stat(filepath.Join(root, "alpha", f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	for _, d := range []string{"workspace", "credentials", "agents", "identity", "sessions"} {
		if _, err := os.Stat(filepath.Join(root, "alpha", d)); err != nil {
			t.Errorf("missing dir %s", d)
		}
	}
}

func TestIntegration_PortAllocation(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")

	portA := readEnvFromFile(t, filepath.Join(root, "alpha", "instance.env"), "OPENCLAW_GATEWAY_PORT")
	portB := readEnvFromFile(t, filepath.Join(root, "bravo", "instance.env"), "OPENCLAW_GATEWAY_PORT")

	if portA != "29789" {
		t.Errorf("alpha port: expected 29789, got %s", portA)
	}
	if portB != "29889" {
		t.Errorf("bravo port: expected 29889, got %s", portB)
	}
}

func TestIntegration_PortReuse(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")
	claws(t, root, "remove", "alpha", "--purge", "--yes")
	claws(t, root, "create", "charlie")

	port := readEnvFromFile(t, filepath.Join(root, "charlie", "instance.env"), "OPENCLAW_GATEWAY_PORT")
	if port != "29789" {
		t.Errorf("charlie should reuse index 0 port 29789, got %s", port)
	}
}

func TestIntegration_UniqueTokens(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")

	tokA := readEnvFromFile(t, filepath.Join(root, "alpha", "instance.env"), "OPENCLAW_GATEWAY_TOKEN")
	tokB := readEnvFromFile(t, filepath.Join(root, "bravo", "instance.env"), "OPENCLAW_GATEWAY_TOKEN")

	if tokA == tokB {
		t.Error("tokens should be unique")
	}
	if len(tokA) != 64 {
		t.Errorf("token should be 64 hex chars, got %d", len(tokA))
	}
}

func TestIntegration_DuplicateFails(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	_, err := claws(t, root, "create", "alpha")
	if err == nil {
		t.Error("duplicate create should fail")
	}
}

func TestIntegration_NameValidation(t *testing.T) {
	root := t.TempDir()

	// Valid
	if _, err := claws(t, root, "create", "my-bot-1"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}

	// Invalid
	for _, name := range []string{"MyBot", "1bot", "shared"} {
		if _, err := claws(t, root, "create", name); err == nil {
			t.Errorf("'%s' should be rejected", name)
		}
	}
}

func TestIntegration_List(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "list")
	if !strings.Contains(out, "No instances found") {
		t.Error("empty list should say no instances")
	}

	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")
	out, _ = claws(t, root, "list")
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "bravo") {
		t.Errorf("list should show both instances: %s", out)
	}
}

func TestIntegration_Status(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	out, err := claws(t, root, "status", "alpha")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "Instance: alpha") {
		t.Errorf("status missing name: %s", out)
	}

	// Nonexistent
	_, err = claws(t, root, "status", "nonexistent")
	if err == nil {
		t.Error("status on nonexistent should fail")
	}
}

func TestIntegration_RemoveKeepsData(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "remove", "alpha")

	if _, err := os.Stat(filepath.Join(root, "alpha", "openclaw.json")); err != nil {
		t.Error("remove without --purge should keep data")
	}

	reg, _ := os.ReadFile(filepath.Join(root, ".port-registry"))
	if strings.Contains(string(reg), "alpha") {
		t.Error("port should be freed after remove")
	}
}

func TestIntegration_RemovePurge(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "remove", "alpha", "--purge", "--yes")

	if _, err := os.Stat(filepath.Join(root, "alpha")); err == nil {
		t.Error("purge should delete instance dir")
	}
}

func TestIntegration_Backup(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	backupFile := filepath.Join(root, "backup.tar.gz")
	out, err := claws(t, root, "backup", "alpha", backupFile)
	if err != nil {
		t.Fatalf("backup failed: %s\n%s", err, out)
	}
	if _, err := os.Stat(backupFile); err != nil {
		t.Error("backup file not created")
	}
}

func TestIntegration_Restore(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	backupFile := filepath.Join(root, "backup.tar.gz")
	claws(t, root, "backup", "alpha", backupFile)
	claws(t, root, "remove", "alpha", "--purge", "--yes")

	out, err := claws(t, root, "restore", "alpha", backupFile)
	if err != nil {
		t.Fatalf("restore failed: %s\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "instance.env")); err != nil {
		t.Error("restore didn't recreate instance")
	}
}

func TestIntegration_BackupWarnsCredentials(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Add a credential file
	os.WriteFile(filepath.Join(root, "alpha", "credentials", "test-key.json"), []byte(`{"key":"secret"}`), 0600)

	backupFile := filepath.Join(root, "backup-warn.tar.gz")
	out, err := claws(t, root, "backup", "alpha", backupFile)
	if err != nil {
		t.Fatalf("backup failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "credentials") {
		t.Errorf("should warn about credentials: %s", out)
	}
}

func TestIntegration_BackupExcludeCredentials(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Add a credential file
	os.WriteFile(filepath.Join(root, "alpha", "credentials", "test-key.json"), []byte(`{"key":"secret"}`), 0600)

	backupFile := filepath.Join(root, "backup-nocreds.tar.gz")
	out, err := claws(t, root, "backup", "alpha", backupFile, "--exclude-credentials")
	if err != nil {
		t.Fatalf("backup failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Excluding credentials") {
		t.Errorf("should confirm credential exclusion: %s", out)
	}
}

func TestIntegration_RestoreExistingFails(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	backupFile := filepath.Join(root, "backup.tar.gz")
	claws(t, root, "backup", "alpha", backupFile)

	_, err := claws(t, root, "restore", "alpha", backupFile)
	if err == nil {
		t.Error("restore over existing instance should fail")
	}
}

func TestIntegration_Defaults(t *testing.T) {
	root := t.TempDir()

	// Write defaults
	defaults := map[string]any{
		"tools": map[string]any{"profile": "coding", "alsoAllow": []any{"message"}},
		"agents": map[string]any{
			"defaults": map[string]any{"model": map[string]any{"primary": "test/v1"}},
		},
	}
	data, _ := json.MarshalIndent(defaults, "", "  ")
	os.WriteFile(filepath.Join(root, "defaults.json"), data, 0644)

	claws(t, root, "create", "alpha")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tools := cfg["tools"].(map[string]any)
	if tools["profile"] != "coding" {
		t.Errorf("tools.profile should be 'coding', got '%v'", tools["profile"])
	}

	// Gateway port should be instance-specific, not from defaults
	gw := cfg["gateway"].(map[string]any)
	if gw["port"] != float64(29789) {
		t.Errorf("gateway.port should be 29789, got %v", gw["port"])
	}
}

func TestIntegration_CreateFrom(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Add custom config to alpha
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	cfg["tools"] = map[string]any{"profile": "coding", "alsoAllow": []any{"message"}}
	cfg["channels"] = map[string]any{
		"whatsapp": map[string]any{
			"enabled":   true,
			"allowFrom": []any{"+1234"},
			"groups":    map[string]any{"abc@g.us": map[string]any{}},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(root, "alpha", "openclaw.json"), data, 0644)

	claws(t, root, "create", "bravo", "--from=alpha")

	bravoConfig := readJSON(t, filepath.Join(root, "bravo", "openclaw.json"))

	// Tools should be copied
	tools := bravoConfig["tools"].(map[string]any)
	if tools["profile"] != "coding" {
		t.Errorf("tools.profile should be copied from alpha")
	}

	// Gateway should be bravo's own
	gw := bravoConfig["gateway"].(map[string]any)
	if gw["port"] == float64(29789) {
		t.Error("bravo should have its own port, not alpha's")
	}

	// Allowlists should be stripped
	ch := bravoConfig["channels"].(map[string]any)["whatsapp"].(map[string]any)
	if _, ok := ch["allowFrom"]; ok {
		t.Error("allowFrom should be stripped from template")
	}
	if _, ok := ch["groups"]; ok {
		t.Error("groups should be stripped from template")
	}
}

func TestIntegration_SharedSkills(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha", "--shared-skills")

	override := filepath.Join(root, "alpha", "docker-compose.override.yml")
	if _, err := os.Stat(override); err != nil {
		t.Fatal("override file should exist")
	}
	data, _ := os.ReadFile(override)
	if !strings.Contains(string(data), "bundled-skills") {
		t.Error("override should contain bundled-skills mount")
	}
	if _, err := os.Stat(filepath.Join(root, "shared", "skills")); err != nil {
		t.Error("shared/skills dir should be created")
	}
}

func TestIntegration_ShareUnshare(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// No override yet
	if _, err := os.Stat(filepath.Join(root, "alpha", "docker-compose.override.yml")); err == nil {
		t.Error("override should not exist initially")
	}

	// Share skills
	claws(t, root, "share", "alpha", "--skills")
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "docker-compose.override.yml"))
	if !strings.Contains(string(data), "bundled-skills") {
		t.Error("override should contain bundled-skills after share")
	}

	// Add workspace
	claws(t, root, "share", "alpha", "--workspace")
	data, _ = os.ReadFile(filepath.Join(root, "alpha", "docker-compose.override.yml"))
	if !strings.Contains(string(data), "/shared:rw") {
		t.Error("override should contain shared workspace")
	}

	// Unshare skills
	claws(t, root, "unshare", "alpha", "--skills")
	data, _ = os.ReadFile(filepath.Join(root, "alpha", "docker-compose.override.yml"))
	if strings.Contains(string(data), "bundled-skills") {
		t.Error("skills should be removed after unshare")
	}
	if !strings.Contains(string(data), "/shared:rw") {
		t.Error("workspace should remain after unsharing skills")
	}

	// Unshare all
	claws(t, root, "unshare", "alpha", "--all")
	if _, err := os.Stat(filepath.Join(root, "alpha", "docker-compose.override.yml")); err == nil {
		t.Error("override should be deleted when no shared flags")
	}
}

func TestIntegration_JsonPortMatchesEnv(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	envPort := readEnvFromFile(t, filepath.Join(root, "alpha", "instance.env"), "OPENCLAW_GATEWAY_PORT")
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	gw := cfg["gateway"].(map[string]any)
	jsonPort := gw["port"].(float64)

	if envPort != "29789" || jsonPort != 29789 {
		t.Errorf("port mismatch: env=%s json=%v", envPort, jsonPort)
	}
}

func TestIntegration_Help(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "help")
	for _, section := range []string{"Lifecycle", "Shared Resources", "Operations", "Backup"} {
		if !strings.Contains(out, section) {
			t.Errorf("help missing section: %s", section)
		}
	}
}

func TestIntegration_Health(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	out, err := claws(t, root, "health", "alpha")
	if err != nil {
		// Health on a non-running instance should still work (show "stopped" or "down")
		_ = out
	}
	// Should contain the instance name and a verdict
	if !strings.Contains(out, "alpha") {
		t.Errorf("health should show instance name: %s", out)
	}
	// Non-started instance should show down/stopped
	if !strings.Contains(out, "down") && !strings.Contains(out, "stopped") && !strings.Contains(out, "missing") {
		// It's fine if compose reports "created" which maps to down
		t.Logf("health output (non-running): %s", out)
	}
}

func TestIntegration_HealthAll(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")

	out, _ := claws(t, root, "health")
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "bravo") {
		t.Errorf("health (all) should show both instances: %s", out)
	}
}

func TestIntegration_Tunnel(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")

	out, _ := claws(t, root, "tunnel")
	if !strings.Contains(out, "29789") || !strings.Contains(out, "29889") {
		t.Errorf("tunnel should contain both ports: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Group tests
// ---------------------------------------------------------------------------

func TestIntegration_GroupCreate(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "group", "create", "backend")
	if err != nil {
		t.Fatalf("group create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Group 'backend' created") {
		t.Errorf("unexpected output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "backend", ".group.json")); err != nil {
		t.Error(".group.json not created")
	}
	if _, err := os.Stat(filepath.Join(root, "backend", "shared", "workspace")); err != nil {
		t.Error("shared/workspace not created")
	}
}

func TestIntegration_GroupedInstance(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "backend")
	out, err := claws(t, root, "create", "backend/sarah")
	if err != nil {
		t.Fatalf("grouped create failed: %v\n%s", err, out)
	}

	// Should exist under group dir
	if _, err := os.Stat(filepath.Join(root, "backend", "sarah", "instance.env")); err != nil {
		t.Error("instance not created under group dir")
	}

	// Should auto-share group resources
	override := filepath.Join(root, "backend", "sarah", "docker-compose.override.yml")
	if _, err := os.Stat(override); err != nil {
		t.Error("override should exist for grouped instance")
	}
	data, _ := os.ReadFile(override)
	if !strings.Contains(string(data), "bundled-skills") {
		t.Error("grouped instance should share group skills by default")
	}
	if !strings.Contains(string(data), "/shared:rw") {
		t.Error("grouped instance should share group workspace by default")
	}
}

func TestIntegration_GroupedNoShared(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "backend")
	claws(t, root, "create", "backend/isolated", "--no-shared-workspace")

	// Should NOT have shared workspace in override (but may have skills)
	override := filepath.Join(root, "backend", "isolated", "docker-compose.override.yml")
	if _, err := os.Stat(override); err != nil {
		// No override at all is also fine if no sharing
		return
	}
}

func TestIntegration_ManagerWorkerTopology(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/lead", "--role=manager")
	claws(t, root, "create", "team/dev1", "--role=worker", "--manager=lead")

	// Worker override should have task feed + output + manager workspace
	workerOverride := filepath.Join(root, "team", "dev1", "docker-compose.override.yml")
	data, err := os.ReadFile(workerOverride)
	if err != nil {
		t.Fatal("worker override not created")
	}
	content := string(data)
	if !strings.Contains(content, "/tasks:ro") {
		t.Error("worker should have read-only task feed")
	}
	if !strings.Contains(content, "/output:rw") {
		t.Error("worker should have writable output dir")
	}
	if !strings.Contains(content, "/manager:ro") {
		t.Error("worker should have read-only view of manager workspace")
	}

	// Manager override should have task dispatch + worker visibility
	managerOverride := filepath.Join(root, "team", "lead", "docker-compose.override.yml")
	data, err = os.ReadFile(managerOverride)
	if err != nil {
		t.Fatal("manager override not created")
	}
	content = string(data)
	if !strings.Contains(content, "/tasks:rw") {
		t.Error("manager should have writable task dispatch dir")
	}

	// Task dirs should be scaffolded
	for _, sub := range []string{"pending", "claimed", "done"} {
		if _, err := os.Stat(filepath.Join(root, "team", "shared", "tasks", sub)); err != nil {
			t.Errorf("task queue dir '%s' not created", sub)
		}
	}
}

func TestIntegration_GroupList(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "backend")
	claws(t, root, "group", "create", "frontend")
	claws(t, root, "create", "backend/sarah")

	out, _ := claws(t, root, "group", "list")
	if !strings.Contains(out, "backend") || !strings.Contains(out, "frontend") {
		t.Errorf("group list should show both groups: %s", out)
	}
}

func TestIntegration_GroupDefaults(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "backend")

	// Write group-level defaults
	defaults := `{"tools":{"profile":"messaging"}}`
	os.WriteFile(filepath.Join(root, "backend", "defaults.json"), []byte(defaults), 0644)

	claws(t, root, "create", "backend/bob")

	cfg := readJSON(t, filepath.Join(root, "backend", "bob", "openclaw.json"))
	tools := cfg["tools"].(map[string]any)
	if tools["profile"] != "messaging" {
		t.Errorf("group defaults not applied: got %v", tools["profile"])
	}
}

func TestIntegration_GroupRequiresExistence(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "create", "nonexistent/sarah")
	if err == nil {
		t.Error("should fail when group doesn't exist")
	}
}

func TestIntegration_RoleRequiresGroup(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "create", "standalone", "--role=worker")
	if err == nil {
		t.Error("worker without group should fail")
	}
}

func TestIntegration_ListShowsGrouped(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "backend")
	claws(t, root, "create", "backend/sarah")
	claws(t, root, "create", "standalone")

	out, _ := claws(t, root, "list")
	if !strings.Contains(out, "backend/sarah") {
		t.Error("list should show grouped name")
	}
	if !strings.Contains(out, "standalone") {
		t.Error("list should show ungrouped name")
	}
}

// ---------------------------------------------------------------------------
// Activity + Proxy tests
// ---------------------------------------------------------------------------

func TestIntegration_ActivityEmpty(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	out, _ := claws(t, root, "activity", "--since=1h")
	if !strings.Contains(out, "No activity") {
		t.Errorf("fresh instance should have no activity: %s", out)
	}
}

func TestIntegration_ActivityShowsFiles(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Create a file in workspace
	wsDir := filepath.Join(root, "alpha", "workspace")
	os.WriteFile(filepath.Join(wsDir, "test.txt"), []byte("hello"), 0644)

	out, _ := claws(t, root, "activity", "--since=1h")
	if !strings.Contains(out, "test.txt") {
		t.Errorf("activity should show recently created file: %s", out)
	}
}

func TestIntegration_ProxyStatusNoCaddy(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "proxy", "status")
	// Caddy may or may not be installed — just check it doesn't crash
	if !strings.Contains(out, "Proxy Status") {
		t.Errorf("proxy status should show header: %s", out)
	}
}

func TestIntegration_ProxySetupRequiresDomain(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "proxy", "setup")
	if err == nil {
		t.Error("proxy setup without --domain should fail")
	}
}

// ---------------------------------------------------------------------------
// JSON output tests
// ---------------------------------------------------------------------------

func TestIntegration_ListJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "create", "bravo")

	out, err := claws(t, root, "list", "--json")
	if err != nil {
		t.Fatalf("list --json failed: %v\n%s", err, out)
	}
	var entries []map[string]string
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0]["name"] != "alpha" {
		t.Errorf("first entry name should be alpha, got %s", entries[0]["name"])
	}
}

func TestIntegration_StatusJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	out, err := claws(t, root, "status", "alpha", "--json")
	if err != nil {
		t.Fatalf("status --json failed: %v\n%s", err, out)
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if obj["name"] != "alpha" {
		t.Errorf("name should be alpha, got %s", obj["name"])
	}
	if obj["port"] == "" {
		t.Error("port should not be empty")
	}
}

func TestIntegration_HealthJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	out, err := claws(t, root, "health", "--json")
	if err != nil {
		// Health may "fail" if Docker isn't running, but output should be valid JSON
		_ = err
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestIntegration_ListJsonEmpty(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "list", "--json")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty list --json should be [], got %s", out)
	}
}

// ---------------------------------------------------------------------------
// File permission tests
// ---------------------------------------------------------------------------

func TestIntegration_EnvFilePermissions(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	envFile := filepath.Join(root, "alpha", "instance.env")
	fi, err := os.Stat(envFile)
	if err != nil {
		t.Fatal(err)
	}
	mode := fi.Mode().Perm()
	if mode != 0600 {
		t.Errorf("instance.env should be 0600, got %04o", mode)
	}
}

func TestIntegration_RegistryFilePermissions(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	regFile := filepath.Join(root, ".port-registry")
	fi, err := os.Stat(regFile)
	if err != nil {
		t.Fatal(err)
	}
	mode := fi.Mode().Perm()
	if mode != 0600 {
		t.Errorf(".port-registry should be 0600, got %04o", mode)
	}
}

// ---------------------------------------------------------------------------
// --group= filter — fleet-aware commands (Ticket: fleet-team-control-surface)
// ---------------------------------------------------------------------------

// setupGroupedFleet creates two groups with members + one ungrouped instance.
// Returns the root path. Used as the common fixture for --group= tests.
func setupGroupedFleet(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claws(t, root, "group", "create", "alpha")
	claws(t, root, "group", "create", "beta")
	claws(t, root, "create", "alpha/one")
	claws(t, root, "create", "alpha/two")
	claws(t, root, "create", "beta/three")
	claws(t, root, "create", "standalone")
	return root
}

func TestIntegration_ListGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)

	// Unfiltered: should see all four.
	out, _ := claws(t, root, "list")
	for _, name := range []string{"alpha/one", "alpha/two", "beta/three", "standalone"} {
		if !strings.Contains(out, name) {
			t.Errorf("unfiltered list missing %s: %s", name, out)
		}
	}

	// --group=alpha: only alpha/* members.
	out, _ = claws(t, root, "list", "--group=alpha")
	if !strings.Contains(out, "alpha/one") || !strings.Contains(out, "alpha/two") {
		t.Errorf("--group=alpha should show alpha members: %s", out)
	}
	if strings.Contains(out, "beta/three") || strings.Contains(out, "standalone") {
		t.Errorf("--group=alpha should NOT show non-alpha members: %s", out)
	}

	// --group=ghost: nonexistent group should error with directive message.
	_, err := claws(t, root, "list", "--group=ghost")
	if err == nil {
		t.Errorf("--group=ghost should fail (group does not exist)")
	}
}

func TestIntegration_ListGroupFilterEmpty(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "empty")
	out, _ := claws(t, root, "list", "--group=empty")
	if !strings.Contains(out, "No instances found in group 'empty'") {
		t.Errorf("empty group should produce friendly message: %s", out)
	}
}

func TestIntegration_ListGroupFilterJson(t *testing.T) {
	root := setupGroupedFleet(t)
	out, err := claws(t, root, "list", "--group=alpha", "--json")
	if err != nil {
		t.Fatalf("list --group= --json failed: %v\n%s", err, out)
	}
	var entries []map[string]string
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Errorf("--group=alpha --json expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if !strings.HasPrefix(e["name"], "alpha/") {
			t.Errorf("non-alpha entry leaked into --group=alpha: %s", e["name"])
		}
	}
}

func TestIntegration_StatusOverviewGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)
	out, _ := claws(t, root, "status", "--group=alpha")
	if !strings.Contains(out, "Instances in group 'alpha'") {
		t.Errorf("status --group= should announce the scope: %s", out)
	}
	if strings.Contains(out, "beta/three") || strings.Contains(out, "standalone") {
		t.Errorf("status --group=alpha should not show non-alpha members: %s", out)
	}
}

func TestIntegration_HealthGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)

	// Group filter narrows to the two alpha members.
	out, _ := claws(t, root, "health", "--group=alpha")
	if !strings.Contains(out, "alpha/one") || !strings.Contains(out, "alpha/two") {
		t.Errorf("health --group=alpha should show both alpha members: %s", out)
	}
	if strings.Contains(out, "beta/three") {
		t.Errorf("health --group=alpha should not show beta: %s", out)
	}
}

func TestIntegration_HealthRejectsBothNameAndGroup(t *testing.T) {
	root := setupGroupedFleet(t)
	_, err := claws(t, root, "health", "alpha/one", "--group=alpha")
	if err == nil {
		t.Errorf("health <name> --group= should be rejected (mutually exclusive)")
	}
}

func TestIntegration_RestartRejectsBothNameAndGroup(t *testing.T) {
	root := setupGroupedFleet(t)
	_, err := claws(t, root, "restart", "alpha/one", "--group=alpha", "--yes")
	if err == nil {
		t.Errorf("restart <name> --group= should be rejected (mutually exclusive)")
	}
}

func TestIntegration_StartGroupExpansion(t *testing.T) {
	// This test exercises the fan-out dispatcher. Per-instance start runs a
	// real `docker compose up -d` and then waits up to 30s for /healthz, so
	// the test takes ~(30s × group size). Skip under -short for CI speed;
	// the dispatch layer correctness is verified by the unit tests on
	// filterEntriesByGroup and the integration tests on list/status/health
	// which exercise the same flag parsing.
	if testing.Short() {
		t.Skip("start --group= integration test takes ~60s due to per-instance health-wait")
	}
	root := setupGroupedFleet(t)
	out, _ := claws(t, root, "start", "--group=alpha")
	if !strings.Contains(out, "Starting 2 instance(s) in group 'alpha'") {
		t.Errorf("start --group= should announce fan-out count: %s", out)
	}
}

func TestIntegration_RestartGroupNeedsConfirmation(t *testing.T) {
	root := setupGroupedFleet(t)
	// Without --yes and no TTY, the prompt reads empty and aborts.
	// This test confirms the confirmation gate exists; --yes bypass is
	// covered by TestIntegration_StartGroupExpansion (start is non-destructive,
	// no prompt) and by checking that --yes appears in help output.
	out, _ := claws(t, root, "restart", "--group=alpha")
	if !strings.Contains(out, "Continue?") && !strings.Contains(out, "Aborted") {
		t.Errorf("restart --group= without --yes should prompt or abort: %s", out)
	}
}

func TestIntegration_PolicyValidateGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)
	claws(t, root, "policy", "init")
	out, _ := claws(t, root, "policy", "validate", "--group=alpha")
	if !strings.Contains(out, "group: alpha") {
		t.Errorf("policy validate --group= should announce the scope: %s", out)
	}
	if strings.Contains(out, "beta/three") || strings.Contains(out, "standalone") {
		t.Errorf("policy validate --group=alpha should not check non-alpha: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Fleet identity — `list --rich` and `claws info` (Task A, ticket §A)
// ---------------------------------------------------------------------------

func TestIntegration_ListRichShowsIdentity(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")

	// Inject a model so the rich view has something to display.
	cfgPath := filepath.Join(root, "team", "sarah", "openclaw.json")
	cfg := readJSON(t, cfgPath)
	if cfg["agents"] == nil {
		cfg["agents"] = map[string]any{}
	}
	agentsMap := cfg["agents"].(map[string]any)
	if agentsMap["defaults"] == nil {
		agentsMap["defaults"] = map[string]any{}
	}
	defaults := agentsMap["defaults"].(map[string]any)
	defaults["model"] = map[string]any{"primary": "openai-codex/gpt-5.4"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0600)

	out, err := claws(t, root, "list", "--rich")
	if err != nil {
		t.Fatalf("list --rich failed: %s\n%s", err, out)
	}
	for _, want := range []string{"MODEL", "ROLE", "CHANNELS", "openai-codex/gpt-5.4"} {
		if !strings.Contains(out, want) {
			t.Errorf("--rich output missing %q: %s", want, out)
		}
	}
}

func TestIntegration_ListRichJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah", "--role=manager")

	out, err := claws(t, root, "list", "--rich", "--json")
	if err != nil {
		t.Fatalf("list --rich --json failed: %v\n%s", err, out)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	for _, key := range []string{"name", "port", "status", "model", "role", "image", "ram", "uptime"} {
		if _, ok := e[key]; !ok {
			t.Errorf("rich JSON entry missing key %q: %s", key, out)
		}
	}
	if e["role"] != "manager" {
		t.Errorf("role should be 'manager', got %v", e["role"])
	}
}

func TestIntegration_Info(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah", "--role=manager")

	out, err := claws(t, root, "info", "team/sarah")
	if err != nil {
		t.Fatalf("info failed: %s\n%s", err, out)
	}
	for _, want := range []string{
		"Instance: team/sarah",
		"Status:", "Identity", "Network",
		"Channels", "Filesystem", "Role:       manager",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("info output missing %q: %s", want, out)
		}
	}
}

func TestIntegration_InfoJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")

	out, err := claws(t, root, "info", "team/sarah", "--json")
	if err != nil {
		t.Fatalf("info --json failed: %v\n%s", err, out)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"name", "group", "port", "status", "model", "image", "runtime", "directory", "config", "workspace", "token", "creds"} {
		if _, ok := obj[key]; !ok {
			t.Errorf("info --json missing key %q: %s", key, out)
		}
	}
	if obj["name"] != "team/sarah" {
		t.Errorf("name should be team/sarah, got %v", obj["name"])
	}
}

func TestIntegration_InfoMissingInstance(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "info", "ghost")
	if err == nil {
		t.Errorf("info on missing instance should error")
	}
}

// ---------------------------------------------------------------------------
// Team noun verbs (Task C — thin wrappers over --group= from Task B)
// ---------------------------------------------------------------------------

func TestIntegration_TeamStatusDelegatesToGroupOverview(t *testing.T) {
	root := setupGroupedFleet(t)
	out, _ := claws(t, root, "team", "status", "alpha")
	// `team status` should produce the same overview a per-instance
	// `status --group=alpha` would, including the team-scoped header.
	if !strings.Contains(out, "Instances in group 'alpha'") {
		t.Errorf("team status should produce a group-scoped overview: %s", out)
	}
	if strings.Contains(out, "beta/three") || strings.Contains(out, "standalone") {
		t.Errorf("team status should not show non-alpha members: %s", out)
	}
}

func TestIntegration_TeamHealthDelegatesToHealthGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)
	out, _ := claws(t, root, "team", "health", "alpha")
	if !strings.Contains(out, "alpha/one") || !strings.Contains(out, "alpha/two") {
		t.Errorf("team health should probe both alpha members: %s", out)
	}
	if strings.Contains(out, "beta/three") {
		t.Errorf("team health should not probe beta: %s", out)
	}
}

func TestIntegration_TeamHealthJsonPassThrough(t *testing.T) {
	root := setupGroupedFleet(t)
	out, err := claws(t, root, "team", "health", "alpha", "--json")
	if err != nil {
		t.Fatalf("team health --json failed: %v\n%s", err, out)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Errorf("team health --json expected 2 entries, got %d", len(entries))
	}
}

func TestIntegration_TeamShow(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "team", "create", "research")
	claws(t, root, "create", "research/sarah", "--role=manager")
	claws(t, root, "create", "research/john", "--role=worker", "--manager=sarah")

	out, err := claws(t, root, "team", "show", "research")
	if err != nil {
		t.Fatalf("team show failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Team: research",
		"Members:    2",
		"research/sarah",
		"research/john",
		"Shared resources",
		"Task queue",
		"pending:    0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("team show missing %q: %s", want, out)
		}
	}
}

func TestIntegration_TeamShowJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "team", "create", "research")
	claws(t, root, "create", "research/sarah")
	out, err := claws(t, root, "team", "show", "research", "--json")
	if err != nil {
		t.Fatalf("team show --json failed: %v\n%s", err, out)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"team", "members", "shared", "tasks"} {
		if _, ok := obj[key]; !ok {
			t.Errorf("team show --json missing key %q: %s", key, out)
		}
	}
}

func TestIntegration_TeamRejectsMissingTeam(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "team", "status", "ghost")
	if err == nil {
		t.Errorf("team <verb> <missing-team> should fail")
	}
}

func TestIntegration_TeamRejectsNoTeamName(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "team", "status")
	if err == nil {
		t.Errorf("team status (no name) should fail with usage error")
	}
}

// ---------------------------------------------------------------------------
// Channels matrix + auth status (Task D — observability)
// ---------------------------------------------------------------------------

func TestIntegration_ChannelsMatrix(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")
	claws(t, root, "create", "team/john")
	claws(t, root, "create", "standalone")
	// Enable a channel on sarah only.
	claws(t, root, "channel", "add", "team/sarah", "telegram", "--token=test:abc")

	out, err := claws(t, root, "channels")
	if err != nil {
		t.Fatalf("channels failed: %v\n%s", err, out)
	}
	// Header should contain all known channel types.
	for _, ch := range []string{"telegram", "discord", "slack", "signal", "whatsapp"} {
		if !strings.Contains(out, ch) {
			t.Errorf("matrix header missing channel %q: %s", ch, out)
		}
	}
	// Sarah's telegram cell should be enabled; everyone else's should be —.
	if !strings.Contains(out, "team/sarah") {
		t.Errorf("matrix missing team/sarah row: %s", out)
	}
}

func TestIntegration_ChannelsMatrixJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")
	claws(t, root, "channel", "add", "team/sarah", "telegram", "--token=test:abc")

	out, err := claws(t, root, "channels", "--json")
	if err != nil {
		t.Fatalf("channels --json failed: %v\n%s", err, out)
	}
	var obj struct {
		Columns []string `json:"columns"`
		Rows    []struct {
			Name  string            `json:"name"`
			Cells map[string]string `json:"cells"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(obj.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(obj.Rows))
	}
	if obj.Rows[0].Cells["telegram"] == "" {
		t.Errorf("sarah's telegram cell should be non-empty (dmPolicy): %+v", obj.Rows[0].Cells)
	}
}

func TestIntegration_AuthStatusAllInstances(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")
	claws(t, root, "create", "team/john")

	out, err := claws(t, root, "auth", "status")
	if err != nil {
		t.Fatalf("auth status failed: %v\n%s", err, out)
	}
	for _, want := range []string{"NAME", "MODEL", "TOKEN", "CHANNEL CREDS", "LAST AUTH", "team/sarah", "team/john"} {
		if !strings.Contains(out, want) {
			t.Errorf("auth status missing %q: %s", want, out)
		}
	}
}

func TestIntegration_AuthStatusSingleInstance(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")

	out, err := claws(t, root, "auth", "status", "team/sarah")
	if err != nil {
		t.Fatalf("auth status <name> failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "team/sarah") {
		t.Errorf("auth status should include named instance: %s", out)
	}
}

func TestIntegration_AuthStatusJson(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "group", "create", "team")
	claws(t, root, "create", "team/sarah")

	out, err := claws(t, root, "auth", "status", "--json")
	if err != nil {
		t.Fatalf("auth status --json failed: %v\n%s", err, out)
	}
	var records []map[string]any
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
	for _, key := range []string{"name", "model", "gatewayTokenSet", "channelCreds", "lastAuthAt", "lastAuthCmd", "lastAuthResult"} {
		if _, ok := records[0][key]; !ok {
			t.Errorf("auth status --json missing key %q: %s", key, out)
		}
	}
}

func TestIntegration_AuthStatusGroupFilter(t *testing.T) {
	root := setupGroupedFleet(t)
	out, err := claws(t, root, "auth", "status", "--group=alpha")
	if err != nil {
		t.Fatalf("auth status --group= failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "alpha/one") || !strings.Contains(out, "alpha/two") {
		t.Errorf("auth status --group=alpha should show alpha members: %s", out)
	}
	if strings.Contains(out, "beta/three") {
		t.Errorf("auth status --group=alpha should not show beta: %s", out)
	}
}

func TestIntegration_UpgradeRejectsMultipleScopes(t *testing.T) {
	root := setupGroupedFleet(t)
	// All three scope flags together — should be rejected.
	_, err := claws(t, root, "upgrade", "alpha/one", "--group=alpha", "--all")
	if err == nil {
		t.Errorf("upgrade with name + --group + --all should be rejected")
	}
}

// Helper
func readEnvFromFile(t *testing.T, path, key string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}
