package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// channelProfile defines what config keys each channel needs.
type channelProfile struct {
	Name        string
	RequiredKey string   // the primary credential flag name (e.g., "--token")
	ConfigKeys  []configPair // config keys to set from flags
	LoginFlow   bool     // true if channel needs interactive login (e.g., WhatsApp QR)
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
		Name:        "signal",
		RequiredKey: "--number",
		ConfigKeys: []configPair{
			{"--number", "channels.signal.account"},
		},
	},
	"whatsapp": {
		Name:      "whatsapp",
		LoginFlow: true,
	},
}

// ---------------------------------------------------------------------------
// clawctl channel — add/remove/list channels
// ---------------------------------------------------------------------------

func cmdChannel(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl channel add <instance> <channel> [--token=...]\n       clawctl channel <instance> <channel> [args...]  (wizard)")
	}

	// New subcommand style: clawctl channel add <instance> <channel> --token=...
	if args[0] == "add" {
		return cmdChannelAdd(args[1:])
	}
	if args[0] == "remove" {
		return cmdChannelRemove(args[1:])
	}
	if args[0] == "status" {
		return cmdChannelStatus(args[1:])
	}

	// Legacy style: clawctl channel <instance> <channel> [args...]
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
		return errorf("usage: clawctl channel add <instance> <channel> [--token=...] [--dm-policy=pairing]")
	}

	paths := resolvePaths()
	name := args[0]
	channel := args[1]

	if err := requireInstance(paths, name); err != nil {
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
	dmPolicy := "pairing"
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--dm-policy=") {
			dmPolicy = a[12:]
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
		return cmdChannelAddWithLogin(paths, name, channel, dmPolicy)
	}

	// Check required credential
	if _, ok := flags[profile.RequiredKey]; !ok {
		// Slack needs two tokens
		if channel == "slack" {
			return errorf("usage: clawctl channel add %s slack --bot-token=xoxb-... --app-token=xapp-...", name)
		}
		return errorf("usage: clawctl channel add %s %s %s=<value>", name, channel, profile.RequiredKey)
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
	configPath := filepath.Join(ref.Dir(paths), "openclaw.json")

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

	// Write config
	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	// Restart (skip in test mode)
	if os.Getenv("CLAWCTL_SKIP_VALIDATE") == "" {
		info("Restarting gateway...")
		if err := dcRun(paths, ref.RegistryName(), "restart", "openclaw-gateway"); err != nil {
			return errorf("restart failed: %v", err)
		}
	}

	fmt.Println()
	info(fmt.Sprintf("Channel '%s' configured on '%s'.", channel, name))
	fmt.Println()
	fmt.Println("  Next step:")
	fmt.Printf("    1. Message the bot on %s\n", channel)
	fmt.Printf("    2. It will reply with a pairing code\n")
	fmt.Printf("    3. Run: clawctl approve %s %s <CODE>\n", name, channel)
	fmt.Println()

	return nil
}

// cmdChannelAddWithLogin handles channels that need interactive login (WhatsApp QR).
func cmdChannelAddWithLogin(paths Paths, name, channel, dmPolicy string) error {
	ref, _ := ParseRef(name)
	configPath := filepath.Join(ref.Dir(paths), "openclaw.json")

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	setNestedConfig(cfg, "channels."+channel+".enabled", true)
	setNestedConfig(cfg, "channels."+channel+".dmPolicy", dmPolicy)

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	info("Restarting gateway...")
	dcRun(paths, ref.RegistryName(), "restart", "openclaw-gateway")

	fmt.Println()
	info(fmt.Sprintf("Starting %s login flow...", channel))
	fmt.Println()

	// Run interactive login
	cmd := dc(paths, ref.RegistryName(), "run", "--rm", "openclaw-cli", "channels", "login", "--channel", channel)
	if err := cmd.Run(); err != nil {
		return errorf("%s login failed: %v", channel, err)
	}

	info("Restarting gateway...")
	dcRun(paths, ref.RegistryName(), "restart", "openclaw-gateway")

	fmt.Println()
	info(fmt.Sprintf("Channel '%s' configured on '%s'.", channel, name))
	fmt.Println()
	fmt.Println("  Next step:")
	fmt.Printf("    1. Message the bot on %s\n", channel)
	fmt.Printf("    2. Run: clawctl approve %s %s <CODE>\n", name, channel)
	fmt.Println()

	return nil
}

// cmdChannelLegacy is the old wizard-based path.
func cmdChannelLegacy(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl channel <name> <channel> <args...>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	channel := args[1]
	info(fmt.Sprintf("Adding channel '%s' to '%s' (interactive wizard)...", channel, name))
	composeArgs := append([]string{"run", "--rm", "-T", "openclaw-cli", "channels", "add"}, args[1:]...)
	if err := dcRun(paths, name, composeArgs...); err != nil {
		return err
	}
	info("Restarting gateway...")
	dcRun(paths, name, "restart", "openclaw-gateway")
	info(fmt.Sprintf("Channel '%s' added to '%s'.", channel, name))
	return nil
}

// cmdChannelRemove disables a channel.
func cmdChannelRemove(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl channel remove <instance> <channel>")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := filepath.Join(ref.Dir(paths), "openclaw.json")

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	setNestedConfig(cfg, "channels."+channel+".enabled", false)

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	if os.Getenv("CLAWCTL_SKIP_VALIDATE") == "" {
		info("Restarting gateway...")
		dcRun(paths, ref.RegistryName(), "restart", "openclaw-gateway")
	}
	info(fmt.Sprintf("Channel '%s' disabled on '%s'.", channel, name))
	return nil
}

// cmdChannelStatus shows channel status for an instance.
func cmdChannelStatus(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl channel status <instance>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	configPath := filepath.Join(ref.Dir(paths), "openclaw.json")

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	channels, ok := cfg["channels"].(map[string]any)
	if !ok || len(channels) == 0 {
		fmt.Println("No channels configured.")
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
// clawctl approve — shortcut for pairing approval
// ---------------------------------------------------------------------------

func cmdApprove(args []string) error {
	if len(args) < 3 {
		return errorf("usage: clawctl approve <instance> <channel> <code>")
	}
	paths := resolvePaths()
	name := args[0]
	channel := args[1]
	code := args[2]

	if err := requireInstance(paths, name); err != nil {
		return err
	}

	info(fmt.Sprintf("Approving %s pairing for '%s'...", channel, name))
	if err := dcRun(paths, name, "run", "--rm", "-T", "openclaw-cli", "pairing", "approve", channel, code); err != nil {
		return err
	}
	info(fmt.Sprintf("Pairing approved. '%s' can now reach '%s' via %s.", code, name, channel))
	return nil
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
