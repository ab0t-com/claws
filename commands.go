package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
		return errorf("unknown runtime '%s' — see available: clawctl runtime list", runtimeName)
	}

	if nameArg == "" {
		return errorf("usage: clawctl create <name|group/name> [--from=<instance>] [--role=manager|worker] [--runtime=openclaw] [--shared-*]")
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
			return errorf("group '%s' does not exist — create it first: clawctl group create %s", ref.Group, ref.Group)
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
		return errorf("workers must be in a group — use: clawctl create <group>/%s --role=worker", ref.Name)
	}
	if role == "manager" && ref.Group == "" {
		return errorf("managers must be in a group — use: clawctl create <group>/%s --role=manager", ref.Name)
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

	// Create dirs
	for _, sub := range []string{"credentials", "agents", "identity", "workspace", "sessions", "canvas", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			cleanup(dir, paths, name)
			return err
		}
	}

	// Write instance.env (image already resolved above for policy check)
	envContent := fmt.Sprintf(`# Instance: %s
# Created: %s
# Port index: %d

INSTANCE_NAME=%s
OPENCLAW_CONFIG_DIR=%s
OPENCLAW_WORKSPACE_DIR=%s/workspace
OPENCLAW_GATEWAY_PORT=%d
OPENCLAW_BRIDGE_PORT=%d
OPENCLAW_GATEWAY_TOKEN=%s
OPENCLAW_GATEWAY_BIND=%s
OPENCLAW_HOST_BIND=%s
OPENCLAW_IMAGE=%s
CLAWCTL_RUNTIME=%s
OPENCLAW_ALLOW_INSECURE_PRIVATE_WS=
CLAUDE_AI_SESSION_KEY=
CLAUDE_WEB_SESSION_KEY=
CLAUDE_WEB_COOKIE=
`, name, time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), index,
		name, dir, dir, gatewayPort, bridgePort, token, bindMode, hostBind(bindMode), image, runtimeName)

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
	if os.Getenv("CLAWCTL_SKIP_VALIDATE") == "" {
		if _, err := dcOutput(paths, ref.RegistryName(), "config"); err != nil {
			cleanup(dir, paths, ref.RegistryName())
			return errorf("compose config validation failed")
		}
	}

	info(fmt.Sprintf("Instance '%s' created.", name))
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
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    clawctl auth %s codex              # add OpenAI Codex auth\n", name)
	fmt.Printf("    clawctl auth %s apikey openai <key> # or add an API key\n", name)
	fmt.Printf("    clawctl start %s                    # start the instance\n", name)
	fmt.Println()
	fmt.Println("  SSH tunnel:")
	fmt.Printf("    ssh -N -L %d:127.0.0.1:%d ubuntu@<server>\n", gatewayPort, gatewayPort)
	return nil
}

func cleanup(dir string, paths Paths, name string) {
	os.RemoveAll(dir)
	lockedUnregisterPort(paths, name)
}

func portInUse(port int) bool {
	if os.Getenv("CLAWCTL_SKIP_VALIDATE") != "" {
		return false // skip in test mode
	}
	cmd := exec.Command("ss", "-tlnp")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf(":%d ", port))
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

func cmdStart(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl start <name>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	info(fmt.Sprintf("Starting instance '%s'...", name))
	if err := dcRun(paths, name, "up", "-d", gatewayService(paths, name)); err != nil {
		return err
	}

	// Wait for health
	dir := instanceDir(paths, name)
	port := readEnvValue(filepath.Join(dir, "instance.env"), "OPENCLAW_GATEWAY_PORT")
	rt := mustResolveRuntime(paths, name)
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, rt.HealthEndpoint)

	fmt.Println()
	info("Waiting for health...")
	for i := 0; i < 15; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			info(fmt.Sprintf("Instance '%s' is healthy on :%s", name, port))
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	warn("Health check didn't pass in 30s — check: clawctl logs " + name)
	return nil
}

// ---------------------------------------------------------------------------
// stop
// ---------------------------------------------------------------------------

func cmdStop(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl stop <name>")
	}
	paths := resolvePaths()
	name := args[0]
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
	if len(args) < 1 {
		return errorf("usage: clawctl restart <name> [--hard]")
	}
	paths := resolvePaths()
	name := args[0]
	hard := hasFlag(args[1:], "--hard")
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
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if jsonMode {
			fmt.Println("[]")
		} else {
			fmt.Println("No instances found.")
		}
		return nil
	}

	type listEntry struct {
		Name   string `json:"name"`
		Port   string `json:"port"`
		Status string `json:"status"`
		RAM    string `json:"ram"`
		Uptime string `json:"uptime"`
	}
	var jsonEntries []listEntry

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	if !jsonMode {
		fmt.Printf("%s%-15s %-8s %-12s %-10s %s%s\n", bold, "NAME", "PORT", "STATUS", "RAM", "UPTIME", nc)
		fmt.Printf("%-15s %-8s %-12s %-10s %s\n", "───────────────", "────────", "────────────", "──────────", "──────────")
	}

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

		if jsonMode {
			jsonEntries = append(jsonEntries, listEntry{
				Name: e.Name, Port: port, Status: statusPlain, RAM: ram, Uptime: uptime,
			})
		} else {
			fmt.Printf("%-15s :%-7s %-22s %-10s %s\n", e.Name, port, status, ram, uptime)
		}
	}

	if jsonMode {
		data, _ := json.MarshalIndent(jsonEntries, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func cmdStatus(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl status <name> [--json]")
	}
	paths := resolvePaths()
	name := args[0]
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

// ---------------------------------------------------------------------------
// remove
// ---------------------------------------------------------------------------

func cmdRemove(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl remove <name> [--purge] [--yes]")
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
	if len(args) < 1 {
		return errorf("usage: clawctl logs <name> [-f]")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	composeArgs := append([]string{"logs"}, args[1:]...)
	composeArgs = append(composeArgs, gatewayService(paths, name))
	return dc(paths, name, composeArgs...).Run()
}

// ---------------------------------------------------------------------------
// exec
// ---------------------------------------------------------------------------

func cmdExec(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl exec <name> <command...>")
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
	if len(args) < 2 {
		return errorf("usage: clawctl auth <name> codex|apikey <provider> <key>")
	}
	paths := resolvePaths()
	name := args[0]
	method := args[1]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	switch method {
	case "codex":
		info(fmt.Sprintf("Starting OAuth flow for '%s'...", name))
		if err := dcRun(paths, name, "run", "--rm", cliService(paths, name), "models", "auth", "login", "--provider", "openai-codex", "--set-default"); err != nil {
			return err
		}
		info("Restarting gateway...")
		dcRun(paths, name, "restart", gatewayService(paths, name))
		info(fmt.Sprintf("Auth complete for '%s'.", name))

	case "apikey":
		if len(args) < 4 {
			return errorf("usage: clawctl auth <name> apikey <provider> <key>")
		}
		provider, key := args[2], args[3]
		info(fmt.Sprintf("Adding %s API key to '%s'...", provider, name))
		if err := dcRun(paths, name, "run", "--rm", "-T", cliService(paths, name), "onboard", "--mode", "headless", "--"+provider+"-api-key", key); err != nil {
			return err
		}
		info("Restarting gateway...")
		dcRun(paths, name, "restart", gatewayService(paths, name))
		info(fmt.Sprintf("Auth complete for '%s'.", name))

	default:
		return errorf("unknown auth method '%s' — use 'codex' or 'apikey'", method)
	}
	return nil
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
		return errorf("usage: clawctl backup <name> [<output-path>] [--exclude-credentials]")
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
		return errorf("usage: clawctl restore <name> <backup-file>")
	}
	paths := resolvePaths()
	name := args[0]
	backup := args[1]

	if _, err := os.Stat(backup); err != nil {
		return errorf("backup file not found: %s", backup)
	}

	dir := instanceDir(paths, name)
	if _, err := os.Stat(filepath.Join(dir, "instance.env")); err == nil {
		return errorf("instance '%s' already exists — remove it first: clawctl remove %s --purge", name, name)
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
	fmt.Printf("  Start with: clawctl start %s\n", name)
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
		return errorf("usage: clawctl share <name> --skills|--workspace|--hooks|--all")
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
	info(fmt.Sprintf("Shared resources updated for '%s'. Restart to apply: clawctl restart %s", name, name))
	return nil
}

func cmdUnshare(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl unshare <name> --skills|--workspace|--hooks|--all")
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
	info(fmt.Sprintf("Shared resources updated for '%s'. Restart to apply: clawctl restart %s", name, name))
	return nil
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

	var names []string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			names = append(names, a)
		}
	}
	if len(names) == 0 {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if jsonMode {
				fmt.Println("[]")
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

	fmt.Printf("clawctl dashboard — refreshing every %s (Ctrl+C to exit)\n", interval)

	for {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}

		// Clear screen
		fmt.Print("\033[2J\033[H")
		fmt.Printf("%sclawctl dashboard%s — %s — refreshing every %s\n\n",
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
