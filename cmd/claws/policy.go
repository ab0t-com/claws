package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Policy defines admin-enforced constraints across all instances.
type Policy struct {
	// Network
	AllowedBindModes []string `json:"allowedBindModes,omitempty"` // e.g., ["loopback"], empty = any
	MaxInstances     int      `json:"maxInstances,omitempty"`     // 0 = use default (8)

	// Container
	MemoryLimitMB int  `json:"memoryLimitMB,omitempty"` // 0 = no limit
	CPULimit      float64 `json:"cpuLimit,omitempty"`    // 0 = no limit
	AllowDockerSocket bool `json:"allowDockerSocket,omitempty"`

	// Agent
	RequireSandbox           bool     `json:"requireSandbox,omitempty"`
	AllowedToolProfiles      []string `json:"allowedToolProfiles,omitempty"` // empty = any
	RequireDmPairing         bool     `json:"requireDmPairing,omitempty"`
	RequireOutboundAllowlist bool     `json:"requireOutboundAllowlist,omitempty"` // sendMessage requires allowFrom
	BlockedChannels          []string `json:"blockedChannels,omitempty"`

	// Image
	AllowedImages []string `json:"allowedImages,omitempty"` // glob patterns, empty = any

	// Audit
	AuditLog bool `json:"auditLog,omitempty"`
}

const policyFile = "policy.json"

func readPolicy(paths Paths) Policy {
	data, err := os.ReadFile(filepath.Join(paths.Root, policyFile))
	if err != nil {
		return Policy{} // no policy = no constraints
	}
	var p Policy
	json.Unmarshal(data, &p)
	return p
}

func writePolicy(paths Paths, p Policy) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.Root, policyFile), append(data, '\n'), 0600)
}

func policyExists(paths Paths) bool {
	_, err := os.Stat(filepath.Join(paths.Root, policyFile))
	return err == nil
}

// ---------------------------------------------------------------------------
// Enforcement
// ---------------------------------------------------------------------------

// enforceBindPolicy checks if a bind mode is allowed by policy.
func (p Policy) enforceBindPolicy(bind string) error {
	if len(p.AllowedBindModes) == 0 {
		return nil
	}
	for _, allowed := range p.AllowedBindModes {
		if allowed == bind {
			return nil
		}
	}
	return fmt.Errorf("policy violation: bind mode '%s' not allowed (allowed: %s)", bind, strings.Join(p.AllowedBindModes, ", "))
}

// enforceImagePolicy checks if an image is allowed by policy.
func (p Policy) enforceImagePolicy(image string) error {
	if len(p.AllowedImages) == 0 {
		return nil
	}
	for _, pattern := range p.AllowedImages {
		if matchGlob(pattern, image) {
			return nil
		}
	}
	return fmt.Errorf("policy violation: image '%s' not allowed (allowed: %s)", image, strings.Join(p.AllowedImages, ", "))
}

// enforceChannelPolicy checks if a channel is blocked by policy.
func (p Policy) enforceChannelPolicy(channel string) error {
	for _, blocked := range p.BlockedChannels {
		if blocked == channel {
			return fmt.Errorf("policy violation: channel '%s' is blocked by admin policy", channel)
		}
	}
	return nil
}

// enforceDmPolicy checks that DM policy is not "open" (pairing or stricter required).
func (p Policy) enforceDmPolicy(dmPolicy string) error {
	if p.RequireDmPairing && dmPolicy != "pairing" && dmPolicy != "allowlist" && dmPolicy != "disabled" {
		return fmt.Errorf("policy violation: dmPolicy '%s' is too permissive (pairing or allowlist required)", dmPolicy)
	}
	return nil
}

// enforceOutboundAllowlist checks that sendMessage is not enabled without an allowFrom list.
func (p Policy) enforceOutboundAllowlist(channel string, chMap map[string]any) error {
	if !p.RequireOutboundAllowlist {
		return nil
	}
	actions, ok := chMap["actions"].(map[string]any)
	if !ok {
		return nil // no explicit actions = using defaults (we can't enforce here)
	}
	sendKey := channelSendAction(channel)
	sendEnabled, ok := actions[sendKey].(bool)
	if !ok || !sendEnabled {
		return nil
	}
	// sendMessage is enabled — check allowFrom
	af, _ := chMap["allowFrom"].([]any)
	if len(af) == 0 {
		return fmt.Errorf("channel %s: outbound messaging enabled without allowFrom contacts", channel)
	}
	return nil
}

// enforceMaxInstances checks instance count against policy.
func (p Policy) enforceMaxInstances(current int) error {
	max := p.MaxInstances
	if max == 0 {
		max = maxInstances // use default
	}
	if current >= max {
		return fmt.Errorf("policy violation: maximum %d instances reached", max)
	}
	return nil
}

// matchGlob does simple glob matching (supports * as wildcard).
func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	// Simple prefix/suffix matching with single *
	parts := strings.SplitN(pattern, "*", 2)
	if len(parts) == 2 {
		return strings.HasPrefix(s, parts[0]) && strings.HasSuffix(s, parts[1])
	}
	return false
}

// ---------------------------------------------------------------------------
// claws policy — view and manage admin policy
// ---------------------------------------------------------------------------

func cmdPolicy(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws policy <show|init|validate|enforce>")
	}
	switch args[0] {
	case "show":
		return cmdPolicyShow(args[1:])
	case "init":
		return cmdPolicyInit(args[1:])
	case "validate":
		return cmdPolicyValidate(args[1:])
	case "enforce":
		return cmdPolicyEnforce(args[1:])
	default:
		return errorf("unknown policy subcommand: %s", args[0])
	}
}

func cmdPolicyShow(args []string) error {
	paths := resolvePaths()
	if !policyExists(paths) {
		fmt.Println("No policy configured. Run: claws policy init")
		return nil
	}
	p := readPolicy(paths)
	data, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(data))
	return nil
}

func cmdPolicyInit(args []string) error {
	paths := resolvePaths()
	policyPath := filepath.Join(paths.Root, policyFile)

	if policyExists(paths) && !hasFlag(args, "--force") {
		return errorf("policy.json already exists. Use --force to overwrite.")
	}

	// Secure defaults
	p := Policy{
		AllowedBindModes:         []string{"loopback"},
		MaxInstances:             8,
		MemoryLimitMB:            2048,
		CPULimit:                 2.0,
		AllowDockerSocket:        false,
		RequireSandbox:           false, // start permissive, admin can tighten
		RequireDmPairing:         true,
		RequireOutboundAllowlist: true,
		AllowedImages:            []string{"openclaw:*"},
		AuditLog:                 true,
	}

	if err := writePolicy(paths, p); err != nil {
		return err
	}

	info(fmt.Sprintf("Policy created: %s", policyPath))
	fmt.Println()
	data, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(data))
	fmt.Println()
	fmt.Println("  Edit to customize: claws config edit policy")
	fmt.Println("  Validate instances: claws policy validate")
	return nil
}

func cmdPolicyValidate(args []string) error {
	paths := resolvePaths()
	if !policyExists(paths) {
		fmt.Println("No policy configured. Run: claws policy init")
		return nil
	}

	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	jsonMode := hasFlag(args, "--json")

	p := readPolicy(paths)
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 && filterGroup != "" {
		if jsonMode {
			fmt.Println("[]")
		} else {
			fmt.Printf("No instances in group '%s'.\n", filterGroup)
		}
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"

	if !jsonMode {
		if filterGroup != "" {
			fmt.Printf("%sPolicy validation (group: %s)%s\n\n", bold, filterGroup, nc)
		} else {
			fmt.Printf("%sPolicy validation%s\n\n", bold, nc)
		}
	}

	type instanceReport struct {
		Name     string   `json:"name"`
		Issues   []string `json:"issues"`
		Compliant bool    `json:"compliant"`
	}
	var jsonReports []instanceReport

	violations := 0
	for _, e := range entries {
		ref, _ := ParseRef(e.Name)
		dir := ref.Dir(paths)
		envFile := filepath.Join(dir, "instance.env")

		if _, err := os.Stat(envFile); err != nil {
			continue
		}

		bind := readEnvValue(envFile, "OPENCLAW_GATEWAY_BIND")
		image := readEnvValue(envFile, "OPENCLAW_IMAGE")

		var issues []string

		if err := p.enforceBindPolicy(bind); err != nil {
			issues = append(issues, fmt.Sprintf("bind=%s (allowed: %s)", bind, strings.Join(p.AllowedBindModes, ",")))
		}
		if err := p.enforceImagePolicy(image); err != nil {
			issues = append(issues, fmt.Sprintf("image=%s not in allowed list", image))
		}

		// Check channel DM policies
		configPath := mustResolveRuntime(paths, e.Name).ConfigPath(dir)
		if cfg, err := readInstanceConfig(configPath); err == nil {
			if channels, ok := cfg["channels"].(map[string]any); ok {
				for ch, v := range channels {
					chMap, ok := v.(map[string]any)
					if !ok {
						continue
					}
					enabled, _ := chMap["enabled"].(bool)
					if !enabled {
						continue
					}
					if err := p.enforceChannelPolicy(ch); err != nil {
						issues = append(issues, fmt.Sprintf("channel %s is blocked", ch))
					}
					if dm, ok := chMap["dmPolicy"].(string); ok {
						if err := p.enforceDmPolicy(dm); err != nil {
							issues = append(issues, fmt.Sprintf("channel %s: dmPolicy=%s (pairing required)", ch, dm))
						}
					}
					if err := p.enforceOutboundAllowlist(ch, chMap); err != nil {
						issues = append(issues, err.Error())
					}
				}
			}
		}

		jsonReports = append(jsonReports, instanceReport{
			Name: e.Name, Issues: issues, Compliant: len(issues) == 0,
		})

		if jsonMode {
			violations += len(issues)
			continue
		}
		if len(issues) == 0 {
			fmt.Printf("  %s%-20s%s %s✓%s\n", bold, e.Name, nc, green, nc)
		} else {
			for _, issue := range issues {
				fmt.Printf("  %s%-20s%s %s✗ %s%s\n", bold, e.Name, nc, red, issue, nc)
				violations++
			}
		}
	}

	if jsonMode {
		if jsonReports == nil {
			jsonReports = []instanceReport{}
		}
		data, _ := json.MarshalIndent(jsonReports, "", "  ")
		fmt.Println(string(data))
		// Non-zero exit on violations even in JSON mode — matches text path.
		if violations > 0 {
			return errorf("%d policy violation(s) found", violations)
		}
		return nil
	}

	fmt.Println()
	if violations > 0 {
		return errorf("%d policy violation(s) found", violations)
	}
	info("All instances comply with policy.")
	return nil
}

// cmdPolicyEnforce fixes all policy violations across instances.
func cmdPolicyEnforce(args []string) error {
	paths := resolvePaths()
	if !policyExists(paths) {
		return errorf("no policy configured — run: claws policy init")
	}

	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	p := readPolicy(paths)
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 && filterGroup != "" {
		fmt.Printf("No instances in group '%s'.\n", filterGroup)
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"

	if filterGroup != "" {
		fmt.Printf("%sPolicy enforcement (group: %s)%s\n\n", bold, filterGroup, nc)
	} else {
		fmt.Printf("%sPolicy enforcement%s\n\n", bold, nc)
	}

	fixed := 0
	needsRestart := map[string]bool{}

	for _, e := range entries {
		ref, _ := ParseRef(e.Name)
		dir := ref.Dir(paths)
		envFile := filepath.Join(dir, "instance.env")

		if _, err := os.Stat(envFile); err != nil {
			continue
		}

		// Fix bind mode
		bind := readEnvValue(envFile, "OPENCLAW_GATEWAY_BIND")
		if err := p.enforceBindPolicy(bind); err != nil {
			if len(p.AllowedBindModes) > 0 {
				newBind := p.AllowedBindModes[0]
				updateEnvValue(envFile, "OPENCLAW_GATEWAY_BIND", newBind)
				updateEnvValue(envFile, "OPENCLAW_HOST_BIND", hostBind(newBind))
				info(fmt.Sprintf("%s: bind %s → %s", e.Name, bind, newBind))
				fixed++
				needsRestart[e.Name] = true
			}
		}

		// Ensure OPENCLAW_HOST_BIND exists and matches bind mode
		currentHostBind := readEnvValue(envFile, "OPENCLAW_HOST_BIND")
		currentBind := readEnvValue(envFile, "OPENCLAW_GATEWAY_BIND")
		expectedHostBind := hostBind(currentBind)
		if currentHostBind != expectedHostBind {
			updateEnvValue(envFile, "OPENCLAW_HOST_BIND", expectedHostBind)
			if currentHostBind != "" {
				info(fmt.Sprintf("%s: host bind %s → %s", e.Name, currentHostBind, expectedHostBind))
			} else {
				info(fmt.Sprintf("%s: added host bind %s", e.Name, expectedHostBind))
			}
			fixed++
			needsRestart[e.Name] = true
		}

		// Fix channel DM policies
		configPath := mustResolveRuntime(paths, e.Name).ConfigPath(dir)
		cfg, err := readInstanceConfig(configPath)
		if err != nil {
			continue
		}

		cfgChanged := false
		channels, ok := cfg["channels"].(map[string]any)
		if !ok {
			continue
		}

		for ch, v := range channels {
			chMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			enabled, _ := chMap["enabled"].(bool)
			if !enabled {
				continue
			}

			// Block disallowed channels
			if err := p.enforceChannelPolicy(ch); err != nil {
				chMap["enabled"] = false
				info(fmt.Sprintf("%s: disabled blocked channel %s", e.Name, ch))
				fixed++
				cfgChanged = true
				needsRestart[e.Name] = true
			}

			// Fix DM policy
			dm, _ := chMap["dmPolicy"].(string)
			if err := p.enforceDmPolicy(dm); err != nil {
				chMap["dmPolicy"] = "pairing"
				info(fmt.Sprintf("%s: %s dmPolicy %s → pairing", e.Name, ch, dm))
				fixed++
				cfgChanged = true
				needsRestart[e.Name] = true
			}

			// Fix outbound without allowlist
			if err := p.enforceOutboundAllowlist(ch, chMap); err != nil {
				sendKey := channelSendAction(ch)
				actions, ok := chMap["actions"].(map[string]any)
				if ok {
					actions[sendKey] = false
					info(fmt.Sprintf("%s: %s %s disabled (no allowFrom contacts)", e.Name, ch, sendKey))
					fixed++
					cfgChanged = true
					needsRestart[e.Name] = true
				}
			}
		}

		if cfgChanged {
			writeInstanceConfig(configPath, cfg)
		}
	}

	// Fix file permissions
	envFixed, credFixed, regFixed := fixAllPermissions(paths.Root)
	permFixed := envFixed + credFixed + regFixed
	if permFixed > 0 {
		info(fmt.Sprintf("Fixed permissions: %d env, %d credential, %d registry", envFixed, credFixed, regFixed))
		fixed += permFixed
	}

	fmt.Println()
	if fixed == 0 {
		info("No violations found — all instances comply with policy.")
		return nil
	}

	info(fmt.Sprintf("%d fix(es) applied.", fixed))

	if len(needsRestart) > 0 {
		fmt.Println()
		fmt.Println("  Instances that need restart to apply changes:")
		for name := range needsRestart {
			fmt.Printf("    claws restart %s\n", name)
		}
		fmt.Println()

		if hasFlag(args, "--restart") {
			for name := range needsRestart {
				info(fmt.Sprintf("Hard-restarting %s (recreating container)...", name))
				dcRun(paths, name, "down")
				dcRun(paths, name, "up", "-d", gatewayService(paths, name))
			}
			info("All affected instances restarted with updated config.")
		} else {
			fmt.Println("  Add --restart to automatically restart affected instances.")
		}
	}

	return nil
}
