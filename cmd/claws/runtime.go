package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// Runtime Interface — the adapter pattern contract
// ---------------------------------------------------------------------------

// Runtime defines how claws interacts with a specific agent gateway runtime.
// Every containerized agent runtime (OpenClaw, custom, etc.) implements this
// interface by providing a Runtime struct. claws uses these values instead
// of hardcoded assumptions throughout the codebase.
type Runtime struct {
	// Identity
	Name        string `json:"name"`        // unique identifier (e.g., "openclaw")
	Description string `json:"description"` // human-friendly description
	Version     string `json:"version"`     // runtime version (informational)

	// Docker Image
	DefaultImage string `json:"defaultImage"` // e.g., "openclaw:local"

	// Compose Services
	GatewayService string `json:"gatewayService"` // compose service name for the gateway (e.g., "openclaw-gateway")
	CLIService     string `json:"cliService"`     // compose service name for CLI (e.g., "openclaw-cli"), empty = no CLI service
	ComposeFile    string `json:"composeFile"`    // compose template filename (e.g., "docker-compose.yml")

	// Networking
	InternalPort int `json:"internalPort"` // port gateway listens on inside container (e.g., 18789)
	BridgePort   int `json:"bridgePort"`   // companion port offset from gateway (0 = none, 1 = gateway+1)

	// Health Probes
	HealthEndpoint string `json:"healthEndpoint"` // liveness endpoint (e.g., "/healthz")
	ReadyEndpoint  string `json:"readyEndpoint"`  // readiness endpoint (e.g., "/readyz"), empty = no readiness check

	// Container Paths (inside the container)
	ContainerHome      string `json:"containerHome"`      // e.g., "/home/node"
	ContainerConfigDir string `json:"containerConfigDir"` // e.g., "/home/node/.openclaw"
	ContainerWorkspace string `json:"containerWorkspace"` // e.g., "/home/node/.openclaw/workspace"

	// Mount Points (where shared resources mount inside the container)
	MountSkills    string `json:"mountSkills"`    // e.g., "/home/node/.openclaw/bundled-skills"
	MountWorkspace string `json:"mountWorkspace"` // e.g., "/home/node/.openclaw/shared"
	MountHooks     string `json:"mountHooks"`     // e.g., "/home/node/.openclaw/shared-hooks"
	MountTasks     string `json:"mountTasks"`     // e.g., "/home/node/.openclaw/tasks"
	MountOutput    string `json:"mountOutput"`    // e.g., "/home/node/.openclaw/output"
	MountManager   string `json:"mountManager"`   // e.g., "/home/node/.openclaw/manager"
	MountWorkers   string `json:"mountWorkers"`   // e.g., "/home/node/.openclaw/workers" (workers/<name> appended)

	// Environment Variable for Skills Dir (set in compose override)
	SkillsEnvVar string `json:"skillsEnvVar"` // e.g., "OPENCLAW_BUNDLED_SKILLS_DIR"

	// Project Naming
	ProjectPrefix string `json:"projectPrefix"` // Docker project name prefix (e.g., "openclaw")

	// Config
	ConfigFileName    string `json:"configFileName"`    // e.g., "openclaw.json"
	ConfigFormat      string `json:"configFormat"`      // "json", "yaml", "toml" — for display/merge awareness
	SupportsConfig    bool   `json:"supportsConfig"`    // whether config merging applies
	ComposeOverride   string `json:"composeOverride"`   // override filename (e.g., "docker-compose.override.yml")

	// Health Check Tuning
	HealthCheckType    string `json:"healthCheckType"`    // "http" (default), "tcp", "exec"
	HealthCheckTimeout int    `json:"healthCheckTimeout"` // seconds per attempt (default: 2)
	HealthCheckRetries int    `json:"healthCheckRetries"` // number of retries (default: 15)

	// Custom Environment Variables (injected into compose override)
	CustomEnvVars map[string]string `json:"customEnvVars,omitempty"` // e.g., {"MY_VAR": "value"}

	// CLI Commands (how to invoke operations inside the container)
	// Empty slices = feature not supported by this runtime.
	ChannelAddCmd   []string `json:"channelAddCmd,omitempty"`   // e.g., ["channels", "add"]
	ChannelLoginCmd []string `json:"channelLoginCmd,omitempty"` // e.g., ["channels", "login", "--channel"]
	PairingCmd      []string `json:"pairingCmd,omitempty"`      // e.g., ["pairing", "approve"]
	AuthCodexCmd    []string `json:"authCodexCmd,omitempty"`    // e.g., ["models", "auth", "login", "--provider", "openai-codex", "--set-default"]
	AuthApiKeyCmd   []string `json:"authApiKeyCmd,omitempty"`   // e.g., ["onboard", "--mode", "headless"]
	ConfigGetCmd    []string `json:"configGetCmd,omitempty"`    // e.g., ["config", "get"]
	ConfigSetCmd    []string `json:"configSetCmd,omitempty"`    // e.g., ["config", "set"]

	// Hook register (v1.4) — declares the lifecycle hook contract.
	// Templates and `claws apply` use this to know where to materialise
	// hook scripts and which event names this runtime recognises.
	HooksDir            string   `json:"hooksDir,omitempty"`            // workspace-relative dir, e.g. "hooks"
	HookFileExt         string   `json:"hookFileExt,omitempty"`         // e.g. ".sh"
	SupportedHookEvents []string `json:"supportedHookEvents,omitempty"` // e.g. ["onStart","onMessage","onIdle","onError","onShutdown"]

	// Capabilities — what features this runtime supports
	Capabilities RuntimeCapabilities `json:"capabilities"`
}

// RuntimeCapabilities declares which claws features this runtime supports.
type RuntimeCapabilities struct {
	Channels bool `json:"channels"` // messaging channel integration
	Pairing  bool `json:"pairing"`  // DM pairing/approval flow
	Auth     bool `json:"auth"`     // model auth (codex, api keys)
	Config   bool `json:"config"`   // JSON config merging
	Tasks    bool `json:"tasks"`    // manager/worker task queue
	Shared   bool `json:"shared"`   // shared resources (skills, workspace, hooks)
	Bridge   bool `json:"bridge"`   // bridge/companion port
}

// ---------------------------------------------------------------------------
// Built-in OpenClaw Runtime
// ---------------------------------------------------------------------------

func openclawRuntime() Runtime {
	return Runtime{
		Name:        "openclaw",
		Description: "OpenClaw AI agent gateway",
		Version:     "2026.3",

		DefaultImage: "openclaw:local",

		GatewayService: "openclaw-gateway",
		CLIService:     "openclaw-cli",
		ComposeFile:    "docker-compose.yml",

		InternalPort: 18789,
		BridgePort:   1, // gateway + 1

		HealthEndpoint: "/healthz",
		ReadyEndpoint:  "/readyz",

		ContainerHome:      "/home/node",
		ContainerConfigDir: "/home/node/.openclaw",
		ContainerWorkspace: "/home/node/.openclaw/workspace",

		MountSkills:    "/home/node/.openclaw/bundled-skills",
		MountWorkspace: "/home/node/.openclaw/shared",
		MountHooks:     "/home/node/.openclaw/shared-hooks",
		MountTasks:     "/home/node/.openclaw/tasks",
		MountOutput:    "/home/node/.openclaw/output",
		MountManager:   "/home/node/.openclaw/manager",
		MountWorkers:   "/home/node/.openclaw/workers",

		SkillsEnvVar: "OPENCLAW_BUNDLED_SKILLS_DIR",

		ProjectPrefix: "openclaw",

		ConfigFileName:  "openclaw.json",
		ConfigFormat:    "json",
		SupportsConfig:  true,
		ComposeOverride: "docker-compose.override.yml",

		HealthCheckType:    "http",
		HealthCheckTimeout: 2,
		HealthCheckRetries: 15,

		ChannelAddCmd:   []string{"channels", "add"},
		ChannelLoginCmd: []string{"channels", "login", "--channel"},
		PairingCmd:      []string{"pairing", "approve"},
		AuthCodexCmd:    []string{"models", "auth", "login", "--provider", "openai-codex", "--set-default"},
		AuthApiKeyCmd:   []string{"onboard", "--mode", "headless"},
		ConfigGetCmd:    []string{"config", "get"},
		ConfigSetCmd:    []string{"config", "set"},

		// Hook contract — openclaw runtime scans workspace/hooks/<event>.sh
		// on startup and invokes the matching script on each lifecycle event.
		HooksDir:    "hooks",
		HookFileExt: ".sh",
		SupportedHookEvents: []string{
			"onStart", "onMessage", "onIdle", "onError", "onShutdown",
		},

		Capabilities: RuntimeCapabilities{
			Channels: true,
			Pairing:  true,
			Auth:     true,
			Config:   true,
			Tasks:    true,
			Shared:   true,
			Bridge:   true,
		},
	}
}

// ---------------------------------------------------------------------------
// Runtime Registry
// ---------------------------------------------------------------------------

const runtimesDir = "runtimes"

// builtinRuntimes are always available.
var builtinRuntimes = map[string]Runtime{
	"openclaw": openclawRuntime(),
}

// listRuntimes returns all available runtimes (built-in + custom).
func listRuntimes(paths Paths) map[string]Runtime {
	runtimes := make(map[string]Runtime)

	// Built-ins
	for k, v := range builtinRuntimes {
		runtimes[k] = v
	}

	// Custom runtimes from disk
	rtDir := filepath.Join(paths.Root, runtimesDir)
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		return runtimes
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		rt, err := loadRuntimeFromFile(filepath.Join(rtDir, e.Name()))
		if err != nil {
			continue
		}
		runtimes[rt.Name] = rt
	}

	return runtimes
}

// getRuntimeByName looks up a runtime by name.
func getRuntimeByName(paths Paths, name string) (Runtime, bool) {
	all := listRuntimes(paths)
	rt, ok := all[name]
	return rt, ok
}

// resolveRuntime gets the runtime for a specific instance.
// Reads CLAWS_RUNTIME from instance.env, defaults to "openclaw".
// Returns error only if the instance explicitly references a runtime that doesn't exist.
func resolveRuntime(paths Paths, instanceName string) (Runtime, error) {
	ref, err := ParseRef(instanceName)
	if err != nil {
		return openclawRuntime(), nil // invalid ref → default is safe
	}
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	rtName := readEnvValue(envFile, "CLAWS_RUNTIME")
	if rtName == "" {
		return openclawRuntime(), nil // no runtime specified → default
	}
	rt, ok := getRuntimeByName(paths, rtName)
	if !ok {
		return Runtime{}, fmt.Errorf("runtime '%s' not found (referenced by instance '%s'). Register it: claws runtime add %s --image=...", rtName, instanceName, rtName)
	}
	return rt, nil
}

// mustResolveRuntime is a convenience wrapper that returns the default on error.
// Use resolveRuntime directly when you need to handle the error.
func mustResolveRuntime(paths Paths, instanceName string) Runtime {
	rt, err := resolveRuntime(paths, instanceName)
	if err != nil {
		warn(err.Error())
		return openclawRuntime()
	}
	return rt
}

// loadRuntimeFromFile loads a custom runtime definition from a JSON file.
func loadRuntimeFromFile(path string) (Runtime, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Runtime{}, err
	}
	var rt Runtime
	if err := json.Unmarshal(data, &rt); err != nil {
		return Runtime{}, err
	}
	if rt.Name == "" {
		return Runtime{}, fmt.Errorf("runtime missing 'name' field")
	}
	return rt, nil
}

// saveRuntimeToFile writes a runtime definition to the registry.
func saveRuntimeToFile(paths Paths, rt Runtime) error {
	rtDir := filepath.Join(paths.Root, runtimesDir)
	os.MkdirAll(rtDir, 0755)
	data, err := json.MarshalIndent(rt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rtDir, rt.Name+".json"), append(data, '\n'), 0644)
}

// ---------------------------------------------------------------------------
// Runtime helper methods
// ---------------------------------------------------------------------------

// ConfigPath returns the full path to the instance config file.
func (rt Runtime) ConfigPath(instanceDir string) string {
	return filepath.Join(instanceDir, rt.ConfigFileName)
}

// OverridePath returns the full path to the compose override file.
func (rt Runtime) OverridePath(instanceDir string) string {
	name := rt.ComposeOverride
	if name == "" {
		name = "docker-compose.override.yml"
	}
	return filepath.Join(instanceDir, name)
}

// MakeProjectName builds the Docker Compose project name for an instance.
func (rt Runtime) MakeProjectName(ref InstanceRef) string {
	prefix := rt.ProjectPrefix
	if prefix == "" {
		prefix = "openclaw" // backward compat
	}
	if ref.Group != "" {
		return prefix + "-" + ref.Group + "-" + ref.Name
	}
	return prefix + "-" + ref.Name
}

// DefaultContainerName returns the expected container name for the gateway.
func (rt Runtime) DefaultContainerName(ref InstanceRef) string {
	return rt.MakeProjectName(ref) + "-" + rt.GatewayService + "-1"
}

// HasCLI returns true if this runtime has a CLI service.
func (rt Runtime) HasCLI() bool {
	return rt.CLIService != ""
}

// SupportsChannels returns true if channel operations are available.
func (rt Runtime) SupportsChannels() bool {
	return rt.Capabilities.Channels && len(rt.ChannelAddCmd) > 0
}

// SupportsPairing returns true if DM pairing is available.
func (rt Runtime) SupportsPairing() bool {
	return rt.Capabilities.Pairing && len(rt.PairingCmd) > 0
}

// SupportsAuth returns true if auth operations are available.
func (rt Runtime) SupportsAuth() bool {
	return rt.Capabilities.Auth
}

// ComposeTemplatePath returns the full path to this runtime's compose template.
func (rt Runtime) ComposeTemplatePath(paths Paths) string {
	// Check runtime-specific compose in registry
	rtCompose := filepath.Join(paths.Root, runtimesDir, rt.Name+"-compose.yml")
	if _, err := os.Stat(rtCompose); err == nil {
		return rtCompose
	}
	// Fall back to paths.ComposeTemplate (default)
	return paths.ComposeTemplate
}

// BridgePortFor calculates the bridge port for a given gateway port.
// Returns 0 if the runtime doesn't use a bridge port.
func (rt Runtime) BridgePortFor(gatewayPort int) int {
	if rt.BridgePort == 0 {
		return 0
	}
	return gatewayPort + rt.BridgePort
}

// RequireCapability returns an error if a capability is not supported.
func (rt Runtime) RequireCapability(capability string) error {
	switch capability {
	case "channels":
		if !rt.Capabilities.Channels {
			return fmt.Errorf("runtime '%s' does not support messaging channels", rt.Name)
		}
	case "pairing":
		if !rt.Capabilities.Pairing {
			return fmt.Errorf("runtime '%s' does not support DM pairing", rt.Name)
		}
	case "auth":
		if !rt.Capabilities.Auth {
			return fmt.Errorf("runtime '%s' does not support model authentication", rt.Name)
		}
	case "config":
		if !rt.Capabilities.Config {
			return fmt.Errorf("runtime '%s' does not support config merging", rt.Name)
		}
	case "tasks":
		if !rt.Capabilities.Tasks {
			return fmt.Errorf("runtime '%s' does not support task queues", rt.Name)
		}
	case "shared":
		if !rt.Capabilities.Shared {
			return fmt.Errorf("runtime '%s' does not support shared resources", rt.Name)
		}
	case "cli":
		if !rt.HasCLI() {
			return fmt.Errorf("runtime '%s' does not have a CLI service", rt.Name)
		}
	default:
		return fmt.Errorf("unknown capability: %s", capability)
	}
	return nil
}

// ---------------------------------------------------------------------------
// claws runtime — manage runtime definitions
// ---------------------------------------------------------------------------

func cmdRuntime(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime <list|show|add|remove>")
	}
	switch args[0] {
	case "list", "ls":
		return cmdRuntimeList(args[1:])
	case "show":
		return cmdRuntimeShow(args[1:])
	case "add":
		return cmdRuntimeAdd(args[1:])
	case "init":
		return cmdRuntimeInit(args[1:])
	case "test":
		return cmdRuntimeTest(args[1:])
	case "export":
		return cmdRuntimeExport(args[1:])
	case "import":
		return cmdRuntimeImport(args[1:])
	case "detect":
		return cmdRuntimeDetect(args[1:])
	case "remove", "rm":
		return cmdRuntimeRemove(args[1:])
	default:
		return errorf("unknown runtime subcommand: %s", args[0])
	}
}

func cmdRuntimeList(args []string) error {
	paths := resolvePaths()
	runtimes := listRuntimes(paths)

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"

	fmt.Printf("%s%-15s %-30s %-25s %s%s\n", bold, "NAME", "DESCRIPTION", "IMAGE", "CAPABILITIES", nc)
	fmt.Printf("%-15s %-30s %-25s %s\n", "───────────────", "──────────────────────────────", "─────────────────────────", "─────────────────────")

	for _, rt := range runtimes {
		caps := ""
		if rt.Capabilities.Channels {
			caps += "channels "
		}
		if rt.Capabilities.Auth {
			caps += "auth "
		}
		if rt.Capabilities.Tasks {
			caps += "tasks "
		}
		if rt.Capabilities.Shared {
			caps += "shared "
		}

		marker := ""
		if _, ok := builtinRuntimes[rt.Name]; ok {
			marker = green + " (built-in)" + nc
		}
		fmt.Printf("%-15s %-30s %-25s %s%s\n", rt.Name, rt.Description, rt.DefaultImage, caps, marker)
	}
	return nil
}

func cmdRuntimeShow(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime show <name>")
	}
	paths := resolvePaths()
	name := args[0]

	rt, ok := getRuntimeByName(paths, name)
	if !ok {
		return errorf("runtime '%s' not found", name)
	}

	if hasFlag(args[1:], "--json") {
		data, _ := json.MarshalIndent(rt, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"

	fmt.Printf("%sRuntime: %s%s\n", bold, rt.Name, nc)
	fmt.Printf("  Description:    %s\n", rt.Description)
	fmt.Printf("  Default Image:  %s\n", rt.DefaultImage)
	fmt.Printf("  Gateway:        %s (port %d)\n", rt.GatewayService, rt.InternalPort)
	if rt.HasCLI() {
		fmt.Printf("  CLI:            %s\n", rt.CLIService)
	}
	fmt.Printf("  Health:         %s\n", rt.HealthEndpoint)
	if rt.ReadyEndpoint != "" {
		fmt.Printf("  Ready:          %s\n", rt.ReadyEndpoint)
	}
	fmt.Printf("  Config:         %s\n", rt.ConfigFileName)
	fmt.Printf("  Container:      %s\n", rt.ContainerHome)
	fmt.Println()
	fmt.Printf("  %sCapabilities:%s\n", bold, nc)
	printCap := func(name string, enabled bool) {
		if enabled {
			fmt.Printf("    ✓ %s\n", name)
		} else {
			fmt.Printf("    ✗ %s\n", name)
		}
	}
	printCap("Channels", rt.Capabilities.Channels)
	printCap("Pairing", rt.Capabilities.Pairing)
	printCap("Auth", rt.Capabilities.Auth)
	printCap("Config Merging", rt.Capabilities.Config)
	printCap("Tasks", rt.Capabilities.Tasks)
	printCap("Shared Resources", rt.Capabilities.Shared)
	printCap("Bridge Port", rt.Capabilities.Bridge)

	return nil
}

func cmdRuntimeAdd(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime add <name> --image=<image> [--health=<endpoint>] [--port=<port>] [--gateway-service=<name>]")
	}

	paths := resolvePaths()
	name := args[0]

	// Check not overriding built-in
	if _, ok := builtinRuntimes[name]; ok {
		return errorf("cannot override built-in runtime '%s'", name)
	}

	// Start from base runtime (--from=) or OpenClaw defaults
	var fromName string
	for _, a := range args[1:] {
		if hasPrefix(a, "--from=") {
			fromName = a[7:]
		}
	}

	var rt Runtime
	if fromName != "" {
		base, ok := getRuntimeByName(paths, fromName)
		if !ok {
			return errorf("base runtime '%s' not found — see available: claws runtime list", fromName)
		}
		rt = base
		rt.Name = name
		rt.Description = fmt.Sprintf("Based on %s", fromName)
	} else {
		rt = openclawRuntime()
		rt.Name = name
		rt.Description = "Custom runtime: " + name
	}

	for _, a := range args[1:] {
		switch {
		case hasPrefix(a, "--image="):
			rt.DefaultImage = a[8:]
		case hasPrefix(a, "--health="):
			rt.HealthEndpoint = a[9:]
		case hasPrefix(a, "--ready="):
			rt.ReadyEndpoint = a[8:]
		case hasPrefix(a, "--port="):
			fmt.Sscanf(a[7:], "%d", &rt.InternalPort)
		case hasPrefix(a, "--gateway-service="):
			rt.GatewayService = a[18:]
		case hasPrefix(a, "--cli-service="):
			rt.CLIService = a[14:]
		case hasPrefix(a, "--config-file="):
			rt.ConfigFileName = a[14:]
		case hasPrefix(a, "--container-home="):
			rt.ContainerHome = a[17:]
		case hasPrefix(a, "--description="):
			rt.Description = a[14:]
		case a == "--no-channels":
			rt.Capabilities.Channels = false
			rt.ChannelAddCmd = nil
			rt.ChannelLoginCmd = nil
		case a == "--no-pairing":
			rt.Capabilities.Pairing = false
			rt.PairingCmd = nil
		case a == "--no-auth":
			rt.Capabilities.Auth = false
			rt.AuthCodexCmd = nil
			rt.AuthApiKeyCmd = nil
		case a == "--no-cli":
			rt.CLIService = ""
		case a == "--no-bridge":
			rt.BridgePort = 0
			rt.Capabilities.Bridge = false
		}
	}

	if err := saveRuntimeToFile(paths, rt); err != nil {
		return err
	}

	info(fmt.Sprintf("Runtime '%s' registered.", name))
	fmt.Printf("  Image:   %s\n", rt.DefaultImage)
	fmt.Printf("  Health:  %s\n", rt.HealthEndpoint)
	fmt.Printf("  Config:  %s/%s/%s.json\n", paths.Root, runtimesDir, name)
	fmt.Println()
	fmt.Printf("  Use: claws create <name> --runtime=%s\n", name)
	fmt.Println()
	fmt.Println("  To customize further, edit the JSON file or provide a compose template:")
	fmt.Printf("    cp docker-compose.yml %s/%s/%s-compose.yml\n", paths.Root, runtimesDir, name)
	return nil
}

func cmdRuntimeRemove(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime remove <name>")
	}
	paths := resolvePaths()
	name := args[0]

	if _, ok := builtinRuntimes[name]; ok {
		return errorf("cannot remove built-in runtime '%s'", name)
	}

	rtFile := filepath.Join(paths.Root, runtimesDir, name+".json")
	if _, err := os.Stat(rtFile); err != nil {
		return errorf("runtime '%s' not found", name)
	}

	os.Remove(rtFile)
	// Also remove compose template if it exists
	os.Remove(filepath.Join(paths.Root, runtimesDir, name+"-compose.yml"))

	info(fmt.Sprintf("Runtime '%s' removed.", name))
	return nil
}

// gatewayService returns the gateway service name for an instance.
func gatewayService(paths Paths, name string) string {
	return mustResolveRuntime(paths, name).GatewayService
}

// cliService returns the CLI service name for an instance.
func cliService(paths Paths, name string) string {
	return mustResolveRuntime(paths, name).CLIService
}

// ---------------------------------------------------------------------------
// runtime init — scaffold a new runtime definition
// ---------------------------------------------------------------------------

func cmdRuntimeInit(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime init <name>")
	}
	paths := resolvePaths()
	name := args[0]

	if _, ok := builtinRuntimes[name]; ok {
		return errorf("cannot init built-in runtime '%s'", name)
	}

	rtDir := filepath.Join(paths.Root, runtimesDir)
	os.MkdirAll(rtDir, 0755)
	jsonPath := filepath.Join(rtDir, name+".json")
	composePath := filepath.Join(rtDir, name+"-compose.yml")

	if _, err := os.Stat(jsonPath); err == nil {
		if !hasFlag(args[1:], "--force") {
			return errorf("runtime '%s' already exists at %s. Use --force to overwrite.", name, jsonPath)
		}
	}

	// Create runtime JSON with sensible defaults and the custom name
	rt := Runtime{
		Name:           name,
		Description:    "Custom runtime — edit this file to match your agent",
		DefaultImage:   name + ":latest",
		GatewayService: name + "-gateway",
		CLIService:     "",
		ComposeFile:    name + "-compose.yml",
		InternalPort:   8080,
		BridgePort:     0,
		HealthEndpoint: "/health",
		ReadyEndpoint:  "",
		ContainerHome:      "/app",
		ContainerConfigDir: "/app/config",
		ContainerWorkspace: "/app/workspace",
		MountSkills:    "/app/skills",
		MountWorkspace: "/app/shared",
		MountHooks:     "/app/hooks",
		MountTasks:     "/app/tasks",
		MountOutput:    "/app/output",
		MountManager:   "/app/manager",
		MountWorkers:   "/app/workers",
		SkillsEnvVar:   "",
		ProjectPrefix:  name,
		ConfigFileName: "config.json",
		ConfigFormat:   "json",
		SupportsConfig: false,
		ComposeOverride: "docker-compose.override.yml",
		HealthCheckType:    "http",
		HealthCheckTimeout: 2,
		HealthCheckRetries: 15,
		Capabilities: RuntimeCapabilities{
			Channels: false,
			Pairing:  false,
			Auth:     false,
			Config:   false,
			Tasks:    true,
			Shared:   true,
			Bridge:   false,
		},
	}

	if err := saveRuntimeToFile(paths, rt); err != nil {
		return err
	}

	// Create compose template scaffold
	composeContent := fmt.Sprintf(`# Compose template for %s runtime
# Edit this file to match your agent's requirements.
# Documentation: claws runtime --help
services:
  %s-gateway:
    image: ${OPENCLAW_IMAGE:-%s:latest}
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    ports:
      - "${OPENCLAW_HOST_BIND:-127.0.0.1}:${OPENCLAW_GATEWAY_PORT:-8080}:8080"
    init: true
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: ${OPENCLAW_MEMORY_LIMIT:-2G}
          cpus: "${OPENCLAW_CPU_LIMIT:-2.0}"
    # Edit: your agent's start command
    # command: ["python", "main.py", "--port", "8080"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://127.0.0.1:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 5
      start_period: 20s
`, name, name, name)

	os.WriteFile(composePath, []byte(composeContent), 0644)

	info(fmt.Sprintf("Runtime '%s' scaffolded.", name))
	fmt.Println()
	fmt.Printf("  Runtime definition: %s\n", jsonPath)
	fmt.Printf("  Compose template:  %s\n", composePath)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    1. Edit the runtime JSON to match your agent's settings")
	fmt.Println("    2. Edit the compose template with your agent's start command")
	fmt.Printf("    3. Test it: claws runtime test %s\n", name)
	fmt.Printf("    4. Create an instance: claws create my-agent --runtime=%s\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// runtime test — validate a runtime definition
// ---------------------------------------------------------------------------

func cmdRuntimeTest(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime test <name>")
	}
	paths := resolvePaths()
	name := args[0]

	rt, ok := getRuntimeByName(paths, name)
	if !ok {
		return errorf("runtime '%s' not found — register it first: claws runtime add %s --image=...", name, name)
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"

	fmt.Printf("%sTesting runtime '%s'%s\n\n", bold, name, nc)

	// 1. Check image exists
	fmt.Printf("  Checking image '%s'... ", rt.DefaultImage)
	if err := exec.Command("docker", "image", "inspect", rt.DefaultImage).Run(); err != nil {
		fmt.Printf("%snot found%s\n", red, nc)
		fmt.Printf("    Pull it: docker pull %s\n", rt.DefaultImage)
		fmt.Printf("    Or build it locally\n")
		return errorf("image '%s' not found", rt.DefaultImage)
	}
	fmt.Printf("%sfound%s\n", green, nc)

	// 2. Check compose template exists
	composePath := rt.ComposeTemplatePath(paths)
	fmt.Printf("  Checking compose template... ")
	if _, err := os.Stat(composePath); err != nil {
		fmt.Printf("%snot found%s\n", red, nc)
		fmt.Printf("    Expected: %s\n", composePath)
		fmt.Printf("    Create it: claws runtime init %s\n", name)
		return errorf("compose template not found at %s", composePath)
	}
	fmt.Printf("%sfound%s (%s)\n", green, nc, composePath)

	// 3. Check gateway service exists in compose
	fmt.Printf("  Checking service '%s' in compose... ", rt.GatewayService)
	// Read compose file and check for service name
	composeData, _ := os.ReadFile(composePath)
	if composeData != nil && !contains(string(composeData), rt.GatewayService+":") {
		fmt.Printf("%snot found%s\n", red, nc)
		fmt.Printf("    The compose template doesn't define a service named '%s'\n", rt.GatewayService)
		return errorf("service '%s' not found in compose template", rt.GatewayService)
	}
	fmt.Printf("%sfound%s\n", green, nc)

	// 4. Check health endpoint (if image is running, try it)
	fmt.Printf("  Health endpoint: %s\n", rt.HealthEndpoint)
	fmt.Printf("  Config file: %s\n", rt.ConfigFileName)
	fmt.Printf("  Internal port: %d\n", rt.InternalPort)

	fmt.Println()
	info(fmt.Sprintf("Runtime '%s' looks valid. Create an instance: claws create <name> --runtime=%s", name, name))
	return nil
}

func contains(s, sub string) bool {
	return indexOfStr(s, sub) >= 0
}

// ---------------------------------------------------------------------------
// runtime export/import — share runtime definitions
// ---------------------------------------------------------------------------

func cmdRuntimeExport(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime export <name>")
	}
	paths := resolvePaths()
	name := args[0]

	rt, ok := getRuntimeByName(paths, name)
	if !ok {
		return errorf("runtime '%s' not found", name)
	}

	// Build export bundle — runtime JSON + optional compose template
	type exportBundle struct {
		Runtime         Runtime `json:"runtime"`
		ComposeTemplate string  `json:"composeTemplate,omitempty"`
	}

	bundle := exportBundle{Runtime: rt}

	// Include compose template if it exists
	composePath := filepath.Join(paths.Root, runtimesDir, name+"-compose.yml")
	if data, err := os.ReadFile(composePath); err == nil {
		bundle.ComposeTemplate = string(data)
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func cmdRuntimeImport(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime import <file.json>")
	}
	paths := resolvePaths()
	filePath := args[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return errorf("cannot read '%s': %v", filePath, err)
	}

	// Try bundle format first
	type exportBundle struct {
		Runtime         Runtime `json:"runtime"`
		ComposeTemplate string  `json:"composeTemplate,omitempty"`
	}
	var bundle exportBundle
	if err := json.Unmarshal(data, &bundle); err != nil || bundle.Runtime.Name == "" {
		// Try plain runtime format
		var rt Runtime
		if err := json.Unmarshal(data, &rt); err != nil {
			return errorf("invalid runtime file: %v", err)
		}
		bundle.Runtime = rt
	}

	rt := bundle.Runtime
	if rt.Name == "" {
		return errorf("runtime file missing 'name' field")
	}

	if _, ok := builtinRuntimes[rt.Name]; ok {
		return errorf("cannot import over built-in runtime '%s'", rt.Name)
	}

	if err := saveRuntimeToFile(paths, rt); err != nil {
		return err
	}

	// Save compose template if included
	if bundle.ComposeTemplate != "" {
		composePath := filepath.Join(paths.Root, runtimesDir, rt.Name+"-compose.yml")
		os.WriteFile(composePath, []byte(bundle.ComposeTemplate), 0644)
		info(fmt.Sprintf("Imported runtime '%s' with compose template.", rt.Name))
	} else {
		info(fmt.Sprintf("Imported runtime '%s'.", rt.Name))
	}

	fmt.Printf("  Image: %s\n", rt.DefaultImage)
	fmt.Printf("  Use: claws create <name> --runtime=%s\n", rt.Name)
	return nil
}

// ---------------------------------------------------------------------------
// runtime detect — auto-detect settings from Docker image
// ---------------------------------------------------------------------------

func cmdRuntimeDetect(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws runtime detect <image:tag>")
	}
	image := args[0]

	bold := "\033[1m"
	nc := "\033[0m"

	fmt.Printf("%sInspecting %s%s\n\n", bold, image, nc)

	// Check image exists
	if err := exec.Command("docker", "image", "inspect", image).Run(); err != nil {
		return errorf("image '%s' not found — pull it first: docker pull %s", image, image)
	}

	// Inspect image
	type imageInfo struct {
		Config struct {
			ExposedPorts map[string]struct{} `json:"ExposedPorts"`
			User         string              `json:"User"`
			Entrypoint   []string            `json:"Entrypoint"`
			Cmd          []string            `json:"Cmd"`
			Healthcheck  *struct {
				Test []string `json:"Test"`
			} `json:"Healthcheck"`
		} `json:"Config"`
	}

	out, err := exec.Command("docker", "image", "inspect", image).Output()
	if err != nil {
		return errorf("failed to inspect image: %v", err)
	}

	var images []imageInfo
	json.Unmarshal(out, &images)
	if len(images) == 0 {
		return errorf("could not parse image metadata")
	}
	img := images[0]

	// Extract useful info
	var ports []string
	for p := range img.Config.ExposedPorts {
		ports = append(ports, p)
	}

	user := img.Config.User
	if user == "" {
		user = "root (default)"
	}

	entrypoint := ""
	if len(img.Config.Entrypoint) > 0 {
		entrypoint = fmt.Sprintf("%v", img.Config.Entrypoint)
	}
	cmd := ""
	if len(img.Config.Cmd) > 0 {
		cmd = fmt.Sprintf("%v", img.Config.Cmd)
	}

	fmt.Printf("  Exposed ports:  %v\n", ports)
	fmt.Printf("  User:           %s\n", user)
	if entrypoint != "" {
		fmt.Printf("  Entrypoint:     %s\n", entrypoint)
	}
	if cmd != "" {
		fmt.Printf("  Cmd:            %s\n", cmd)
	}
	if img.Config.Healthcheck != nil {
		fmt.Printf("  Healthcheck:    %v\n", img.Config.Healthcheck.Test)
	}

	// Detect if it's OpenClaw-compatible
	isNode := false
	if entrypoint != "" && contains(entrypoint, "node") {
		isNode = true
	}

	// Suggest a name
	name := image
	if idx := indexOfStr(name, ":"); idx >= 0 {
		name = name[:idx]
	}
	if idx := indexOfStr(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	fmt.Println()
	if isNode && contains(entrypoint, "dist/index.js") {
		fmt.Printf("  %sDetected: OpenClaw-compatible (Node.js gateway)%s\n", bold, nc)
		fmt.Println()
		fmt.Println("  This image looks OpenClaw-compatible. You can probably just use --image=:")
		fmt.Printf("    claws create my-agent --image=%s\n", image)
		fmt.Println()
		fmt.Println("  Or if it has differences (health endpoint, port):")
		fmt.Printf("    claws runtime add %s --from=openclaw --image=%s\n", name, image)
	} else {
		fmt.Printf("  %sDetected: Custom runtime%s\n", bold, nc)
		fmt.Println()

		// Guess port
		port := "8080"
		if len(ports) > 0 {
			port = ports[0]
			// Strip /tcp suffix
			if idx := indexOfStr(port, "/"); idx >= 0 {
				port = port[:idx]
			}
		}

		// Guess home dir
		home := "/app"
		if user == "node" || user == "1000" {
			home = "/home/node"
		}

		fmt.Println("  Suggested command:")
		fmt.Printf("    claws runtime add %s --image=%s --port=%s --health=/health --container-home=%s --no-channels --no-cli\n",
			name, image, port, home)
		fmt.Println()
		fmt.Println("  Or scaffold and customize:")
		fmt.Printf("    claws runtime init %s\n", name)
	}

	return nil
}

// hasPrefix is a simple helper (strings.HasPrefix but shorter for flag parsing).
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
