package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// claws config — view and edit instance configuration
// ---------------------------------------------------------------------------

func cmdConfig(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws config <show|edit|get|set> <instance> [args...]")
	}
	switch args[0] {
	case "show":
		return cmdConfigShow(args[1:])
	case "edit":
		return cmdConfigEdit(args[1:])
	case "get":
		return cmdConfigGet(args[1:])
	case "set":
		return cmdConfigSet(args[1:])
	default:
		return errorf("unknown config subcommand: %s — use show, edit, get, or set", args[0])
	}
}

// cmdConfigShow prints the full merged config for an instance.
func cmdConfigShow(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws config show <instance> [--raw|--no-secrets]")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	noSecrets := hasFlag(args[1:], "--no-secrets")

	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	configPath := rt.ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	if noSecrets {
		maskSecrets(cfg)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// cmdConfigGet reads a specific dotted path from the config.
func cmdConfigGet(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws config get <instance> <path>  (e.g., channels.telegram.enabled)")
	}
	paths := resolvePaths()
	name := args[0]
	keyPath := args[1]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	configPath := rt.ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	val := getNestedConfig(cfg, keyPath)
	if val == nil {
		return errorf("key '%s' not found", keyPath)
	}

	switch v := val.(type) {
	case string:
		fmt.Println(v)
	case bool:
		fmt.Println(v)
	case float64:
		if v == float64(int(v)) {
			fmt.Printf("%d\n", int(v))
		} else {
			fmt.Println(v)
		}
	default:
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

// cmdConfigSet sets a specific dotted path in the config.
func cmdConfigSet(args []string) error {
	if len(args) < 3 {
		return errorf("usage: claws config set <instance> <path> <value>")
	}
	paths := resolvePaths()
	name := args[0]
	keyPath := args[1]
	rawValue := args[2]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	configPath := rt.ConfigPath(ref.Dir(paths))

	cfg, err := readInstanceConfig(configPath)
	if err != nil {
		return errorf("failed to read config: %v", err)
	}

	// Parse value: try JSON first, then treat as string
	var value any
	if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
		value = rawValue // treat as plain string
	}

	setNestedConfig(cfg, keyPath, value)

	if err := writeInstanceConfig(configPath, cfg); err != nil {
		return errorf("failed to write config: %v", err)
	}

	info(fmt.Sprintf("Set %s = %v", keyPath, value))
	fmt.Println("  Restart to apply: claws restart " + name)
	return nil
}

// cmdConfigEdit opens the config in $EDITOR.
func cmdConfigEdit(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws config edit <instance>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	configPath := rt.ConfigPath(ref.Dir(paths))

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorf("editor failed: %v", err)
	}

	info("Config edited. Restart to apply: claws restart " + name)
	return nil
}

// getNestedConfig reads a dotted path like "channels.telegram.enabled".
func getNestedConfig(cfg map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = cfg
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// maskSecrets replaces sensitive values with "***".
func maskSecrets(cfg map[string]any) {
	sensitiveKeys := map[string]bool{
		"token": true, "botToken": true, "appToken": true,
		"apiKey": true, "secret": true, "password": true,
		"signingSecret": true, "webhookSecret": true,
	}

	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		for k, v := range m {
			if sensitiveKeys[k] {
				if s, ok := v.(string); ok && len(s) > 0 {
					if len(s) > 8 {
						m[k] = s[:4] + "***" + s[len(s)-4:]
					} else {
						m[k] = "***"
					}
				}
			} else if sub, ok := v.(map[string]any); ok {
				walk(sub)
			}
		}
	}
	walk(cfg)
}
