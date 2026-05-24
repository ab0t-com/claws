package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// create
// ---------------------------------------------------------------------------

func cmdCreate(args []string) error {
	paths := resolvePaths()
	var nameArg, fromInstance, role, managerName, bindMode, runtimeName string
	var inlineAuth, inlineTelegram, inlineDiscord, inlineSlackBot, inlineSlackApp string
	shared := SharedFlags{}
	noShared := false

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--from="):
			fromInstance = a[7:]
		case strings.HasPrefix(a, "--role="):
			role = a[7:]
		case strings.HasPrefix(a, "--manager="):
			managerName = a[10:]
		case strings.HasPrefix(a, "--bind="):
			bindMode = a[7:]
		case strings.HasPrefix(a, "--image="):
			// handled below after policy check
		case strings.HasPrefix(a, "--runtime="):
			runtimeName = a[10:]
		case strings.HasPrefix(a, "--auth="):
			inlineAuth = a[7:]
		case strings.HasPrefix(a, "--telegram="):
			inlineTelegram = a[11:]
		case strings.HasPrefix(a, "--discord="):
			inlineDiscord = a[10:]
		case strings.HasPrefix(a, "--slack-bot="):
			inlineSlackBot = a[12:]
		case strings.HasPrefix(a, "--slack-app="):
			inlineSlackApp = a[12:]
		case a == "--shared-skills" || a == "--skills":
			shared.Skills = true
		case a == "--shared-workspace" || a == "--workspace":
			shared.Workspace = true
		case a == "--shared-hooks" || a == "--hooks":
			shared.Hooks = true
		case a == "--shared":
			shared.Skills = true
			shared.Workspace = true
		case a == "--no-shared-workspace":
			noShared = true
		case strings.HasPrefix(a, "-"):
			return errorf("unknown flag: %s", a)
		default:
			nameArg = a
		}
	}

	// Default bind: loopback (secure). Use --bind=lan for network access.
	if bindMode == "" {
		bindMode = "loopback"
	}
	if bindMode != "loopback" && bindMode != "lan" && bindMode != "wan" {
		return errorf("invalid bind mode '%s' — use 'loopback', 'lan', or 'wan'", bindMode)
	}
	// Resolve runtime
	if runtimeName == "" {
		runtimeName = "openclaw"
	}
	rt, ok := getRuntimeByName(paths, runtimeName)
	if !ok {
		return errorf("unknown runtime '%s' — see available: claws runtime list", runtimeName)
	}

	if nameArg == "" {
		return errorf("usage: claws create <name|group/name> [--from=<instance>] [--role=manager|worker] [--runtime=openclaw] [--shared-*]")
	}

	// Parse name — could be "bob" or "backend/bob"
	ref, err := ParseRef(nameArg)
	if err != nil {
		return err
	}

	// If grouped, verify group exists
	if ref.Group != "" {
		groupDir := ref.GroupDir(paths)
		if !IsGroup(groupDir) {
			return errorf("group '%s' does not exist — create it first: claws group create %s", ref.Group, ref.Group)
		}
		// Grouped instances share group resources by default (unless --no-shared-workspace)
		if !noShared && !shared.Any() {
			shared.Skills = true
			shared.Workspace = true
		}
	}

	// Validate role
	if role != "" && role != "manager" && role != "worker" {
		return errorf("invalid role '%s' — use 'manager' or 'worker'", role)
	}
	if role == "worker" && ref.Group == "" {
		return errorf("workers must be in a group — use: claws create <group>/%s --role=worker", ref.Name)
	}
	if role == "manager" && ref.Group == "" {
		return errorf("managers must be in a group — use: claws create <group>/%s --role=manager", ref.Name)
	}

	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")

	if _, err := os.Stat(envFile); err == nil {
		return errorf("instance '%s' already exists", ref.FullName())
	}

	// Validate --from
	if fromInstance != "" {
		fromRef, _ := ParseRef(fromInstance)
		fromDir := fromRef.Dir(paths)
		if _, err := os.Stat(filepath.Join(fromDir, rt.ConfigFileName)); err != nil {
			return errorf("template instance '%s' not found", fromInstance)
		}
	}

	name := ref.FullName()

	// Resolve image: --image= flag > env var > runtime default
	image := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--image=") {
			image = a[8:]
		}
	}
	if image == "" {
		image = os.Getenv("OPENCLAW_IMAGE")
	}
	if image == "" {
		image = rt.DefaultImage
	}

	// Policy enforcement
	policy := readPolicy(paths)
	if err := policy.enforceBindPolicy(bindMode); err != nil {
		return err
	}
	if err := policy.enforceImagePolicy(image); err != nil {
		return err
	}

	// Resource check (policy max overrides default)
	count := instanceCount(paths)
	if err := policy.enforceMaxInstances(count); err != nil {
		return err
	}
	if count >= maxInstances {
		return errorf("maximum %d instances reached — remove one first", maxInstances)
	}
	if count >= warnInstances {
		warn(fmt.Sprintf("you have %d instances — RAM may be tight", count))
	}

	// Allocate port (atomic: find next index + register under lock)
	index, err := lockedAllocatePort(paths, ref.RegistryName())
	if err != nil {
		return err
	}
	gatewayPort := portForIndex(index)
	bridgePort := gatewayPort + 1

	// Check port not in use
	if portInUse(gatewayPort) {
		lockedUnregisterPort(paths, ref.RegistryName())
		return errorf("port %d is already in use", gatewayPort)
	}

	token := generateToken()

	if !quietCreate {
		info(fmt.Sprintf("Creating instance '%s'...", name))
		fmt.Printf("  Index:     %d\n", index)
		fmt.Printf("  Gateway:   :%d\n", gatewayPort)
		fmt.Printf("  Bridge:    :%d\n", bridgePort)
		fmt.Printf("  Directory: %s\n", dir)
		if ref.Group != "" {
			fmt.Printf("  Group:     %s\n", ref.Group)
		}
		if role != "" {
			fmt.Printf("  Role:      %s\n", role)
		}
		if fromInstance != "" {
			fmt.Printf("  Template:  %s\n", fromInstance)
		}
		fmt.Println()
	}

	// Create dirs
	for _, sub := range []string{"credentials", "agents", "identity", "workspace", "sessions", "canvas", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			cleanup(dir, paths, name)
			return err
		}
	}

	// v1.6 — agent UUID generated at create time. Stable for the agent's
	// lifetime; cross-system references (intent-gateway, sharedwatch, audit
	// chain) can use this rather than the renameable name.
	instanceUUID := randomUUIDv4()

	// Write instance.env (image already resolved above for policy check)
	envContent := fmt.Sprintf(`# Instance: %s
# Created: %s
# Port index: %d

INSTANCE_NAME=%s
CLAWS_INSTANCE_UUID=%s
OPENCLAW_CONFIG_DIR=%s
OPENCLAW_WORKSPACE_DIR=%s/workspace
OPENCLAW_GATEWAY_PORT=%d
OPENCLAW_BRIDGE_PORT=%d
OPENCLAW_GATEWAY_TOKEN=%s
OPENCLAW_GATEWAY_BIND=%s
OPENCLAW_HOST_BIND=%s
OPENCLAW_IMAGE=%s
CLAWS_RUNTIME=%s
OPENCLAW_ALLOW_INSECURE_PRIVATE_WS=
CLAUDE_AI_SESSION_KEY=
CLAUDE_WEB_SESSION_KEY=
CLAUDE_WEB_COOKIE=
`, name, time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), index,
		name, instanceUUID, dir, dir, gatewayPort, bridgePort, token, bindMode, hostBind(bindMode), image, runtimeName)

	if err := os.WriteFile(envFile, []byte(envContent), credentialFileMode); err != nil {
		cleanup(dir, paths, name)
		return err
	}

	// Build config via config merge
	skeleton := map[string]any{
		"gateway": map[string]any{
			"port": float64(gatewayPort),
			"mode": "local",
			"bind": "lan",
			"auth": map[string]any{
				"mode":  "token",
				"token": token,
			},
		},
	}
	skeletonFile := filepath.Join(dir, ".skeleton.json")
	skeletonData, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		cleanup(dir, paths, name)
		return errorf("failed to marshal skeleton config: %v", err)
	}
	if err := os.WriteFile(skeletonFile, skeletonData, 0600); err != nil {
		cleanup(dir, paths, name)
		return errorf("failed to write skeleton config: %v", err)
	}
	defer os.Remove(skeletonFile)

	var fromConfigPath string
	if fromInstance != "" {
		fromRef, _ := ParseRef(fromInstance)
		fromConfigPath = filepath.Join(fromRef.Dir(paths), rt.ConfigFileName)
	}

	// Config inheritance: global defaults → group defaults → template → skeleton
	defaultsPath := filepath.Join(paths.Root, "defaults.json")

	// If grouped, merge group defaults between global and template
	groupDefaultsPath := ""
	if ref.Group != "" {
		gdp := filepath.Join(ref.GroupDir(paths), "defaults.json")
		if _, err := os.Stat(gdp); err == nil {
			groupDefaultsPath = gdp
		}
	}
	outputPath := filepath.Join(dir, rt.ConfigFileName)

	if !rt.SupportsConfig {
		// Runtime doesn't use config merging — write skeleton directly
		if err := os.WriteFile(outputPath, skeletonData, 0600); err != nil {
			cleanup(dir, paths, name)
			return err
		}
	} else if err := mergeConfigLayers(defaultsPath, groupDefaultsPath, fromConfigPath, fromInstance, skeletonFile, outputPath); err != nil {
		cleanup(dir, paths, name)
		return err
	}

	// Default tool profile: set tools.profile = "coding" if not already set
	// Sandbox warning: warn if sandbox is not enabled
	if rt.SupportsConfig {
		if cfg, err := readInstanceConfig(outputPath); err == nil {
			if getNestedConfig(cfg, "tools.profile") == nil {
				setNestedConfig(cfg, "tools.profile", "coding")
				writeInstanceConfig(outputPath, cfg)
			}
			if getNestedConfig(cfg, "agents.defaults.sandbox") == nil && !quietCreate {
				warn("sandbox is not enabled — agent can read/write files and run commands freely")
				fmt.Printf("  Enable: claws config set %s agents.defaults.sandbox true\n", name)
			}
		}
	}

	// Role
	if role != "" {
		f, _ := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY, credentialFileMode)
		fmt.Fprintf(f, "INSTANCE_ROLE=%s\n", role)
		if managerName != "" {
			fmt.Fprintf(f, "INSTANCE_MANAGER=%s\n", managerName)
		}
		f.Close()
	}

	// Shared flags
	if shared.Any() {
		writeSharedFlags(envFile, shared)
	}

	// Build compose override (handles shared + group + role mounts)
	if ref.Group != "" {
		rebuildGroupOverride(paths, ref)
	} else if shared.Any() {
		rebuildOverride(paths, ref.Name)
	}

	// Port already registered by lockedAllocatePort above

	// Validate compose (unless skipped)
	if os.Getenv("CLAWS_SKIP_VALIDATE") == "" {
		if _, err := dcOutput(paths, ref.RegistryName(), "config"); err != nil {
			cleanup(dir, paths, ref.RegistryName())
			return errorf("compose config validation failed")
		}
	}

	info(fmt.Sprintf("Instance '%s' created.", name))
	if !quietCreate {
		if _, err := os.Stat(defaultsPath); err == nil {
			fmt.Println("  (merged global defaults)")
		}
		if groupDefaultsPath != "" {
			fmt.Printf("  (merged group defaults from %s)\n", ref.Group)
		}
		if fromInstance != "" {
			fmt.Printf("  (config copied from %s)\n", fromInstance)
		}
		if role != "" {
			fmt.Printf("  (role: %s)\n", role)
		}
	}
	// Inline auth chaining (--auth=codex or --auth=apikey)
	if inlineAuth == "codex" {
		fmt.Println()
		info(fmt.Sprintf("Running auth for '%s'...", name))
		if err := cmdAuth([]string{name, "codex"}); err != nil {
			warn(fmt.Sprintf("Auth failed: %v — retry: claws auth %s codex", err, name))
		}
	} else if inlineAuth == "apikey" {
		fmt.Println()
		warn(fmt.Sprintf("API key auth requires provider and key — run: claws auth %s apikey <provider> <key>", name))
	} else if inlineAuth != "" {
		fmt.Println()
		warn(fmt.Sprintf("unknown auth mode '%s' — use 'codex' or 'apikey'", inlineAuth))
	}

	// Inline channel chaining (--telegram=TOKEN, --discord=TOKEN, --slack-bot=TOKEN)
	if inlineTelegram != "" {
		fmt.Println()
		info(fmt.Sprintf("Adding Telegram channel to '%s'...", name))
		if err := cmdChannel([]string{"add", name, "telegram", "--token=" + inlineTelegram}); err != nil {
			warn(fmt.Sprintf("Telegram failed: %v — retry: claws channel add %s telegram --token=...", err, name))
		}
	}
	if inlineDiscord != "" {
		fmt.Println()
		info(fmt.Sprintf("Adding Discord channel to '%s'...", name))
		if err := cmdChannel([]string{"add", name, "discord", "--token=" + inlineDiscord}); err != nil {
			warn(fmt.Sprintf("Discord failed: %v — retry: claws channel add %s discord --token=...", err, name))
		}
	}
	if inlineSlackBot != "" {
		fmt.Println()
		info(fmt.Sprintf("Adding Slack channel to '%s'...", name))
		slackArgs := []string{"add", name, "slack", "--bot-token=" + inlineSlackBot}
		if inlineSlackApp != "" {
			slackArgs = append(slackArgs, "--app-token="+inlineSlackApp)
		}
		if err := cmdChannel(slackArgs); err != nil {
			warn(fmt.Sprintf("Slack failed: %v — retry: claws channel add %s slack ...", err, name))
		}
	}

	if !quietCreate {
		fmt.Println()
		if inlineAuth == "" && inlineTelegram == "" && inlineDiscord == "" && inlineSlackBot == "" {
			fmt.Println("  Next steps:")
			fmt.Printf("    claws auth %s codex              # add OpenAI Codex auth\n", name)
			fmt.Printf("    claws auth %s apikey openai <key> # or add an API key\n", name)
			fmt.Printf("    claws start %s                    # start the instance\n", name)
		} else {
			fmt.Println("  Next step:")
			fmt.Printf("    claws start %s                    # start the instance\n", name)
		}
		fmt.Println()
		if bindMode == "loopback" {
			fmt.Println("  SSH tunnel:")
			fmt.Printf("    ssh -N -L %d:127.0.0.1:%d ubuntu@<server>\n", gatewayPort, gatewayPort)
		}
	}
	return nil
}

func cleanup(dir string, paths Paths, name string) {
	os.RemoveAll(dir)
	lockedUnregisterPort(paths, name)
}

// portInUse reports whether 127.0.0.1:<port> is currently bound by any
// process. Uses a TCP dial with a tight timeout — much cheaper than
// shelling to `ss`, and works without any privileges.
//
// CLAWS_SKIP_VALIDATE=1 forces "free" (test mode), matching the old
// behaviour so existing tests that skip port checks don't break.
func portInUse(port int) bool {
	if os.Getenv("CLAWS_SKIP_VALIDATE") != "" {
		return false
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false // connection refused / timeout → nothing listening
	}
	_ = conn.Close()
	return true
}

// identifyPortHolder asks Docker which container (if any) publishes the
// given host port. Returns the container name + a hint about whether
// it looks like an openclaw container that's NOT in the claws registry
// (i.e. an orphan). Returns ("", false, false) when Docker isn't reachable
// or no container is publishing the port. The third return is true when
// the holder name matches the expected name for `ownerName` — meaning the
// caller's own already-running container, not a foreign holder.
func identifyPortHolder(port int, ownerName string) (containerName string, isOrphan bool, isOwn bool) {
	// `docker ps --filter publish=<port>` lists containers publishing
	// that host port. We grab name + the project label so we can decide
	// whether it's an orphan.
	cmd := exec.Command("docker", "ps", "-a",
		"--filter", fmt.Sprintf("publish=%d", port),
		"--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return "", false, false
	}
	containerName = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])

	// Is it OUR container? Format: openclaw-<group>-<name>-openclaw-gateway-1
	// (compose normalises slashes in the project name to dashes).
	if ownerName != "" {
		expected := "openclaw-" + strings.ReplaceAll(ownerName, "/", "-") + "-openclaw-gateway-1"
		if containerName == expected {
			return containerName, false, true
		}
	}

	// Orphan check: matches our naming convention (openclaw-* + -gateway-1)
	// but doesn't correspond to a registered instance.
	if strings.HasPrefix(containerName, "openclaw-") && strings.HasSuffix(containerName, "-openclaw-gateway-1") {
		isOrphan = true
	}
	return containerName, isOrphan, false
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

func cmdStart(args []string) error {
	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	name := firstPositional(args)
	if filterGroup != "" && name != "" {
		return errorf("specify either positional instance name or --group=, not both")
	}
	if filterGroup != "" {
		// Group fan-out. No confirmation: start is additive, not destructive.
		return runOnGroup(paths, filterGroup, "Starting", cmdStart, args)
	}
	if name == "" {
		return errorf("usage: claws start <name> | claws start --group=<name>")
	}
	// If the name doesn't match an instance, check whether it matches a
	// team (group). `claws start team` is what most operators reach for —
	// they don't want to learn `--group=`. Fan out automatically with a
	// 3-second countdown so an accidental typo isn't irreversible.
	if err := requireInstance(paths, name); err != nil {
		entries, _ := readRegistry(paths)
		members := filterEntriesByGroup(entries, name)
		if len(members) > 0 {
			fmt.Printf("'%s' is a team with %d agent(s). Starting all in 3 seconds — Ctrl-C to cancel.\n",
				name, len(members))
			for _, m := range members {
				fmt.Printf("  • %s\n", m.Name)
			}
			countdown(3)
			return runOnGroup(paths, name, "Starting", cmdStart, args)
		}
		return err
	}

	// Pre-flight port check. The agent's assigned port may be held by a
	// foreign container (typically an orphan from a prior test workspace).
	// Without this check we'd hand control to `docker compose up` which
	// surfaces the conflict as a cryptic Docker error mid-startup; the
	// agent ends up in a half-broken state and the operator has to read
	// the docker daemon error to figure out what happened.
	//
	// Identify the holder and surface a precise next-step instead. Never
	// auto-kill — the operator decides what to do.
	port := readEnvValue(filepath.Join(instanceDir(paths, name), "instance.env"), "OPENCLAW_GATEWAY_PORT")
	portNum, atoiErr := strconv.Atoi(port)
	if atoiErr == nil && portInUse(portNum) {
		holder, isOrphan, isOwn := identifyPortHolder(portNum, name)
		if !isOwn {
			switch {
			case isOrphan:
				return errorf("port %d is held by orphan container '%s' (not in claws registry).\n  Remove it: claws orphans clean %s\n  See all:    claws orphans", portNum, holder, holder)
			case holder != "":
				return errorf("port %d is held by container '%s' (another claws agent — port allocation drift).\n  Diagnose: claws drift", portNum, holder)
			default:
				return errorf("port %d is held by a non-Docker process.\n  Investigate: ss -tlnp | grep :%d\n  Won't auto-kill; remove or reconfigure the holder, then retry.", portNum, portNum)
			}
		}
		// isOwn: our own container is already up. `docker compose up -d`
		// is idempotent, so falling through is safe.
	}

	info(fmt.Sprintf("Starting instance '%s'...", name))
	if err := dcRun(paths, name, "up", "-d", gatewayService(paths, name)); err != nil {
		return err
	}

	// Wait for health
	rt := mustResolveRuntime(paths, name)
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, rt.HealthEndpoint)

	fmt.Println()
	info("Waiting for health...")
	healthy := false
	for i := 0; i < 15; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			info(fmt.Sprintf("Instance '%s' is healthy on :%s", name, port))
			healthy = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	if !healthy {
		warn("Health check didn't pass in 30s — check: claws logs " + name)
	}
	// "Next:" hints — same suggestions whether health passed or not (in
	// both cases the operator wants ping + logs; the hint provider keys
	// off AgentName, not the outcome).
	ctx := hintsCtxCheap(paths)
	ctx.AgentName = name
	if healthy {
		ctx.AgentStatus = "healthy"
	}
	hintsRender("start", ctx)
	return nil
}

// ---------------------------------------------------------------------------
// stop
// ---------------------------------------------------------------------------

func cmdStop(args []string) error {
	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	name := firstPositional(args)
	if filterGroup != "" && name != "" {
		return errorf("specify either positional instance name or --group=, not both")
	}
	if filterGroup != "" {
		entries, _ := readRegistry(paths)
		members := filterEntriesByGroup(entries, filterGroup)
		if len(members) == 0 {
			info(fmt.Sprintf("No instances in group '%s'.", filterGroup))
			return nil
		}
		if !confirmGroupOp("stop", filterGroup, len(members), hasFlag(args, "--yes")) {
			return nil
		}
		return runOnGroup(paths, filterGroup, "Stopping", cmdStop, args)
	}
	if name == "" {
		return errorf("usage: claws stop <name> | claws stop --group=<name> [--yes]")
	}
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	info(fmt.Sprintf("Stopping instance '%s'...", name))
	if err := dcRun(paths, name, "stop"); err != nil {
		return err
	}
	info(fmt.Sprintf("Instance '%s' stopped.", name))
	return nil
}

// ---------------------------------------------------------------------------
// restart
// ---------------------------------------------------------------------------

func cmdRestart(args []string) error {
	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	name := firstPositional(args)
	if filterGroup != "" && name != "" {
		return errorf("specify either positional instance name or --group=, not both")
	}
	if filterGroup != "" {
		entries, _ := readRegistry(paths)
		members := filterEntriesByGroup(entries, filterGroup)
		if len(members) == 0 {
			info(fmt.Sprintf("No instances in group '%s'.", filterGroup))
			return nil
		}
		if !confirmGroupOp("restart", filterGroup, len(members), hasFlag(args, "--yes")) {
			return nil
		}
		return runOnGroup(paths, filterGroup, "Restarting", cmdRestart, args)
	}
	if name == "" {
		return errorf("usage: claws restart <name> [--hard] | claws restart --group=<name> [--yes]")
	}
	hard := hasFlag(args, "--hard")
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	if hard {
		// --hard: tear down and recreate the container (picks up compose template changes)
		info(fmt.Sprintf("Hard-restarting instance '%s' (recreating container)...", name))
		dcRun(paths, name, "down")
		if err := dcRun(paths, name, "up", "-d", gatewayService(paths, name)); err != nil {
			return err
		}
	} else {
		// Normal: just restart the process inside the existing container
		info(fmt.Sprintf("Restarting instance '%s'...", name))
		if err := dcRun(paths, name, "restart", gatewayService(paths, name)); err != nil {
			return err
		}
	}

	info(fmt.Sprintf("Instance '%s' restarted.", name))
	return nil
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func cmdList(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	rich := hasFlag(args, "--rich") || hasFlag(args, "--wide")
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 {
		if jsonMode {
			fmt.Println("[]")
		} else if filterGroup != "" {
			fmt.Printf("No instances found in group '%s'.\n", filterGroup)
		} else {
			fmt.Println("No instances found.")
		}
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	// Rich mode: model + role + channels per agent. Reads instance.env +
	// openclaw.json for every entry. Cost: same order as the basic list
	// (already shells docker compose ps per instance for status).
	if rich {
		type richEntry struct {
			Name     string   `json:"name"`
			Group    string   `json:"group,omitempty"`
			Port     string   `json:"port"`
			Status   string   `json:"status"`
			Model    string   `json:"model"`
			Role     string   `json:"role,omitempty"`
			Channels []string `json:"channels,omitempty"`
			Image    string   `json:"image"`
			RAM      string   `json:"ram"`
			Uptime   string   `json:"uptime"`
		}
		var jsonEntries []richEntry

		// Column widths (visible chars). Wide enough for typical fleet shapes:
		// NAME up to ~18 (group/name combos), MODEL up to ~26 (provider/model).
		const (
			wName     = 18
			wPort     = 8
			wStatus   = 9
			wModel    = 26
			wRole     = 8
			wChannels = 18
			wRAM      = 9
		)

		if !jsonMode {
			fmt.Print(bold)
			fmt.Print(padVisible("NAME", wName))
			fmt.Print(padVisible(" PORT", wPort+1))
			fmt.Print(padVisible(" STATUS", wStatus+1))
			fmt.Print(padVisible(" MODEL", wModel+1))
			fmt.Print(padVisible(" ROLE", wRole+1))
			fmt.Print(padVisible(" CHANNELS", wChannels+1))
			fmt.Print(padVisible(" RAM", wRAM+1))
			fmt.Print(" UPTIME")
			fmt.Println(nc)
			fmt.Print(padVisible(strings.Repeat("─", wName), wName))
			fmt.Print(padVisible(" "+strings.Repeat("─", wPort), wPort+1))
			fmt.Print(padVisible(" "+strings.Repeat("─", wStatus), wStatus+1))
			fmt.Print(padVisible(" "+strings.Repeat("─", wModel), wModel+1))
			fmt.Print(padVisible(" "+strings.Repeat("─", wRole), wRole+1))
			fmt.Print(padVisible(" "+strings.Repeat("─", wChannels), wChannels+1))
			fmt.Print(padVisible(" "+strings.Repeat("─", wRAM), wRAM+1))
			fmt.Println(" " + strings.Repeat("─", 10))
		}

		for _, e := range entries {
			info := gatherRichInfo(paths, e.Name)

			if jsonMode {
				jsonEntries = append(jsonEntries, richEntry{
					Name: info.Name, Group: info.Group, Port: info.Port,
					Status: info.Status, Model: info.Model, Role: info.Role,
					Channels: info.Channels, Image: info.Image,
					RAM: info.RAM, Uptime: info.Uptime,
				})
				continue
			}

			// Color the status; padVisible accounts for the invisible chars.
			var statusColored string
			switch info.Status {
			case "healthy":
				statusColored = green + info.Status + nc
			case "starting":
				statusColored = yellow + info.Status + nc
			case "stopped":
				statusColored = red + info.Status + nc
			default:
				statusColored = info.Status
			}

			channels := "—"
			if len(info.Channels) > 0 {
				channels = strings.Join(info.Channels, ",")
			}

			fmt.Print(padVisible(info.Name, wName))
			fmt.Print(" " + padVisible(":"+info.Port, wPort))
			fmt.Print(" " + padVisible(statusColored, wStatus))
			fmt.Print(" " + padVisible(truncate(orDash(info.Model), wModel), wModel))
			fmt.Print(" " + padVisible(truncate(orDash(info.Role), wRole), wRole))
			fmt.Print(" " + padVisible(truncate(channels, wChannels), wChannels))
			fmt.Print(" " + padVisible(truncate(orDash(info.RAM), wRAM), wRAM))
			fmt.Println(" " + orDash(info.Uptime))
		}

		if jsonMode {
			data, _ := json.MarshalIndent(jsonEntries, "", "  ")
			fmt.Println(string(data))
		}
		return nil
	}

	// Default (non-rich) view — backward compatible with pre-A1 output.
	type listEntry struct {
		Name   string `json:"name"`
		Port   string `json:"port"`
		Status string `json:"status"`
		RAM    string `json:"ram"`
		Uptime string `json:"uptime"`
	}
	var jsonEntries []listEntry

	if !jsonMode {
		fmt.Printf("%s%-18s %-8s %-12s %-10s %-10s %s%s\n", bold, "NAME", "PORT", "STATUS", "RAM", "UPTIME", "NEXT", nc)
		fmt.Printf("%-18s %-8s %-12s %-10s %-10s %s\n", "──────────────────", "────────", "────────────", "──────────", "──────────", "────")
	}

	// Collected statuses for the trailing hints provider (avoids re-running
	// docker compose ps just to populate hints.Context).
	statuses := map[string]string{}

	for _, e := range entries {
		dir := instanceDir(paths, e.Name)
		envFile := filepath.Join(dir, "instance.env")
		port := "—"
		status := "missing"
		statusPlain := "missing"
		ram := "—"
		uptime := "—"

		if _, err := os.Stat(envFile); err == nil {
			port = readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
			cs := containerStatus(paths, e.Name)
			if strings.Contains(cs, "Up") {
				if strings.Contains(cs, "healthy") {
					status = green + "healthy" + nc
					statusPlain = "healthy"
				} else {
					status = yellow + "starting" + nc
					statusPlain = "starting"
				}
				ram = containerRAM(paths, e.Name)
				uptime = cs
				uptime = strings.Replace(uptime, "Up ", "", 1)
				if idx := strings.Index(uptime, " ("); idx >= 0 {
					uptime = uptime[:idx]
				}
			} else if cs != "" {
				status = red + "stopped" + nc
				statusPlain = "stopped"
			} else {
				status = "created"
				statusPlain = "created"
			}
		}
		statuses[e.Name] = statusPlain

		if jsonMode {
			jsonEntries = append(jsonEntries, listEntry{
				Name: e.Name, Port: port, Status: statusPlain, RAM: ram, Uptime: uptime,
			})
		} else {
			// NEXT column: one terse, copy-pasteable command per row.
			// Computed inline (cheap; same logic as the hint provider
			// but rendered per-row instead of aggregated).
			next := perRowNext(e.Name, statusPlain)
			fmt.Printf("%-18s :%-7s %-22s %-10s %-10s %s\n", e.Name, port, status, ram, uptime, next)
		}
	}

	if jsonMode {
		data, _ := json.MarshalIndent(jsonEntries, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Aggregate fleet-level "Next:" block beneath the table.
	ctx := hintsCtxCheap(paths)
	hintsAttachStatus(&ctx, statuses)
	hintsRender("list", ctx)
	return nil
}

// perRowNext picks the single best follow-up command for an agent row
// based on its observable status. Mirrors the provider logic but per-row
// (the hints package emits at command-level, not row-level).
func perRowNext(name, status string) string {
	switch status {
	case "healthy":
		return "claws agent ping " + name
	case "starting":
		return "claws logs " + name + " -f"
	case "stopped":
		return "claws start " + name
	case "created", "":
		return "claws start " + name
	case "missing":
		return "claws remove " + name + " --purge"
	}
	return "claws info " + name
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func cmdStatus(args []string) error {
	// Route to the system overview when no instance name is given. The overview
	// accepts its own flags (--group=) so we test for the presence of a
	// positional arg rather than enumerating known flag names.
	if firstPositional(args) == "" {
		return cmdStatusOverview(args)
	}
	paths := resolvePaths()
	name := firstPositional(args)
	jsonMode := hasFlag(args, "--json")
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	rt := mustResolveRuntime(paths, name)
	dir := instanceDir(paths, name)
	envFile := filepath.Join(dir, "instance.env")
	port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
	token := readEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN")
	created := ""
	if data, err := os.ReadFile(envFile); err == nil {
		for _, line := range splitLines(string(data)) {
			if strings.HasPrefix(line, "# Created:") {
				created = strings.TrimPrefix(line, "# Created: ")
			}
		}
	}

	if jsonMode {
		tokenDisplay := ""
		if len(token) > 16 {
			tokenDisplay = token[:8] + "..." + token[len(token)-8:]
		}
		obj := map[string]string{
			"name":      name,
			"created":   created,
			"directory": dir,
			"port":      port,
			"token":     tokenDisplay,
			"config":    rt.ConfigPath(dir),
			"workspace": filepath.Join(dir, "workspace"),
		}
		data, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	fmt.Printf("%sInstance: %s%s\n", bold, name, nc)
	fmt.Printf("  Created:    %s\n", created)
	fmt.Printf("  Directory:  %s\n", dir)
	fmt.Printf("  Gateway:    :%s\n", port)
	if len(token) > 16 {
		fmt.Printf("  Token:      %s...%s\n", token[:8], token[len(token)-8:])
	}
	fmt.Printf("  Config:     %s\n", rt.ConfigPath(dir))
	fmt.Printf("  Workspace:  %s/workspace/\n", dir)
	fmt.Println()

	fmt.Printf("%sContainer:%s\n", bold, nc)
	dcRun(paths, name, "ps", gatewayService(paths, name))
	return nil
}

// cmdStatusOverview shows a unified system overview: health, policy, warnings.
// Accepts an optional --group=<name> filter to scope to a single team.
func cmdStatusOverview(args []string) error {
	paths := resolvePaths()
	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 {
		if filterGroup != "" {
			fmt.Printf("No instances in group '%s'.\n", filterGroup)
			return nil
		}
		fmt.Println("No instances registered. Run: claws setup")
		return nil
	}

	// --- Health ---
	if filterGroup != "" {
		fmt.Printf("%sInstances in group '%s' (%d)%s\n\n", bold, filterGroup, len(entries), nc)
	} else {
		fmt.Printf("%sInstances (%d)%s\n\n", bold, len(entries), nc)
	}
	fmt.Printf("  %-20s %-8s %-12s %s\n", "NAME", "PORT", "HEALTH", "DETAILS")
	fmt.Printf("  %-20s %-8s %-12s %s\n", "────────────────────", "────────", "────────────", "──────────────────")

	healthy, degraded, down := 0, 0, 0
	for _, e := range entries {
		h := probeInstance(paths, e.Name)
		var color, details string
		switch h.Verdict {
		case "healthy":
			color = green
			healthy++
			details = "live + ready"
		case "degraded":
			color = yellow
			degraded++
			if len(h.Failing) > 0 {
				details = "failing: " + strings.Join(h.Failing, ", ")
			} else {
				details = "live but not ready"
			}
		case "stopped":
			color = red
			down++
			details = "container stopped"
		default:
			color = red
			down++
			details = "container down"
		}
		fmt.Printf("  %-20s %-8s %s%-12s%s %s\n", e.Name, h.Port, color, h.Verdict, nc, details)
	}
	fmt.Println()
	fmt.Printf("  %s%d healthy%s", green, healthy, nc)
	if degraded > 0 {
		fmt.Printf(", %s%d degraded%s", yellow, degraded, nc)
	}
	if down > 0 {
		fmt.Printf(", %s%d down%s", red, down, nc)
	}
	fmt.Println()

	// --- Policy ---
	fmt.Println()
	if policyExists(paths) {
		policy := readPolicy(paths)
		violations := 0
		for _, e := range entries {
			ref, _ := ParseRef(e.Name)
			dir := ref.Dir(paths)
			envFile := filepath.Join(dir, "instance.env")
			if _, statErr := os.Stat(envFile); statErr != nil {
				continue
			}
			bind := readEnvValue(envFile, "OPENCLAW_GATEWAY_BIND")
			image := readEnvValue(envFile, "OPENCLAW_IMAGE")
			if policy.enforceBindPolicy(bind) != nil {
				violations++
			}
			if policy.enforceImagePolicy(image) != nil {
				violations++
			}
			// Check channel DM policies
			configPath := mustResolveRuntime(paths, e.Name).ConfigPath(dir)
			if cfg, cfgErr := readInstanceConfig(configPath); cfgErr == nil {
				if channels, ok := cfg["channels"].(map[string]any); ok {
					for ch, v := range channels {
						chMap, ok := v.(map[string]any)
						if !ok {
							continue
						}
						if enabled, _ := chMap["enabled"].(bool); !enabled {
							continue
						}
						if policy.enforceChannelPolicy(ch) != nil {
							violations++
						}
						if dm, ok := chMap["dmPolicy"].(string); ok {
							if policy.enforceDmPolicy(dm) != nil {
								violations++
							}
						}
						if policy.enforceOutboundAllowlist(ch, chMap) != nil {
							violations++
						}
					}
				}
			}
		}
		if violations == 0 {
			fmt.Printf("%sPolicy:%s %s✓ all instances compliant%s\n", bold, nc, green, nc)
		} else {
			fmt.Printf("%sPolicy:%s %s✗ %d violation(s)%s — run: claws policy validate\n", bold, nc, red, violations, nc)
		}

		// Audit status
		if policy.AuditLog {
			fmt.Printf("%sAudit:%s  enabled\n", bold, nc)
		} else {
			fmt.Printf("%sAudit:%s  %sdisabled%s — enable in policy.json\n", bold, nc, yellow, nc)
		}
	} else {
		fmt.Printf("%sPolicy:%s %snot configured%s — run: claws policy init\n", bold, nc, yellow, nc)
	}

	// --- Access ---
	if accessExists(paths) {
		fmt.Printf("%sAccess:%s configured\n", bold, nc)
	} else {
		fmt.Printf("%sAccess:%s %snot configured%s — run: claws access init\n", bold, nc, yellow, nc)
	}

	fmt.Println()
	return nil
}

// ---------------------------------------------------------------------------
// remove
// ---------------------------------------------------------------------------

func cmdRemove(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws remove <name> [--purge] [--yes]")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	purge := hasFlag(args[1:], "--purge")
	confirmed := hasFlag(args[1:], "--yes")
	dir := instanceDir(paths, name)

	// Confirmation for purge
	if purge && !confirmed {
		warn(fmt.Sprintf("This will permanently delete ALL data for '%s':", name))
		fmt.Printf("  Directory: %s\n", dir)
		fmt.Printf("  Including: config, credentials, workspace, sessions\n")
		fmt.Print("\n  Continue? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			info("Aborted.")
			return nil
		}
	}

	// Stop
	dcRun(paths, name, "down")
	lockedUnregisterPort(paths, name)

	if purge {
		warn(fmt.Sprintf("Purging all data for '%s' (config, credentials, workspace)...", name))
		os.RemoveAll(dir)
		info(fmt.Sprintf("Instance '%s' purged.", name))
	} else {
		info(fmt.Sprintf("Instance '%s' removed (data kept at %s).", name, dir))
		fmt.Printf("  To delete data: rm -rf %s\n", dir)
	}
	return nil
}

// ---------------------------------------------------------------------------
// logs
// ---------------------------------------------------------------------------

func cmdLogs(args []string) error {
	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	grep := flagValue(args, "--grep=")
	follow := hasFlag(args, "-f") || hasFlag(args, "--follow")
	name := firstPositional(args)

	if filterGroup != "" && name != "" {
		return errorf("specify either positional instance name or --group=, not both")
	}

	// Group fan-out: two paths.
	// Follow: goroutine-multiplexed live tail with per-member color prefix.
	// Non-follow: sequential per-member dump with section headers.
	if filterGroup != "" {
		entries, _ := readRegistry(paths)
		members := filterEntriesByGroup(entries, filterGroup)
		if len(members) == 0 {
			info(fmt.Sprintf("No instances in group '%s'.", filterGroup))
			return nil
		}
		if follow {
			return logsGroupFollow(paths, members, grep)
		}
		// Pass-through flags minus --group= and the positional (none here).
		var passThrough []string
		for _, a := range args {
			if strings.HasPrefix(a, "--group=") {
				continue
			}
			if strings.HasPrefix(a, "-") {
				passThrough = append(passThrough, a)
			}
		}
		for _, e := range members {
			fmt.Printf("\033[1m=== %s ===\033[0m\n", e.Name)
			perInstanceArgs := append([]string{e.Name}, passThrough...)
			if err := cmdLogs(perInstanceArgs); err != nil {
				warn(fmt.Sprintf("logs failed for '%s': %v", e.Name, err))
			}
			fmt.Println()
		}
		return nil
	}

	if name == "" {
		return errorf("usage: claws logs <name> [-f] [--grep=<pattern>] [--since=<dur>] | claws logs --group=<name> [--grep=<pattern>]")
	}
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	// Build the docker compose logs arg list; drop our own flags that
	// docker doesn't understand.
	composeArgs := []string{"logs"}
	for _, a := range args {
		if a == name || a == "--group="+filterGroup {
			continue
		}
		if strings.HasPrefix(a, "--grep=") || strings.HasPrefix(a, "--group=") {
			continue
		}
		composeArgs = append(composeArgs, a)
	}
	composeArgs = append(composeArgs, gatewayService(paths, name))

	if grep == "" {
		// No filter — straight pass-through (preserves color, streaming, etc).
		return dc(paths, name, composeArgs...).Run()
	}

	// Filter mode: build the docker compose command ourselves so we own
	// stdout. `dc()` pre-binds cmd.Stdout = os.Stdout which conflicts with
	// StdoutPipe(); going through exec.Command directly keeps the same
	// project/template resolution logic that compose.go uses.
	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	override := rt.OverridePath(dir)
	composeTemplate := rt.ComposeTemplatePath(paths)
	allArgs := []string{"compose", "-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		allArgs = append(allArgs, "-f", override)
	}
	allArgs = append(allArgs, "--env-file", envFile, "-p", rt.MakeProjectName(ref))
	allArgs = append(allArgs, composeArgs...)

	cmd := exec.Command("docker", allArgs...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	pat := strings.ToLower(grep)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // grow for long log lines
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), pat) {
			fmt.Println(line)
		}
	}
	return cmd.Wait()
}

// ---------------------------------------------------------------------------
// exec
// ---------------------------------------------------------------------------

func cmdExec(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws exec <name> <command...>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	composeArgs := append([]string{"run", "--rm", cliService(paths, name)}, args[1:]...)
	return dc(paths, name, composeArgs...).Run()
}

// ---------------------------------------------------------------------------
// auth
// ---------------------------------------------------------------------------

func cmdAuth(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws auth <name> codex|apikey <provider> <key> | claws auth status [name] | claws auth verify <name>")
	}
	// `auth status` and `auth verify` are read-only inspections that live
	// next to the auth verbs because operators look for "is auth working?"
	// under the same noun they used to set it up. Mirrors how `channel
	// status` sits next to `channel add`.
	if args[0] == "status" {
		return cmdAuthStatus(args[1:])
	}
	if args[0] == "verify" {
		return cmdAuthVerify(args[1:])
	}
	if len(args) < 2 {
		return errorf("usage: claws auth <name> codex|apikey <provider> <key> | claws auth status [name] | claws auth verify <name>")
	}
	paths := resolvePaths()
	name := args[0]
	method := args[1]
	force := hasFlag(args, "--force")
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	// Idempotence preflight: if auth already verifies, no-op unless --force.
	// Skipped/inconclusive results do NOT short-circuit (we can't safely
	// claim "already works" without evidence). Only an explicit verified=true
	// means we can skip the dance — matches the Cloudflare-style "verify is
	// a first-class verb" principle: same primitive answers the "skip?"
	// question that answers the "is it working?" question.
	if !force {
		pre := verifyOneInstance(paths, name)
		if pre.Verified {
			info(fmt.Sprintf("'%s' is already authed and verified (strategy: %s) — no action needed", name, pre.Strategy))
			fmt.Println("  Pass --force to re-run anyway (e.g., to rotate to a fresh credential).")
			return nil
		}
	}

	switch method {
	case "codex":
		info(fmt.Sprintf("Starting OAuth flow for '%s'...", name))
		if err := dcRun(paths, name, "run", "--rm", cliService(paths, name), "models", "auth", "login", "--provider", "openai-codex", "--set-default"); err != nil {
			return err
		}
		info("Restarting gateway...")
		dcRun(paths, name, "restart", gatewayService(paths, name))
		return reportPostAuthVerify(paths, name)

	case "apikey":
		if len(args) < 4 {
			return errorf("usage: claws auth <name> apikey <provider> <key> [--force]")
		}
		provider, key := args[2], args[3]
		info(fmt.Sprintf("Adding %s API key to '%s'...", provider, name))
		if err := dcRun(paths, name, "run", "--rm", "-T", cliService(paths, name), "onboard", "--mode", "headless", "--"+provider+"-api-key", key); err != nil {
			return err
		}
		info("Restarting gateway...")
		dcRun(paths, name, "restart", gatewayService(paths, name))
		return reportPostAuthVerify(paths, name)

	default:
		return errorf("unknown auth method '%s' — use 'codex' or 'apikey'", method)
	}
}

// reportPostAuthVerify runs auth verify after a successful credential
// install + gateway restart, then surfaces a truthful result. Replaces
// the legacy "==> Auth complete" message which was aspirational (told the
// operator the CLI flow finished, not that the credential actually works).
//
// Three outcomes:
//   - verified=true: print success, exit 0.
//   - verified=false (explicit failure): print directive error, return error.
//   - inconclusive: print warning, exit 0 (the install itself succeeded;
//     we just can't *prove* the credential works yet because no log signal
//     has materialised). Operator gets the next-step hint.
func reportPostAuthVerify(paths Paths, name string) error {
	// Give the gateway a beat to come up after restart. Without this, the
	// /readyz probe races and the log scan window is empty.
	time.Sleep(3 * time.Second)

	res := verifyOneInstance(paths, name)
	red := "\033[0;31m"
	yellow := "\033[0;33m"
	nc := "\033[0m"

	switch {
	case res.Verified:
		switch res.Strategy {
		case "endpoint":
			info(fmt.Sprintf("Auth applied to '%s' and verified via upstream check.", name))
		case "readyz":
			info(fmt.Sprintf("Auth applied to '%s'; /readyz reports auth subsystem ready.", name))
		case "logs":
			info(fmt.Sprintf("Auth applied to '%s'; no auth errors observed in last 5m.", name))
			fmt.Printf("  %s(log-scan confidence — send a test message via a channel to fully confirm)%s\n", "\033[0;90m", nc)
		default:
			info(fmt.Sprintf("Auth applied to '%s'.", name))
		}
		return nil
	case res.Strategy == "skipped":
		// Install succeeded; verification was inconclusive (gateway running
		// but no recent activity to read). Don't fail the command — the
		// credential is in place; we just can't prove it works yet.
		fmt.Printf("%s==> Auth applied to '%s'; verification inconclusive — %s%s\n", yellow, name, res.Error, nc)
		if res.FixCommand != "" {
			fmt.Printf("  Next: %s\n", res.FixCommand)
		}
		return nil
	default:
		// Verification produced an explicit failure signal. The install
		// itself completed but the credential isn't working — almost
		// certainly the wrong credential or one that's already expired.
		// Surface honestly and exit non-zero.
		fmt.Printf("%s==> ERROR: auth applied to '%s' but verification failed: %s%s\n", red, name, res.Error, nc)
		if res.FixCommand != "" {
			fmt.Printf("  Fix: %s\n", res.FixCommand)
		}
		return errorf("post-install verification failed")
	}
}

// cmdChannel and cmdApprove are in channel.go

// ---------------------------------------------------------------------------
// tunnel
// ---------------------------------------------------------------------------

func cmdTunnel(args []string) error {
	paths := resolvePaths()
	names := args
	if len(names) == 0 {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		for _, e := range entries {
			names = append(names, e.Name)
		}
	}

	fmt.Println("ssh -N \\")
	for _, name := range names {
		dir := instanceDir(paths, name)
		envFile := filepath.Join(dir, "instance.env")
		port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
		if port != "" {
			fmt.Printf("  -L %s:127.0.0.1:%s \\\n", port, port)
		}
	}
	fmt.Println("  ubuntu@<server>")
	return nil
}

// ---------------------------------------------------------------------------
// stats
// ---------------------------------------------------------------------------

func cmdStats(args []string) error {
	paths := resolvePaths()
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No instances found.")
		return nil
	}

	var containers []string
	for _, e := range entries {
		containers = append(containers, fmt.Sprintf("openclaw-%s-openclaw-gateway-1", e.Name))
	}

	cmd := exec.Command("docker", append([]string{"stats", "--no-stream"}, containers...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// backup
// ---------------------------------------------------------------------------

func cmdBackup(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws backup <name> [<output-path>] [--exclude-credentials]")
	}
	paths := resolvePaths()
	name := args[0]
	excludeCreds := hasFlag(args, "--exclude-credentials")
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	output := ""
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "--") {
			output = a
			break
		}
	}
	if output == "" {
		output = fmt.Sprintf("%s-backup-%s.tar.gz", name, time.Now().UTC().Format("20060102-150405"))
	}

	// Check for credentials and warn
	ref, _ := ParseRef(name)
	credsDir := filepath.Join(ref.Dir(paths), "credentials")
	if entries, err := os.ReadDir(credsDir); err == nil && len(entries) > 0 && !excludeCreds {
		warn("Backup includes credentials directory. Store the backup file securely.")
		fmt.Printf("  Use --exclude-credentials to omit credentials from backup.\n\n")
	}

	info(fmt.Sprintf("Backing up instance '%s'...", name))
	tarArgs := []string{"czf", output, "-C", paths.Root}
	if excludeCreds {
		tarArgs = append(tarArgs, "--exclude=*/credentials/*")
		info("Excluding credentials from backup.")
	}
	tarArgs = append(tarArgs, name)
	cmd := exec.Command("tar", tarArgs...)
	if err := cmd.Run(); err != nil {
		return err
	}

	fi, _ := os.Stat(output)
	size := "unknown"
	if fi != nil {
		mb := float64(fi.Size()) / 1024 / 1024
		if mb >= 1 {
			size = fmt.Sprintf("%.1fMB", mb)
		} else {
			size = fmt.Sprintf("%.0fKB", float64(fi.Size())/1024)
		}
	}
	info(fmt.Sprintf("Backup complete: %s (%s)", output, size))
	return nil
}

// ---------------------------------------------------------------------------
// restore
// ---------------------------------------------------------------------------

func cmdRestore(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws restore <name> <backup-file>")
	}
	paths := resolvePaths()
	name := args[0]
	backup := args[1]

	if _, err := os.Stat(backup); err != nil {
		return errorf("backup file not found: %s", backup)
	}

	dir := instanceDir(paths, name)
	if _, err := os.Stat(filepath.Join(dir, "instance.env")); err == nil {
		return errorf("instance '%s' already exists — remove it first: claws remove %s --purge", name, name)
	}

	info(fmt.Sprintf("Restoring instance '%s' from %s...", name, backup))

	// Extract
	cmd := exec.Command("tar", "xzf", backup, "-C", paths.Root)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Check if tarball root matches name — if not, rename
	tarRoot := tarFirstDir(backup)
	if tarRoot != "" && tarRoot != name {
		os.Rename(filepath.Join(paths.Root, tarRoot), dir)
		envFile := filepath.Join(dir, "instance.env")
		updateEnvValue(envFile, "INSTANCE_NAME", name)
		updateEnvValue(envFile, "OPENCLAW_CONFIG_DIR", dir)
		updateEnvValue(envFile, "OPENCLAW_WORKSPACE_DIR", filepath.Join(dir, "workspace"))
	}

	// Re-register port if needed
	entries, _ := readRegistry(paths)
	found := false
	for _, e := range entries {
		if e.Name == name {
			found = true
			break
		}
	}
	if !found {
		index, err := lockedAllocatePort(paths, name)
		if err != nil {
			return errorf("failed to allocate port for restored instance: %v", err)
		}
		newPort := portForIndex(index)
		envFile := filepath.Join(dir, "instance.env")
		updateEnvValue(envFile, "OPENCLAW_GATEWAY_PORT", fmt.Sprintf("%d", newPort))
		updateEnvValue(envFile, "OPENCLAW_BRIDGE_PORT", fmt.Sprintf("%d", newPort+1))
	}

	info(fmt.Sprintf("Instance '%s' restored.", name))
	fmt.Printf("  Start with: claws start %s\n", name)
	return nil
}

func tarFirstDir(path string) string {
	cmd := exec.Command("tar", "tzf", path)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	first := string(out[:strings.IndexByte(string(out), '\n')])
	if idx := strings.IndexByte(first, '/'); idx >= 0 {
		return first[:idx]
	}
	return first
}

func updateEnvValue(envFile, key, value string) {
	data, err := os.ReadFile(envFile)
	if err != nil {
		return
	}
	var lines []string
	found := false
	for _, line := range splitLines(string(data)) {
		if strings.HasPrefix(line, key+"=") {
			lines = append(lines, key+"="+value)
			found = true
		} else {
			lines = append(lines, line)
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), credentialFileMode)
}

// ---------------------------------------------------------------------------
// share / unshare
// ---------------------------------------------------------------------------

func cmdShare(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws share <name> --skills|--workspace|--hooks|--all")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	envFile := filepath.Join(instanceDir(paths, name), "instance.env")
	changed := false
	for _, a := range args[1:] {
		switch a {
		case "--skills":
			setSharedFlag(envFile, "SHARED_SKILLS", true)
			changed = true
		case "--workspace":
			setSharedFlag(envFile, "SHARED_WORKSPACE", true)
			changed = true
		case "--hooks":
			setSharedFlag(envFile, "SHARED_HOOKS", true)
			changed = true
		case "--all":
			setSharedFlag(envFile, "SHARED_SKILLS", true)
			setSharedFlag(envFile, "SHARED_WORKSPACE", true)
			setSharedFlag(envFile, "SHARED_HOOKS", true)
			changed = true
		default:
			return errorf("unknown flag: %s", a)
		}
	}
	if !changed {
		return errorf("specify what to share: --skills, --workspace, --hooks, or --all")
	}

	rebuildOverride(paths, name)
	info(fmt.Sprintf("Shared resources updated for '%s'. Restart to apply: claws restart %s", name, name))
	return nil
}

func cmdUnshare(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws unshare <name> --skills|--workspace|--hooks|--all")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	envFile := filepath.Join(instanceDir(paths, name), "instance.env")
	changed := false
	for _, a := range args[1:] {
		switch a {
		case "--skills":
			setSharedFlag(envFile, "SHARED_SKILLS", false)
			changed = true
		case "--workspace":
			setSharedFlag(envFile, "SHARED_WORKSPACE", false)
			changed = true
		case "--hooks":
			setSharedFlag(envFile, "SHARED_HOOKS", false)
			changed = true
		case "--all":
			setSharedFlag(envFile, "SHARED_SKILLS", false)
			setSharedFlag(envFile, "SHARED_WORKSPACE", false)
			setSharedFlag(envFile, "SHARED_HOOKS", false)
			changed = true
		default:
			return errorf("unknown flag: %s", a)
		}
	}
	if !changed {
		return errorf("specify what to unshare: --skills, --workspace, --hooks, or --all")
	}

	rebuildOverride(paths, name)
	info(fmt.Sprintf("Shared resources updated for '%s'. Restart to apply: claws restart %s", name, name))
	return nil
}

// runOnGroup invokes perInstance for every instance in group, sequentially,
// stripping --group=/--yes from the recursed arg list so the per-instance
// call takes the single-instance code path. Returns nil on full success or a
// summary error listing the failures. Used by cmdStart, cmdStop, cmdRestart,
// cmdTokenRotate, cmdUpgrade for their --group= variants.
//
// Sequential (not parallel) for two reasons: (1) Docker compose operations
// are not safe to parallelize cheaply — concurrent `up -d` on a shared
// project namespace can race on the network creation; (2) operators want
// predictable per-instance log output. The latency cost is bounded by group
// size × per-instance wait, which is fine for ≤8-instance teams.
func runOnGroup(paths Paths, group, opLabel string, perInstance func([]string) error, args []string) error {
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, group)
	if len(entries) == 0 {
		info(fmt.Sprintf("No instances in group '%s'.", group))
		return nil
	}

	// Build the recursed flag list: everything that started with "-" minus
	// --group= (which we just consumed) and --yes (which the caller has
	// already honored at the confirmation prompt — passing it down would
	// suppress potential per-instance prompts we may add in future).
	var passThroughFlags []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		if strings.HasPrefix(a, "--group=") || a == "--yes" {
			continue
		}
		passThroughFlags = append(passThroughFlags, a)
	}

	info(fmt.Sprintf("%s %d instance(s) in group '%s'...", opLabel, len(entries), group))
	var failed []string
	for _, e := range entries {
		instanceArgs := append([]string{e.Name}, passThroughFlags...)
		if err := perInstance(instanceArgs); err != nil {
			warn(fmt.Sprintf("'%s' failed: %v", e.Name, err))
			failed = append(failed, e.Name)
		}
	}
	if len(failed) > 0 {
		return errorf("%d instance(s) failed: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// confirmGroupOp prompts the operator before a destructive group-scoped
// operation. Returns true iff the operation should proceed. The autoYes
// parameter (typically the value of `--yes`) bypasses the prompt for
// scripted/CI use. Calling code should only invoke this for operations that
// could affect users (stop, restart, token rotate, upgrade).
func confirmGroupOp(verb, group string, count int, autoYes bool) bool {
	if autoYes {
		return true
	}
	warn(fmt.Sprintf("This will %s %d instance(s) in group '%s'.", verb, count, group))
	fmt.Print("  Continue? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		info("Aborted.")
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// start-all / stop-all
// ---------------------------------------------------------------------------

func cmdStartAll(args []string) error {
	paths := resolvePaths()
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	var failed []string
	for _, e := range entries {
		if err := cmdStart([]string{e.Name}); err != nil {
			warn(fmt.Sprintf("failed to start '%s': %v", e.Name, err))
			failed = append(failed, e.Name)
		}
	}
	if len(failed) > 0 {
		return errorf("%d instance(s) failed to start: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func cmdStopAll(args []string) error {
	paths := resolvePaths()
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	var failed []string
	for _, e := range entries {
		if err := cmdStop([]string{e.Name}); err != nil {
			warn(fmt.Sprintf("failed to stop '%s': %v", e.Name, err))
			failed = append(failed, e.Name)
		}
	}
	if len(failed) > 0 {
		return errorf("%d instance(s) failed to stop: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// richInstanceInfo — the per-agent identity record consumed by `list --rich`
// and `claws info`. Sources are all on-disk: instance.env + openclaw.json +
// docker compose ps. No HTTP, no model invocation — this is the cheap path.
// ---------------------------------------------------------------------------

type richInstanceInfo struct {
	Name     string   `json:"name"`
	Group    string   `json:"group,omitempty"`
	Port     string   `json:"port"`
	Status   string   `json:"status"`             // healthy|starting|stopped|created
	Model    string   `json:"model"`              // from openclaw.json or "—"
	Role     string   `json:"role,omitempty"`     // manager|worker|""
	Channels []string `json:"channels,omitempty"` // names of enabled channels
	Image    string   `json:"image"`
	Runtime  string   `json:"runtime"` // runtime name (openclaw, nemoclaw, ...)
	RAM      string   `json:"ram"`
	Uptime   string   `json:"uptime"`
}

// gatherRichInfo assembles the identity record for one instance. It reads
// from disk only: never probes /healthz, never runs `models auth status`.
// That keeps the rich `list --rich` view cheap enough to be the default for
// operators who want fleet identity at a glance.
func gatherRichInfo(paths Paths, name string) richInstanceInfo {
	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	rt := mustResolveRuntime(paths, name)

	info := richInstanceInfo{
		Name:    name,
		Group:   ref.Group,
		Port:    readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT"),
		Image:   readEnvValue(envFile, "OPENCLAW_IMAGE"),
		Role:    readEnvValue(envFile, "INSTANCE_ROLE"),
		Runtime: readEnvValue(envFile, "CLAWS_RUNTIME"),
	}
	if info.Runtime == "" {
		info.Runtime = "openclaw" // default when unset (matches mustResolveRuntime fallback)
	}

	// Model + channels are read out of the runtime's config file. We use
	// the runtime's ConfigPath so this stays correct if a future runtime
	// uses a non-default config filename.
	configPath := rt.ConfigPath(dir)
	if cfg, err := readInstanceConfig(configPath); err == nil {
		if v := getNestedConfig(cfg, "agents.defaults.model.primary"); v != nil {
			if s, ok := v.(string); ok && s != "" {
				info.Model = s
			}
		}
		if channels, ok := cfg["channels"].(map[string]any); ok {
			for ch, v := range channels {
				chMap, ok := v.(map[string]any)
				if !ok {
					continue
				}
				if enabled, _ := chMap["enabled"].(bool); enabled {
					info.Channels = append(info.Channels, ch)
				}
			}
		}
	}
	sort.Strings(info.Channels) // deterministic output across runs

	// Container status maps to the same vocabulary cmdList already uses, so
	// `list` and `list --rich` agree on the status column.
	cs := containerStatus(paths, name)
	switch {
	case strings.Contains(cs, "Up") && strings.Contains(cs, "healthy"):
		info.Status = "healthy"
	case strings.Contains(cs, "Up"):
		info.Status = "starting"
	case cs != "":
		info.Status = "stopped"
	default:
		info.Status = "created"
	}
	if info.Status == "healthy" || info.Status == "starting" {
		info.RAM = containerRAM(paths, name)
		info.Uptime = strings.Replace(cs, "Up ", "", 1)
		if idx := strings.Index(info.Uptime, " ("); idx >= 0 {
			info.Uptime = info.Uptime[:idx]
		}
	}
	return info
}

// ---------------------------------------------------------------------------
// info — single-agent deep-info command (the inverse of `list --rich`)
// Consolidates the data an operator most often needs at the moment they
// pick a single instance to investigate: identity (status, model, role,
// channels), filesystem layout (dir, config, workspace), credentials state
// (token, channel creds), and a few recent audit-log entries scoped to
// this instance. All reads, no probes — cheap to run during incidents.
// ---------------------------------------------------------------------------

func cmdInfo(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	name := firstPositional(args)
	if name == "" {
		return errorf("usage: claws info <name> [--json]")
	}
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	rich := gatherRichInfo(paths, name)
	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	rt := mustResolveRuntime(paths, name)

	// Extra fields not in richInstanceInfo: created timestamp, token, paths,
	// configured-credential providers, recent activity.
	created := ""
	if data, err := os.ReadFile(envFile); err == nil {
		for _, line := range splitLines(string(data)) {
			if strings.HasPrefix(line, "# Created:") {
				created = strings.TrimPrefix(line, "# Created: ")
				break
			}
		}
	}
	token := readEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN")
	tokenDisplay := ""
	if len(token) > 16 {
		tokenDisplay = token[:8] + "..." + token[len(token)-8:]
	}

	credsDir := filepath.Join(dir, "credentials")
	var credFiles []string
	if entries, err := os.ReadDir(credsDir); err == nil {
		for _, e := range entries {
			credFiles = append(credFiles, e.Name())
		}
		sort.Strings(credFiles)
	}

	// Recent activity for THIS instance from the audit log, last 24h.
	var recent []string
	if data, err := os.ReadFile(filepath.Join(paths.Root, auditLogFile)); err == nil {
		cutoff := time.Now().Add(-24 * time.Hour)
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var entry map[string]any
			if json.Unmarshal([]byte(line), &entry) != nil {
				continue
			}
			ts, _ := entry["ts"].(string)
			if t, err := time.Parse(time.RFC3339, ts); err == nil && t.Before(cutoff) {
				continue
			}
			argsAny, _ := entry["args"].([]any)
			argStrs := make([]string, len(argsAny))
			for i, a := range argsAny {
				argStrs[i], _ = a.(string)
			}
			if !auditEntryInGroupOrName(argStrs, name) {
				continue
			}
			cmd, _ := entry["cmd"].(string)
			result, _ := entry["result"].(string)
			recent = append(recent, fmt.Sprintf("%s  %s  (%s)  %s", ts, cmd, result, strings.Join(argStrs, " ")))
		}
	}
	// Keep recent bounded so the screen stays readable.
	const maxRecent = 8
	if len(recent) > maxRecent {
		recent = recent[len(recent)-maxRecent:]
	}

	if jsonMode {
		obj := map[string]any{
			"name":      rich.Name,
			"group":     rich.Group,
			"port":      rich.Port,
			"status":    rich.Status,
			"model":     rich.Model,
			"role":      rich.Role,
			"channels":  rich.Channels,
			"image":     rich.Image,
			"runtime":   rich.Runtime,
			"ram":       rich.RAM,
			"uptime":    rich.Uptime,
			"created":   created,
			"directory": dir,
			"config":    rt.ConfigPath(dir),
			"workspace": filepath.Join(dir, "workspace"),
			"token":     tokenDisplay,
			"creds":     credFiles,
			"recent":    recent,
		}
		data, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	dim := "\033[0;90m"

	fmt.Printf("%sInstance: %s%s\n", bold, rich.Name, nc)
	fmt.Printf("  Status:     %s\n", rich.Status)
	if rich.Group != "" {
		fmt.Printf("  Group:      %s\n", rich.Group)
	}
	if rich.Role != "" {
		fmt.Printf("  Role:       %s\n", rich.Role)
	}
	fmt.Printf("  Created:    %s\n", created)
	fmt.Println()

	fmt.Printf("%sIdentity%s\n", bold, nc)
	fmt.Printf("  Model:      %s\n", orDash(rich.Model))
	fmt.Printf("  Image:      %s\n", rich.Image)
	fmt.Printf("  Runtime:    %s\n", rich.Runtime)
	fmt.Println()

	fmt.Printf("%sNetwork%s\n", bold, nc)
	fmt.Printf("  Gateway:    :%s\n", rich.Port)
	if tokenDisplay != "" {
		fmt.Printf("  Token:      %s\n", tokenDisplay)
		fmt.Printf("  %s(full: claws token show %s --full)%s\n", dim, name, nc)
	}
	fmt.Println()

	fmt.Printf("%sChannels%s\n", bold, nc)
	if len(rich.Channels) == 0 {
		fmt.Printf("  (none enabled)\n")
	} else {
		for _, ch := range rich.Channels {
			fmt.Printf("  %s\n", ch)
		}
	}
	fmt.Println()

	fmt.Printf("%sCredentials present%s\n", bold, nc)
	if len(credFiles) == 0 {
		fmt.Printf("  (none)\n")
	} else {
		for _, f := range credFiles {
			fmt.Printf("  %s\n", f)
		}
	}
	fmt.Println()

	fmt.Printf("%sFilesystem%s\n", bold, nc)
	fmt.Printf("  Directory:  %s\n", dir)
	fmt.Printf("  Config:     %s\n", rt.ConfigPath(dir))
	fmt.Printf("  Workspace:  %s/workspace/\n", dir)
	fmt.Println()

	if len(recent) > 0 {
		fmt.Printf("%sRecent activity (last 24h, max %d)%s\n", bold, maxRecent, nc)
		for _, r := range recent {
			fmt.Printf("  %s\n", r)
		}
		fmt.Println()
	}

	return nil
}

// auditEntryInGroupOrName returns true when an audit entry's positional args
// reference the exact instance name. Distinct from `auditEntryInGroup` which
// matches the *group*. Used by cmdInfo for per-instance audit filtering.
func auditEntryInGroupOrName(argStrs []string, name string) bool {
	for _, a := range argStrs {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == name
	}
	return false
}

// orDash returns "—" for empty strings; used in human-readable rendering.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// ---------------------------------------------------------------------------
// health — deep probe of each instance
// ---------------------------------------------------------------------------

type instanceHealth struct {
	Name      string
	Port      string
	Container string // up/down/missing
	Live      bool   // /healthz responds
	Ready     bool   // /readyz reports ready
	Failing   []string
	Verdict   string // healthy / degraded / down / stopped
}

func probeInstance(paths Paths, name string) instanceHealth {
	dir := instanceDir(paths, name)
	envFile := filepath.Join(dir, "instance.env")
	port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")

	h := instanceHealth{Name: name, Port: port, Container: "missing"}

	// Container state
	cs := containerStatus(paths, name)
	if strings.Contains(cs, "Up") {
		h.Container = "up"
	} else if cs != "" {
		h.Container = "stopped"
		h.Verdict = "stopped"
		return h
	} else {
		h.Verdict = "down"
		return h
	}

	rt := mustResolveRuntime(paths, name)

	// Liveness
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s%s", port, rt.HealthEndpoint))
	if err == nil && resp.StatusCode == 200 {
		h.Live = true
		resp.Body.Close()
	} else {
		if resp != nil {
			resp.Body.Close()
		}
		h.Verdict = "down"
		return h
	}

	// Readiness
	if rt.ReadyEndpoint != "" {
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%s%s", port, rt.ReadyEndpoint))
		if err == nil {
			defer resp.Body.Close()
			var body struct {
				Ready   bool     `json:"ready"`
				Failing []string `json:"failing"`
			}
			if json.NewDecoder(resp.Body).Decode(&body) == nil {
				h.Ready = body.Ready
				h.Failing = body.Failing
			}
		}
	}

	if h.Ready {
		h.Verdict = "healthy"
	} else {
		h.Verdict = "degraded"
	}
	return h
}

func cmdHealth(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	var names []string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			names = append(names, a)
		}
	}
	// Positional names and --group= are mutually exclusive: positional means
	// "probe these exact instances" while --group= means "probe everyone in
	// this team". Mixing them is ambiguous (intersection vs union) so we
	// reject the combination rather than silently pick one.
	if len(names) > 0 && filterGroup != "" {
		return errorf("specify either positional instance names or --group=, not both")
	}
	if len(names) == 0 {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		entries = filterEntriesByGroup(entries, filterGroup)
		if len(entries) == 0 {
			if jsonMode {
				fmt.Println("[]")
			} else if filterGroup != "" {
				fmt.Printf("No instances in group '%s'.\n", filterGroup)
			} else {
				fmt.Println("No instances found.")
			}
			return nil
		}
		for _, e := range entries {
			names = append(names, e.Name)
		}
	}

	type healthJSON struct {
		Name    string   `json:"name"`
		Port    string   `json:"port"`
		Verdict string   `json:"verdict"`
		Live    bool     `json:"live"`
		Ready   bool     `json:"ready"`
		Failing []string `json:"failing,omitempty"`
	}
	var jsonEntries []healthJSON

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	if !jsonMode {
		fmt.Printf("%s%-15s %-8s %-12s %-10s %s%s\n", bold, "NAME", "PORT", "VERDICT", "LIVE", "DETAILS", nc)
		fmt.Printf("%-15s %-8s %-12s %-10s %s\n", "───────────────", "────────", "────────────", "──────────", "──────────────────────")
	}

	for _, name := range names {
		if err := requireInstance(paths, name); err != nil {
			if jsonMode {
				jsonEntries = append(jsonEntries, healthJSON{Name: name, Verdict: "missing"})
			} else {
				fmt.Printf("%-15s %-8s %s%-12s%s %-10s %s\n", name, "—", red, "missing", nc, "—", "instance not found")
			}
			continue
		}

		h := probeInstance(paths, name)

		if jsonMode {
			jsonEntries = append(jsonEntries, healthJSON{
				Name: h.Name, Port: h.Port, Verdict: h.Verdict,
				Live: h.Live, Ready: h.Ready, Failing: h.Failing,
			})
			continue
		}

		var color string
		switch h.Verdict {
		case "healthy":
			color = green
		case "degraded":
			color = yellow
		case "stopped", "down":
			color = red
		default:
			color = red
		}

		live := "—"
		if h.Container == "up" {
			if h.Live {
				live = green + "yes" + nc
			} else {
				live = red + "no" + nc
			}
		}

		details := ""
		if len(h.Failing) > 0 {
			details = "failing: " + strings.Join(h.Failing, ", ")
		}
		if h.Verdict == "stopped" {
			details = "container stopped"
		}
		if h.Verdict == "down" && h.Container == "missing" {
			details = "not created or removed"
		} else if h.Verdict == "down" && !h.Live {
			details = "gateway not responding"
		}

		fmt.Printf("%-15s :%-7s %s%-12s%s %-18s %s\n", h.Name, h.Port, color, h.Verdict, nc, live, details)
	}

	if jsonMode {
		data, _ := json.MarshalIndent(jsonEntries, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

// ---------------------------------------------------------------------------
// dashboard — live refreshing view of all instances
// ---------------------------------------------------------------------------

func cmdDashboard(args []string) error {
	paths := resolvePaths()
	interval := 5 * time.Second

	// Parse optional interval
	for _, a := range args {
		if strings.HasPrefix(a, "--interval=") {
			if d, err := time.ParseDuration(a[11:]); err == nil {
				interval = d
			}
		}
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"
	_ = yellow // used in render

	fmt.Printf("claws dashboard — refreshing every %s (Ctrl+C to exit)\n", interval)

	for {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}

		// Clear screen
		fmt.Print("\033[2J\033[H")
		fmt.Printf("%sclaws dashboard%s — %s — refreshing every %s\n\n",
			bold, nc, time.Now().Format("15:04:05"), interval)

		if len(entries) == 0 {
			fmt.Println("No instances found.")
		} else {
			fmt.Printf("%s%-15s %-8s %-12s %-10s %-10s %s%s\n", bold, "NAME", "PORT", "HEALTH", "RAM", "UPTIME", "DETAILS", nc)
			fmt.Printf("%-15s %-8s %-12s %-10s %-10s %s\n", "───────────────", "────────", "────────────", "──────────", "──────────", "──────────────────────")

			for _, e := range entries {
				h := probeInstance(paths, e.Name)

				var color string
				switch h.Verdict {
				case "healthy":
					color = green
				case "degraded":
					color = yellow
				default:
					color = red
				}

				ram := "—"
				uptime := "—"
				if h.Container == "up" {
					ram = containerRAM(paths, e.Name)
					cs := containerStatus(paths, e.Name)
					uptime = cs
					uptime = strings.Replace(uptime, "Up ", "", 1)
					if idx := strings.Index(uptime, " ("); idx >= 0 {
						uptime = uptime[:idx]
					}
				}

				details := ""
				if len(h.Failing) > 0 {
					details = "failing: " + strings.Join(h.Failing, ", ")
				}

				fmt.Printf("%-15s :%-7s %s%-12s%s %-10s %-10s %s\n",
					e.Name, h.Port, color, h.Verdict, nc, ram, uptime, details)
			}
		}

		fmt.Printf("\n%s[Ctrl+C to exit]%s", bold, nc)
		time.Sleep(interval)
	}
}
