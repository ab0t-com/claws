package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenclawRuntimeDefaults(t *testing.T) {
	rt := openclawRuntime()
	if rt.Name != "openclaw" {
		t.Errorf("name should be 'openclaw', got '%s'", rt.Name)
	}
	if rt.GatewayService != "openclaw-gateway" {
		t.Errorf("gateway service should be 'openclaw-gateway', got '%s'", rt.GatewayService)
	}
	if rt.CLIService != "openclaw-cli" {
		t.Errorf("CLI service should be 'openclaw-cli', got '%s'", rt.CLIService)
	}
	if rt.InternalPort != 18789 {
		t.Errorf("internal port should be 18789, got %d", rt.InternalPort)
	}
	if rt.HealthEndpoint != "/healthz" {
		t.Errorf("health endpoint should be '/healthz', got '%s'", rt.HealthEndpoint)
	}
	if rt.ConfigFileName != "openclaw.json" {
		t.Errorf("config file should be 'openclaw.json', got '%s'", rt.ConfigFileName)
	}
	if !rt.Capabilities.Channels {
		t.Error("openclaw should support channels")
	}
	if !rt.Capabilities.Tasks {
		t.Error("openclaw should support tasks")
	}
}

func TestRuntimeCapabilities(t *testing.T) {
	rt := openclawRuntime()
	if err := rt.RequireCapability("channels"); err != nil {
		t.Errorf("openclaw should support channels: %v", err)
	}
	if err := rt.RequireCapability("auth"); err != nil {
		t.Errorf("openclaw should support auth: %v", err)
	}

	// Custom runtime without channels
	rt.Capabilities.Channels = false
	if err := rt.RequireCapability("channels"); err == nil {
		t.Error("should fail when channels disabled")
	}
}

func TestRuntimeHelpers(t *testing.T) {
	rt := openclawRuntime()

	if !rt.HasCLI() {
		t.Error("openclaw should have CLI")
	}

	if !rt.SupportsChannels() {
		t.Error("openclaw should support channels")
	}

	if !rt.SupportsPairing() {
		t.Error("openclaw should support pairing")
	}

	if bp := rt.BridgePortFor(18789); bp != 18790 {
		t.Errorf("bridge port should be 18790, got %d", bp)
	}

	// No bridge
	rt.BridgePort = 0
	if bp := rt.BridgePortFor(18789); bp != 0 {
		t.Errorf("no bridge should return 0, got %d", bp)
	}

	// No CLI
	rt.CLIService = ""
	if rt.HasCLI() {
		t.Error("should not have CLI when empty")
	}
}

func TestRuntimeConfigPath(t *testing.T) {
	rt := openclawRuntime()
	path := rt.ConfigPath("/home/ubuntu/.openclaw/alice")
	if path != "/home/ubuntu/.openclaw/alice/openclaw.json" {
		t.Errorf("config path wrong: %s", path)
	}

	rt.ConfigFileName = "agent.yaml"
	path = rt.ConfigPath("/home/ubuntu/.openclaw/alice")
	if path != "/home/ubuntu/.openclaw/alice/agent.yaml" {
		t.Errorf("custom config path wrong: %s", path)
	}
}

func TestLoadRuntimeFromFile(t *testing.T) {
	tmp := t.TempDir()
	rt := Runtime{
		Name:           "test-agent",
		Description:    "Test runtime",
		DefaultImage:   "test:latest",
		GatewayService: "test-gateway",
		InternalPort:   8080,
		HealthEndpoint: "/health",
		ConfigFileName: "config.json",
		Capabilities:   RuntimeCapabilities{Channels: false, Auth: true},
	}

	data, _ := json.MarshalIndent(rt, "", "  ")
	os.WriteFile(filepath.Join(tmp, "test-agent.json"), data, 0644)

	loaded, err := loadRuntimeFromFile(filepath.Join(tmp, "test-agent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "test-agent" {
		t.Errorf("name should be 'test-agent', got '%s'", loaded.Name)
	}
	if loaded.InternalPort != 8080 {
		t.Errorf("port should be 8080, got %d", loaded.InternalPort)
	}
	if loaded.Capabilities.Channels {
		t.Error("should not support channels")
	}
}

func TestResolveRuntimeDefault(t *testing.T) {
	paths := testPaths(t)

	// No instance, should return openclaw default
	rt, err := resolveRuntime(paths, "nonexistent")
	if err != nil {
		t.Errorf("nonexistent instance should default, not error: %v", err)
	}
	if rt.Name != "openclaw" {
		t.Errorf("default runtime should be openclaw, got '%s'", rt.Name)
	}
}

func TestResolveRuntimeFromEnv(t *testing.T) {
	paths := testPaths(t)

	// Create instance with custom runtime
	dir := filepath.Join(paths.Root, "alpha")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "instance.env"), []byte("CLAWS_RUNTIME=custom\n"), 0600)

	// Register custom runtime
	custom := Runtime{Name: "custom", Description: "Custom", DefaultImage: "custom:v1", GatewayService: "custom-gw", InternalPort: 9090, HealthEndpoint: "/up", ConfigFileName: "custom.json"}
	saveRuntimeToFile(paths, custom)

	rt, err := resolveRuntime(paths, "alpha")
	if err != nil {
		t.Fatalf("should resolve: %v", err)
	}
	if rt.Name != "custom" {
		t.Errorf("should resolve to 'custom', got '%s'", rt.Name)
	}
	if rt.InternalPort != 9090 {
		t.Errorf("should have port 9090, got %d", rt.InternalPort)
	}
}

func TestResolveRuntimeBrokenReference(t *testing.T) {
	paths := testPaths(t)

	// Create instance referencing a runtime that doesn't exist
	dir := filepath.Join(paths.Root, "alpha")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "instance.env"), []byte("CLAWS_RUNTIME=nonexistent-runtime\n"), 0600)

	_, err := resolveRuntime(paths, "alpha")
	if err == nil {
		t.Error("should error when runtime reference is broken")
	}
	if !strings.Contains(err.Error(), "nonexistent-runtime") {
		t.Errorf("error should mention the missing runtime: %v", err)
	}
}

func TestResolveRuntimeNoEnvFile(t *testing.T) {
	paths := testPaths(t)

	// Instance dir exists but no env file — should default
	dir := filepath.Join(paths.Root, "alpha")
	os.MkdirAll(dir, 0755)

	rt, err := resolveRuntime(paths, "alpha")
	if err != nil {
		t.Errorf("missing env should default, not error: %v", err)
	}
	if rt.Name != "openclaw" {
		t.Errorf("should default to openclaw, got '%s'", rt.Name)
	}
}

func TestMustResolveRuntimeFallback(t *testing.T) {
	paths := testPaths(t)

	dir := filepath.Join(paths.Root, "alpha")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "instance.env"), []byte("CLAWS_RUNTIME=broken\n"), 0600)

	// mustResolveRuntime should fallback to openclaw with a warning, not panic
	rt := mustResolveRuntime(paths, "alpha")
	if rt.Name != "openclaw" {
		t.Errorf("mustResolve should fallback to openclaw, got '%s'", rt.Name)
	}
}

func TestListRuntimes(t *testing.T) {
	paths := testPaths(t)

	// Should have at least openclaw
	runtimes := listRuntimes(paths)
	if _, ok := runtimes["openclaw"]; !ok {
		t.Error("should include built-in openclaw runtime")
	}

	// Add a custom one
	custom := Runtime{Name: "custom", Description: "Custom"}
	saveRuntimeToFile(paths, custom)

	runtimes = listRuntimes(paths)
	if _, ok := runtimes["custom"]; !ok {
		t.Error("should include custom runtime")
	}
}

func TestRuntimeNewFields(t *testing.T) {
	rt := openclawRuntime()
	if rt.ProjectPrefix != "openclaw" {
		t.Errorf("project prefix should be 'openclaw', got '%s'", rt.ProjectPrefix)
	}
	if rt.ConfigFormat != "json" {
		t.Errorf("config format should be 'json', got '%s'", rt.ConfigFormat)
	}
	if rt.ComposeOverride != "docker-compose.override.yml" {
		t.Errorf("compose override should be set, got '%s'", rt.ComposeOverride)
	}
	if rt.HealthCheckType != "http" {
		t.Errorf("health check type should be 'http', got '%s'", rt.HealthCheckType)
	}
	if rt.HealthCheckRetries != 15 {
		t.Errorf("health check retries should be 15, got %d", rt.HealthCheckRetries)
	}
}

func TestMakeProjectName(t *testing.T) {
	rt := openclawRuntime()

	ref := InstanceRef{Name: "alice"}
	if got := rt.MakeProjectName(ref); got != "openclaw-alice" {
		t.Errorf("expected 'openclaw-alice', got '%s'", got)
	}

	ref = InstanceRef{Group: "team", Name: "sarah"}
	if got := rt.MakeProjectName(ref); got != "openclaw-team-sarah" {
		t.Errorf("expected 'openclaw-team-sarah', got '%s'", got)
	}

	// Custom prefix
	rt.ProjectPrefix = "myagent"
	ref = InstanceRef{Name: "bob"}
	if got := rt.MakeProjectName(ref); got != "myagent-bob" {
		t.Errorf("expected 'myagent-bob', got '%s'", got)
	}
}

func TestDefaultContainerName(t *testing.T) {
	rt := openclawRuntime()
	ref := InstanceRef{Name: "alice"}
	if got := rt.DefaultContainerName(ref); got != "openclaw-alice-openclaw-gateway-1" {
		t.Errorf("expected 'openclaw-alice-openclaw-gateway-1', got '%s'", got)
	}
}

func TestOverridePath(t *testing.T) {
	rt := openclawRuntime()
	if got := rt.OverridePath("/tmp/instance"); got != "/tmp/instance/docker-compose.override.yml" {
		t.Errorf("expected override path, got '%s'", got)
	}

	rt.ComposeOverride = "custom-override.yml"
	if got := rt.OverridePath("/tmp/instance"); got != "/tmp/instance/custom-override.yml" {
		t.Errorf("expected custom override path, got '%s'", got)
	}
}

func TestLoadRuntimeCorruptedJSON(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "bad.json"), []byte("{invalid json"), 0644)

	_, err := loadRuntimeFromFile(filepath.Join(tmp, "bad.json"))
	if err == nil {
		t.Error("should error on corrupted JSON")
	}
}

func TestLoadRuntimeMissingName(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "noname.json"), []byte(`{"description":"test"}`), 0644)

	_, err := loadRuntimeFromFile(filepath.Join(tmp, "noname.json"))
	if err == nil {
		t.Error("should error when name is missing")
	}
}

func TestListRuntimesSkipsNonJSON(t *testing.T) {
	paths := testPaths(t)
	rtDir := filepath.Join(paths.Root, runtimesDir)
	os.MkdirAll(rtDir, 0755)

	// Write a non-JSON file
	os.WriteFile(filepath.Join(rtDir, "readme.txt"), []byte("not a runtime"), 0644)

	// Write a valid runtime
	rt := Runtime{Name: "valid"}
	saveRuntimeToFile(paths, rt)

	runtimes := listRuntimes(paths)
	if _, ok := runtimes["valid"]; !ok {
		t.Error("should load valid runtime")
	}
	// Should not have loaded the txt file as a runtime
	if len(runtimes) != 2 { // openclaw + valid
		t.Errorf("expected 2 runtimes, got %d", len(runtimes))
	}
}

func TestIntegration_RuntimeList(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "list")
	if err != nil {
		t.Fatalf("runtime list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "openclaw") {
		t.Errorf("should show openclaw runtime: %s", out)
	}
	if !strings.Contains(out, "built-in") {
		t.Errorf("should mark openclaw as built-in: %s", out)
	}
}

func TestIntegration_RuntimeShow(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "show", "openclaw")
	if err != nil {
		t.Fatalf("runtime show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "openclaw-gateway") {
		t.Errorf("should show gateway service: %s", out)
	}
	if !strings.Contains(out, "Channels") {
		t.Errorf("should show capabilities: %s", out)
	}
}

func TestIntegration_RuntimeShowJson(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "show", "openclaw", "--json")
	if err != nil {
		t.Fatalf("runtime show --json failed: %v\n%s", err, out)
	}
	var rt Runtime
	if err := json.Unmarshal([]byte(out), &rt); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rt.Name != "openclaw" {
		t.Errorf("JSON name should be openclaw, got '%s'", rt.Name)
	}
}

func TestIntegration_RuntimeAdd(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "add", "my-agent", "--image=my-agent:v1", "--health=/health", "--no-channels", "--no-pairing")
	if err != nil {
		t.Fatalf("runtime add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "registered") {
		t.Errorf("should confirm registration: %s", out)
	}

	// Verify it appears in list
	out, _ = claws(t, root, "runtime", "list")
	if !strings.Contains(out, "my-agent") {
		t.Errorf("should show in list: %s", out)
	}
}

func TestIntegration_RuntimeRemove(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "runtime", "add", "temp-agent", "--image=temp:v1")
	out, err := claws(t, root, "runtime", "remove", "temp-agent")
	if err != nil {
		t.Fatalf("runtime remove failed: %v\n%s", err, out)
	}

	// Should not appear in list
	out, _ = claws(t, root, "runtime", "list")
	if strings.Contains(out, "temp-agent") {
		t.Errorf("should not show after removal: %s", out)
	}
}

func TestIntegration_RuntimeCannotRemoveBuiltin(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "runtime", "remove", "openclaw")
	if err == nil {
		t.Error("should not be able to remove built-in runtime")
	}
}

func TestIntegration_CreateStoresRuntime(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	envFile := filepath.Join(root, "alpha", "instance.env")
	rtName := readEnvFromFile(t, envFile, "CLAWS_RUNTIME")
	if rtName != "openclaw" {
		t.Errorf("default runtime should be 'openclaw', got '%s'", rtName)
	}
}

func TestIntegration_CreateWithCustomRuntime(t *testing.T) {
	root := t.TempDir()

	// Register a custom runtime first
	claws(t, root, "runtime", "add", "myagent", "--image=myagent:v1", "--no-channels")

	out, err := claws(t, root, "create", "alpha", "--runtime=myagent")
	if err != nil {
		t.Fatalf("create with custom runtime failed: %v\n%s", err, out)
	}

	envFile := filepath.Join(root, "alpha", "instance.env")
	rtName := readEnvFromFile(t, envFile, "CLAWS_RUNTIME")
	if rtName != "myagent" {
		t.Errorf("runtime should be 'myagent', got '%s'", rtName)
	}
	img := readEnvFromFile(t, envFile, "OPENCLAW_IMAGE")
	if img != "myagent:v1" {
		t.Errorf("image should be 'myagent:v1', got '%s'", img)
	}
}

func TestIntegration_CreateWithUnknownRuntime(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "create", "alpha", "--runtime=nonexistent")
	if err == nil {
		t.Error("should fail with unknown runtime")
	}
}

// ---------------------------------------------------------------------------
// 7.1 — --from= inheritance
// ---------------------------------------------------------------------------

func TestIntegration_RuntimeAddFrom(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "add", "nemoclaw", "--from=openclaw", "--image=nemoclaw:latest")
	if err != nil {
		t.Fatalf("runtime add --from failed: %v\n%s", err, out)
	}

	// Show it and verify inherited fields
	out, _ = claws(t, root, "runtime", "show", "nemoclaw", "--json")
	var rt Runtime
	json.Unmarshal([]byte(out), &rt)

	if rt.DefaultImage != "nemoclaw:latest" {
		t.Errorf("image should be overridden to nemoclaw:latest, got '%s'", rt.DefaultImage)
	}
	if rt.GatewayService != "openclaw-gateway" {
		t.Errorf("gateway service should be inherited from openclaw, got '%s'", rt.GatewayService)
	}
	if rt.HealthEndpoint != "/healthz" {
		t.Errorf("health should be inherited from openclaw, got '%s'", rt.HealthEndpoint)
	}
	if !rt.Capabilities.Channels {
		t.Error("channels capability should be inherited from openclaw")
	}
}

func TestIntegration_RuntimeAddFromOverrideHealth(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "runtime", "add", "nanoclaw", "--from=openclaw", "--image=nanoclaw:v1", "--health=/status")

	out, _ := claws(t, root, "runtime", "show", "nanoclaw", "--json")
	var rt Runtime
	json.Unmarshal([]byte(out), &rt)

	if rt.HealthEndpoint != "/status" {
		t.Errorf("health should be overridden to /status, got '%s'", rt.HealthEndpoint)
	}
	if rt.DefaultImage != "nanoclaw:v1" {
		t.Errorf("image should be nanoclaw:v1, got '%s'", rt.DefaultImage)
	}
}

func TestIntegration_RuntimeAddFromNonexistent(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "runtime", "add", "bad", "--from=nonexistent")
	if err == nil {
		t.Error("should fail when --from references nonexistent runtime")
	}
}

// ---------------------------------------------------------------------------
// 7.2 — runtime init scaffolding
// ---------------------------------------------------------------------------

func TestIntegration_RuntimeInit(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "runtime", "init", "my-agent")
	if err != nil {
		t.Fatalf("runtime init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "scaffolded") {
		t.Errorf("should confirm scaffolding: %s", out)
	}

	// JSON should exist and be valid
	jsonPath := filepath.Join(root, "runtimes", "my-agent.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal("runtime JSON not created")
	}
	rt, err := loadRuntimeFromFile(jsonPath)
	if err != nil {
		t.Fatalf("runtime JSON invalid: %v", err)
	}
	if rt.Name != "my-agent" {
		t.Errorf("name should be my-agent, got '%s'", rt.Name)
	}
	if rt.DefaultImage != "my-agent:latest" {
		t.Errorf("default image should be my-agent:latest, got '%s'", rt.DefaultImage)
	}

	// Compose should exist
	composePath := filepath.Join(root, "runtimes", "my-agent-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		t.Fatal("compose template not created")
	}
	composeData, _ := os.ReadFile(composePath)
	if !strings.Contains(string(composeData), "my-agent-gateway") {
		t.Error("compose should reference the gateway service name")
	}
}

func TestIntegration_RuntimeInitIdempotent(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "runtime", "init", "my-agent")
	// Second init should fail without --force
	_, err := claws(t, root, "runtime", "init", "my-agent")
	if err == nil {
		t.Error("second init should fail without --force")
	}
}

// ---------------------------------------------------------------------------
// 7.3 — runtime test
// ---------------------------------------------------------------------------

func TestIntegration_RuntimeTestOpenclaw(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "runtime", "test", "openclaw")
	// May fail if image doesn't exist in test env, but should not crash
	if !strings.Contains(out, "openclaw") {
		t.Errorf("should reference openclaw: %s", out)
	}
}

func TestIntegration_RuntimeTestNonexistent(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "runtime", "test", "nonexistent")
	if err == nil {
		t.Error("should fail for nonexistent runtime")
	}
}

// ---------------------------------------------------------------------------
// 7.4 — export/import
// ---------------------------------------------------------------------------

func TestIntegration_RuntimeExportImport(t *testing.T) {
	root := t.TempDir()
	// Register a runtime
	claws(t, root, "runtime", "add", "test-rt", "--image=test:v1", "--health=/up", "--no-channels")

	// Export
	out, err := claws(t, root, "runtime", "export", "test-rt")
	if err != nil {
		t.Fatalf("export failed: %v\n%s", err, out)
	}

	// Write export to file
	exportFile := filepath.Join(root, "export.json")
	os.WriteFile(exportFile, []byte(out), 0644)

	// Remove original
	claws(t, root, "runtime", "remove", "test-rt")

	// Import
	out, err = claws(t, root, "runtime", "import", exportFile)
	if err != nil {
		t.Fatalf("import failed: %v\n%s", err, out)
	}

	// Verify it's back
	out, _ = claws(t, root, "runtime", "show", "test-rt", "--json")
	var rt Runtime
	json.Unmarshal([]byte(out), &rt)
	if rt.DefaultImage != "test:v1" {
		t.Errorf("imported image should be test:v1, got '%s'", rt.DefaultImage)
	}
}

// ---------------------------------------------------------------------------
// 7.5 — detect
// ---------------------------------------------------------------------------

func TestIntegration_RuntimeDetect(t *testing.T) {
	root := t.TempDir()
	// Detect openclaw:local (should work if image exists)
	out, _ := claws(t, root, "runtime", "detect", "openclaw:local")
	// May fail if no image, but should not crash
	if out == "" {
		t.Error("detect should produce output")
	}
}

func TestIntegration_RuntimeDetectNonexistent(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "runtime", "detect", "nonexistent-image:v999")
	if err == nil {
		t.Error("should fail for nonexistent image")
	}
}

func TestIntegration_RuntimeCannotOverrideBuiltin(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "runtime", "add", "openclaw", "--image=fake:v1")
	if err == nil {
		t.Error("should not be able to override built-in runtime")
	}
}
