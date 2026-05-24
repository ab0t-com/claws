package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// channelSafeDefaults defines per-channel action defaults applied on channel add.
// Principle: reactions + read-only info = ON, anything that sends/modifies = OFF.
var channelSafeDefaults = map[string]map[string]any{
	"whatsapp": {"reactions": true, "sendMessage": false, "polls": false},
	"telegram": {"reactions": true, "sendMessage": false, "poll": false, "deleteMessage": false, "sticker": true},
	"discord": {
		"reactions": true, "messages": false, "stickers": true, "polls": false,
		"permissions": false, "moderation": false, "roles": false, "channels": false,
		"emojiUploads": false, "stickerUploads": false, "threads": false, "pins": false,
		"search": true, "memberInfo": true, "roleInfo": true, "channelInfo": true,
		"voiceStatus": false, "events": false, "presence": false,
	},
	"slack": {
		"reactions": true, "messages": false, "pins": false, "search": true,
		"permissions": false, "memberInfo": true, "channelInfo": true, "emojiList": true,
	},
	"signal": {"reactions": true},
}

// channelSendAction returns the action key that controls outbound messaging for a channel.
func channelSendAction(channel string) string {
	switch channel {
	case "discord", "slack":
		return "messages"
	default:
		return "sendMessage"
	}
}

// channelProfile defines what config keys each channel needs.
type channelProfile struct {
	Name            string
	RequiredKey     string     // the primary credential flag name (e.g., "--token")
	ConfigKeys      []configPair // config keys to set from flags
	LoginFlow       bool       // true if channel needs interactive login (e.g., WhatsApp QR)
	DefaultDmPolicy string     // override default dmPolicy (empty = "pairing")
}

type configPair struct {
	Flag      string // CLI flag (e.g., "--token")
	ConfigKey string // openclaw.json path (e.g., "channels.telegram.botToken")
}

var channelProfiles = map[string]channelProfile{
	"telegram": {
		Name:        "telegram",
		RequiredKey: "--token",
		ConfigKeys: []configPair{
			{"--token", "channels.telegram.botToken"},
		},
	},
	"discord": {
		Name:        "discord",
		RequiredKey: "--token",
		ConfigKeys: []configPair{
			{"--token", "channels.discord.token"},
		},
	},
	"slack": {
		Name:        "slack",
		RequiredKey: "--bot-token",
		ConfigKeys: []configPair{
			{"--bot-token", "channels.slack.botToken"},
			{"--app-token", "channels.slack.appToken"},
		},
	},
	"signal": {
		Name:            "signal",
		RequiredKey:     "--number",
		DefaultDmPolicy: "allowlist",
		ConfigKeys: []configPair{
			{"--number", "channels.signal.account"},
		},
	},
	"whatsapp": {
		Name:            "whatsapp",
		LoginFlow:       true,
		DefaultDmPolicy: "allowlist",
	},
}

// ---------------------------------------------------------------------------
// claws channel — add/remove/list channels
// ---------------------------------------------------------------------------

func cmdChannel(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws channel add <instance> <channel> [--token=...]\n       claws channel <instance> <channel> [args...]  (wizard)")
	}

	// New subcommand style: claws channel add <instance> <channel> --token=...
	if args[0] == "add" {
		return cmdChannelAdd(args[1:])
	}
	if args[0] == "remove" {
		return cmdChannelRemove(args[1:])
	}
	if args[0] == "status" {
		return cmdChannelStatus(args[1:])
	}
	if args[0] == "security" {
		return cmdChannelSecurity(args[1:])
	}
	if args[0] == "send" {
		return cmdChannelSend(args[1:])
	}
	if args[0] == "allow" {
		return cmdChannelAllow(args[1:])
	}
	if args[0] == "deny" {
		return cmdChannelDeny(args[1:])
	}

	// Legacy style: claws channel <instance> <channel> [args...]
	// Check if second arg looks like a known channel with flags — route to new path
	if len(args) >= 2 {
		channel := args[1]
		if _, ok := channelProfiles[channel]; ok && hasFlagsInArgs(args[2:]) {
			return cmdChannelAdd(args)
		}
	}

	// Fall through to legacy wizard
	return cmdChannelLegacy(args)
}

func hasFlagsInArgs(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--token=") || strings.HasPrefix(a, "--bot-token=") ||
			strings.HasPrefix(a, "--app-token=") || strings.HasPrefix(a, "--number=") {
			return true
		}
	}
	return false
}

// cmdChannelAdd is the new direct-config path.
func cmdChannelAdd(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws channel add <instance> <channel> [--token=...] [--dm-policy=pairing] [--allow-send]")
	}

	paths := resolvePaths()
	name := args[0]
	channel := args[1]

	if err := requireInstance(paths, name); err != nil {
		return err
	}

	// Check runtime supports channels
	rt, err := resolveRuntime(paths, name)
	if err != nil {
		return err
	}
	if err := rt.RequireCapability("channels"); err != nil {
		return err
	}

	profile, known := channelProfiles[channel]
	if !known {
		// Unknown channel — fall through to wizard
		info(fmt.Sprintf("Channel '%s' not in quick-setup profiles — using interactive wizard.", channel))
		return cmdChannelLegacy(append([]string{name}, args[1:]...))
	}

	// Parse flags
	flags := map[string]string{}
	dmPolicy := profile.DefaultDmPolicy
	if dmPolicy == "" {
		dmPolicy = "pairing"
	}
	allowSend := false
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--dm-policy=") {
			dmPolicy = a[12:]
			continue
		}
		if a == "--allow-send" {
			allowSend = true
			continue
		}
		for _, ck := range profile.ConfigKeys {
			if strings.HasPrefix(a, ck.Flag+"=") {
				flags[ck.Flag] = a[len(ck.Flag)+1:]
			}
		}
	}

	// WhatsApp / login-flow channels
	if profile.LoginFlow {
		return cmdChannelAddWithLogin(paths, name, channel, dmPolicy, allowSend)
	}

	// Check required credential
	if _, ok := flags[profile.RequiredKey]; !ok {
		// Slack needs two tokens
		if channel == "slack" {
			return errorf("usage: claws channel add %s slack --bot-token=xoxb-... --app-token=xapp-...", name)
		}
		return errorf("usage: claws channel add %s %s %s=<value>", name, channel, profile.RequiredKey)
	}

	// Policy enforcement
	policy := readPolicy(paths)
	if err := policy.enforceChannelPolicy(channel); err != nil {
		return err
	}
	if err := policy.enforceDmPolicy(dmPolicy); err != nil {
		return err
	}

	// Apply config
	info(fmt.Sprintf("Configuring %s on '%s'...", channel, name))

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	// Set enabled
	setNestedConfig(cfg, "channels."+channel+".enabled", true)

	// Set credentials
	for _, ck := range profile.ConfigKeys {
		if val, ok := flags[ck.Flag]; ok {
			setNestedConfig(cfg, ck.ConfigKey, val)
		}
	}

	// Set DM policy
	setNestedConfig(cfg, "channels."+channel+".dmPolicy", dmPolicy)

	// Apply safe action defaults
	applyChannelSafeDefaults(cfg, channel, allowSend)

	// Write config
	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	// Restart (skip in test mode)
	if os.Getenv("CLAWS_SKIP_VALIDATE") == "" {
		info("Restarting gateway...")
		if err := dcRun(paths, ref.RegistryName(), "restart", gatewayService(paths, ref.RegistryName())); err != nil {
			return errorf("restart failed: %v", err)
		}
	}

	fmt.Println()
	info(fmt.Sprintf("Channel '%s' configured on '%s'.", channel, name))
	fmt.Println()
	fmt.Println("  Next step:")
	fmt.Printf("    1. Message the bot on %s\n", channel)
	fmt.Printf("    2. It will reply with a pairing code\n")
	fmt.Printf("    3. Run: claws approve %s %s <CODE>\n", name, channel)
	if !allowSend {
		fmt.Println()
		fmt.Println("  Outbound messaging is OFF by default. To enable:")
		fmt.Printf("    claws channel send %s %s --enable\n", name, channel)
	}
	fmt.Println()

	return nil
}

// cmdChannelAddWithLogin handles channels that need interactive login (WhatsApp QR).
func cmdChannelAddWithLogin(paths Paths, name, channel, dmPolicy string, allowSend bool) error {
	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	setNestedConfig(cfg, "channels."+channel+".enabled", true)
	setNestedConfig(cfg, "channels."+channel+".dmPolicy", dmPolicy)

	// Apply safe action defaults
	applyChannelSafeDefaults(cfg, channel, allowSend)

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	info("Restarting gateway...")
	dcRun(paths, ref.RegistryName(), "restart", gatewayService(paths, ref.RegistryName()))

	fmt.Println()
	info(fmt.Sprintf("Starting %s login flow...", channel))
	fmt.Println()

	// Run interactive login
	cmd := dc(paths, ref.RegistryName(), "run", "--rm", cliService(paths, ref.RegistryName()), "channels", "login", "--channel", channel)
	if err := cmd.Run(); err != nil {
		return errorf("%s login failed: %v", channel, err)
	}

	info("Restarting gateway...")
	dcRun(paths, ref.RegistryName(), "restart", gatewayService(paths, ref.RegistryName()))

	fmt.Println()
	info(fmt.Sprintf("Channel '%s' configured on '%s'.", channel, name))
	fmt.Println()
	fmt.Println("  Next step:")
	fmt.Printf("    1. Message the bot on %s\n", channel)
	fmt.Printf("    2. Run: claws approve %s %s <CODE>\n", name, channel)
	if !allowSend {
		fmt.Println()
		fmt.Println("  Outbound messaging is OFF by default. To enable:")
		fmt.Printf("    claws channel send %s %s --enable\n", name, channel)
	}
	fmt.Println()

	return nil
}

// cmdChannelLegacy is the old wizard-based path.
func cmdChannelLegacy(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws channel <name> <channel> <args...>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	channel := args[1]

	// Policy enforcement (same checks as cmdChannelAdd)
	policy := readPolicy(paths)
	if err := policy.enforceChannelPolicy(channel); err != nil {
		return err
	}

	info(fmt.Sprintf("Adding channel '%s' to '%s' (interactive wizard)...", channel, name))
	composeArgs := append([]string{"run", "--rm", "-T", cliService(paths, name), "channels", "add"}, args[1:]...)
	if err := dcRun(paths, name, composeArgs...); err != nil {
		return err
	}
	info("Restarting gateway...")
	dcRun(paths, name, "restart", gatewayService(paths, name))
	info(fmt.Sprintf("Channel '%s' added to '%s'.", channel, name))
	return nil
}

// cmdChannelRemove disables a channel.
func cmdChannelRemove(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws channel remove <instance> <channel>")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	setNestedConfig(cfg, "channels."+channel+".enabled", false)

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	if os.Getenv("CLAWS_SKIP_VALIDATE") == "" {
		info("Restarting gateway...")
		dcRun(paths, ref.RegistryName(), "restart", gatewayService(paths, ref.RegistryName()))
	}
	info(fmt.Sprintf("Channel '%s' disabled on '%s'.", channel, name))
	return nil
}

// cmdChannelStatus shows channel status for an instance.
func cmdChannelStatus(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws channel status <instance> [--json]")
	}
	paths := resolvePaths()
	name := firstPositional(args)
	jsonMode := hasFlag(args, "--json")
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	channels, ok := cfg["channels"].(map[string]any)
	if !ok || len(channels) == 0 {
		if jsonMode {
			fmt.Println("[]")
		} else {
			fmt.Println("No channels configured.")
		}
		return nil
	}

	// JSON mode: emit a flat array of {name, enabled, dmPolicy} per channel.
	if jsonMode {
		type chRec struct {
			Name     string `json:"name"`
			Enabled  bool   `json:"enabled"`
			DmPolicy string `json:"dmPolicy,omitempty"`
		}
		var out []chRec
		for ch, v := range channels {
			cm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			enabled, _ := cm["enabled"].(bool)
			dm, _ := cm["dmPolicy"].(string)
			out = append(out, chRec{Name: ch, Enabled: enabled, DmPolicy: dm})
		}
		// Sort for deterministic JSON output.
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"

	fmt.Printf("%sChannels for '%s':%s\n\n", bold, name, nc)
	for ch, v := range channels {
		chMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		enabled := false
		if e, ok := chMap["enabled"].(bool); ok {
			enabled = e
		}
		status := red + "disabled" + nc
		if enabled {
			status = green + "enabled" + nc
		}
		dmPolicy := "—"
		if p, ok := chMap["dmPolicy"].(string); ok {
			dmPolicy = p
		}
		fmt.Printf("  %-15s %s  (dm: %s)\n", ch, status, dmPolicy)
	}
	return nil
}

// ---------------------------------------------------------------------------
// claws approve — shortcut for pairing approval
// ---------------------------------------------------------------------------

func cmdApprove(args []string) error {
	if len(args) < 3 {
		return errorf("usage: claws approve <instance> <channel> <code>")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	code := args[2]

	if err := requireInstance(paths, name); err != nil {
		return err
	}

	rt, err := resolveRuntime(paths, name)
	if err != nil {
		return err
	}
	if err := rt.RequireCapability("pairing"); err != nil {
		return err
	}

	info(fmt.Sprintf("Approving %s pairing for '%s'...", channel, name))
	if err := dcRun(paths, name, "run", "--rm", "-T", cliService(paths, name), "pairing", "approve", channel, code); err != nil {
		return err
	}
	info(fmt.Sprintf("Pairing approved. '%s' can now reach '%s' via %s.", code, name, channel))
	return nil
}

// ---------------------------------------------------------------------------
// claws channel security — show security posture
// ---------------------------------------------------------------------------

func cmdChannelSecurity(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws channel security <instance> [<channel>] [--json]")
	}
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	// Drop flags so positional indexing still works.
	var positional []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	name := ""
	filterChannel := ""
	if len(positional) >= 1 {
		name = positional[0]
	}
	if len(positional) >= 2 {
		filterChannel = positional[1]
	}
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))
	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	channels, ok := cfg["channels"].(map[string]any)
	if !ok || len(channels) == 0 {
		if jsonMode {
			fmt.Println("[]")
		} else {
			fmt.Println("No channels configured.")
		}
		return nil
	}

	// JSON mode: emit the full security posture for matched channels.
	if jsonMode {
		var out []map[string]any
		for ch, v := range channels {
			cm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if filterChannel != "" && ch != filterChannel {
				continue
			}
			enabled, _ := cm["enabled"].(bool)
			if !enabled {
				continue
			}
			rec := map[string]any{
				"channel":  ch,
				"enabled":  enabled,
				"dmPolicy": cm["dmPolicy"],
				"groupPolicy": cm["groupPolicy"],
			}
			if af, ok := cm["allowFrom"].([]any); ok {
				rec["allowFrom"] = af
			}
			if gaf, ok := cm["groupAllowFrom"].([]any); ok {
				rec["groupAllowFrom"] = gaf
			}
			if actions, ok := cm["actions"].(map[string]any); ok {
				rec["actions"] = actions
			}
			out = append(out, rec)
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["channel"].(string) < out[j]["channel"].(string) })
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"
	yellow := "\033[0;33m"

	fmt.Printf("%sSecurity posture for '%s':%s\n", bold, name, nc)

	shown := 0
	for ch, v := range channels {
		if filterChannel != "" && ch != filterChannel {
			continue
		}
		chMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		enabled := false
		if e, ok := chMap["enabled"].(bool); ok {
			enabled = e
		}
		if !enabled {
			continue
		}

		shown++
		fmt.Printf("\n  %s%s%s\n", bold, ch, nc)

		// DM policy
		dmPolicy := "pairing"
		if p, ok := chMap["dmPolicy"].(string); ok {
			dmPolicy = p
		}
		fmt.Printf("    dm-policy:     %s\n", dmPolicy)

		// Group policy
		groupPolicy := "(not set)"
		if p, ok := chMap["groupPolicy"].(string); ok {
			groupPolicy = p
		}
		fmt.Printf("    group-policy:  %s\n", groupPolicy)

		// allowFrom
		if af, ok := chMap["allowFrom"].([]any); ok && len(af) > 0 {
			contacts := make([]string, len(af))
			for i, c := range af {
				contacts[i] = fmt.Sprintf("%v", c)
			}
			fmt.Printf("    allow-from:    %s\n", strings.Join(contacts, ", "))
		} else {
			fmt.Printf("    allow-from:    %s(none)%s\n", yellow, nc)
		}

		// groupAllowFrom
		if gaf, ok := chMap["groupAllowFrom"].([]any); ok && len(gaf) > 0 {
			contacts := make([]string, len(gaf))
			for i, c := range gaf {
				contacts[i] = fmt.Sprintf("%v", c)
			}
			fmt.Printf("    group-allow:   %s\n", strings.Join(contacts, ", "))
		}

		// Actions
		actions, hasActions := chMap["actions"].(map[string]any)
		if hasActions {
			fmt.Printf("    %sactions:%s\n", bold, nc)
			sendAction := channelSendAction(ch)
			for action, val := range actions {
				bval, isBool := val.(bool)
				if !isBool {
					continue
				}
				label := green + "ON" + nc
				if !bval {
					label = red + "OFF" + nc
				}
				marker := ""
				if action == sendAction || action == "moderation" || action == "permissions" || action == "roles" || action == "deleteMessage" {
					if bval {
						marker = " " + yellow + "(dangerous)" + nc
					}
				}
				fmt.Printf("      %-18s %s%s\n", action, label, marker)
			}
		} else {
			fmt.Printf("    actions:       %s(using defaults — outbound likely ON)%s\n", red, nc)
		}
	}

	if shown == 0 && filterChannel != "" {
		return errorf("channel '%s' not found or not enabled on '%s'", filterChannel, name)
	}

	fmt.Println()
	return nil
}

// ---------------------------------------------------------------------------
// claws channel send — toggle outbound messaging
// ---------------------------------------------------------------------------

func cmdChannelSend(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws channel send <instance> <channel> --enable|--disable")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	enable := false
	disable := false
	for _, a := range args[2:] {
		if a == "--enable" {
			enable = true
		}
		if a == "--disable" {
			disable = true
		}
	}
	if !enable && !disable {
		return errorf("usage: claws channel send %s %s --enable|--disable", name, channel)
	}
	if enable && disable {
		return errorf("cannot use both --enable and --disable")
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))
	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	// Verify channel exists and is enabled
	if err := requireChannelEnabled(cfg, channel); err != nil {
		return err
	}

	sendKey := channelSendAction(channel)

	if enable {
		// Warn if allowFrom is empty
		chMap, _ := cfg["channels"].(map[string]any)
		if chMap != nil {
			if ch, ok := chMap[channel].(map[string]any); ok {
				af, _ := ch["allowFrom"].([]any)
				if len(af) == 0 {
					fmt.Printf("\033[0;33m==> WARNING: No allowFrom contacts set for %s.\033[0m\n", channel)
					fmt.Printf("    The agent can send messages to anyone. Consider adding contacts first:\n")
					fmt.Printf("    claws channel allow %s %s <phone-or-id>\n\n", name, channel)
				}
			}
		}
		setNestedConfig(cfg, "channels."+channel+".actions."+sendKey, true)
		info(fmt.Sprintf("Outbound messaging ENABLED for %s on '%s'.", channel, name))
	} else {
		setNestedConfig(cfg, "channels."+channel+".actions."+sendKey, false)
		info(fmt.Sprintf("Outbound messaging DISABLED for %s on '%s'.", channel, name))
	}

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}
	fmt.Printf("  Restart to apply: claws restart %s\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// claws channel allow — add contacts to allowFrom
// ---------------------------------------------------------------------------

func cmdChannelAllow(args []string) error {
	if len(args) < 3 {
		return errorf("usage: claws channel allow <instance> <channel> <contact> [<contact>...]")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	contacts := args[2:]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))
	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	// Verify channel exists and is enabled
	if err := requireChannelEnabled(cfg, channel); err != nil {
		return err
	}

	// Read existing allowFrom
	existing := []any{}
	if channels, ok := cfg["channels"].(map[string]any); ok {
		if ch, ok := channels[channel].(map[string]any); ok {
			if af, ok := ch["allowFrom"].([]any); ok {
				existing = af
			}
		}
	}

	// Deduplicate and append
	seen := map[string]bool{}
	for _, c := range existing {
		seen[fmt.Sprintf("%v", c)] = true
	}
	added := 0
	for _, c := range contacts {
		if !seen[c] {
			existing = append(existing, c)
			seen[c] = true
			added++
		}
	}

	setNestedConfig(cfg, "channels."+channel+".allowFrom", existing)
	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	info(fmt.Sprintf("Added %d contact(s) to %s allowFrom on '%s'. Total: %d.", added, channel, name, len(existing)))
	fmt.Printf("  Restart to apply: claws restart %s\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// claws channel deny — remove contact from allowFrom
// ---------------------------------------------------------------------------

func cmdChannelDeny(args []string) error {
	if len(args) < 3 {
		return errorf("usage: claws channel deny <instance> <channel> <contact>")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	contact := args[2]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))
	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	// Verify channel exists and is enabled
	if err := requireChannelEnabled(cfg, channel); err != nil {
		return err
	}

	// Read existing allowFrom
	existing := []any{}
	if channels, ok := cfg["channels"].(map[string]any); ok {
		if ch, ok := channels[channel].(map[string]any); ok {
			if af, ok := ch["allowFrom"].([]any); ok {
				existing = af
			}
		}
	}

	// Remove matching contact
	filtered := []any{}
	removed := false
	for _, c := range existing {
		if fmt.Sprintf("%v", c) == contact {
			removed = true
		} else {
			filtered = append(filtered, c)
		}
	}

	if !removed {
		return errorf("contact '%s' not found in %s allowFrom on '%s'", contact, channel, name)
	}

	setNestedConfig(cfg, "channels."+channel+".allowFrom", filtered)
	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	info(fmt.Sprintf("Removed '%s' from %s allowFrom on '%s'. Remaining: %d.", contact, channel, name, len(filtered)))
	fmt.Printf("  Restart to apply: claws restart %s\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// Safe defaults helper
// ---------------------------------------------------------------------------

// requireChannelEnabled checks that a channel exists and is enabled in the config.
func requireChannelEnabled(cfg map[string]any, channel string) error {
	channels, ok := cfg["channels"].(map[string]any)
	if !ok {
		return errorf("no channels configured — add a channel first: claws channel add <instance> %s", channel)
	}
	ch, ok := channels[channel].(map[string]any)
	if !ok {
		return errorf("channel '%s' not configured — add it first: claws channel add <instance> %s", channel, channel)
	}
	enabled, _ := ch["enabled"].(bool)
	if !enabled {
		return errorf("channel '%s' is disabled", channel)
	}
	return nil
}

// applyChannelSafeDefaults sets secure action defaults and groupPolicy on a channel config.
func applyChannelSafeDefaults(cfg map[string]any, channel string, allowSend bool) {
	if defaults, ok := channelSafeDefaults[channel]; ok {
		for action, val := range defaults {
			setNestedConfig(cfg, "channels."+channel+".actions."+action, val)
		}
	}
	// Override send action if --allow-send was passed
	if allowSend {
		sendKey := channelSendAction(channel)
		setNestedConfig(cfg, "channels."+channel+".actions."+sendKey, true)
	}
	// Set group policy to allowlist
	setNestedConfig(cfg, "channels."+channel+".groupPolicy", "allowlist")
}

// ---------------------------------------------------------------------------
// Config helpers for direct JSON manipulation
// ---------------------------------------------------------------------------

func readInstanceConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeInstanceConfig(path string, cfg map[string]any) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// setNestedConfig sets a dotted path like "channels.telegram.enabled" to a value.
func setNestedConfig(cfg map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := cfg
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
}
