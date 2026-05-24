package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------------------
// claws setup — guided interactive onboarding
// ---------------------------------------------------------------------------

func cmdSetup(args []string) error {
	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"

	// Parse flags for non-interactive mode
	var teamName, agentName, authMode, channelType string
	var channelTokens []string
	nonInteractive := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--non-interactive":
			nonInteractive = true
		case strings.HasPrefix(a, "--team="):
			teamName = a[7:]
		case strings.HasPrefix(a, "--agent="):
			agentName = a[8:]
		case strings.HasPrefix(a, "--auth="):
			authMode = a[7:]
		case strings.HasPrefix(a, "--channel="):
			channelType = a[10:]
		case strings.HasPrefix(a, "--telegram-token="):
			channelTokens = append(channelTokens, a[17:])
		case strings.HasPrefix(a, "--discord-token="):
			channelTokens = append(channelTokens, a[16:])
		case strings.HasPrefix(a, "--slack-bot-token="):
			channelTokens = append(channelTokens, a[18:])
		}
	}

	reader := bufio.NewReader(os.Stdin)
	prompt := func(label, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("    %s [%s]: ", label, defaultVal)
		} else {
			fmt.Printf("    %s: ", label)
		}
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultVal
		}
		return line
	}

	// -----------------------------------------------------------------------
	// Welcome
	// -----------------------------------------------------------------------
	fmt.Printf("\n  %sWelcome to claws%s — AI agent team manager.\n\n", bold, nc)
	fmt.Println("  This will set up your server to run a team of AI agents.")
	fmt.Println("  Everything is stored locally. Agents connect to messaging")
	fmt.Println("  apps (Telegram, WhatsApp, Discord, etc.) so people can")
	fmt.Println("  message them.")
	fmt.Println()

	// -----------------------------------------------------------------------
	// Step 1: Check prerequisites
	// -----------------------------------------------------------------------
	fmt.Printf("  %s[1/6] Checking prerequisites...%s\n", bold, nc)

	// Docker
	if _, err := exec.LookPath("docker"); err != nil {
		return errorf("Docker not found — install: https://docs.docker.com/get-docker/")
	}
	if out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").Output(); err == nil {
		fmt.Printf("    %s✓%s Docker running (v%s)\n", green, nc, trimSpace(string(out)))
	} else {
		return errorf("Docker installed but not running — start the Docker daemon")
	}

	// Docker Compose
	if out, err := exec.Command("docker", "compose", "version", "--short").Output(); err == nil {
		fmt.Printf("    %s✓%s Docker Compose v%s\n", green, nc, trimSpace(string(out)))
	} else {
		return errorf("Docker Compose v2 not found — install it: https://docs.docker.com/compose/install/")
	}

	// Image
	paths := resolvePaths()
	image := os.Getenv("OPENCLAW_IMAGE")
	if image == "" {
		image = "openclaw:local"
	}
	if _, err := exec.Command("docker", "image", "inspect", image).Output(); err == nil {
		fmt.Printf("    %s✓%s Image %s found\n", green, nc, image)
	} else {
		// v1.6.4: offer to bootstrap the image inline. Non-technical users
		// shouldn't have to know what `claws image bootstrap` is.
		fmt.Printf("    %s!%s Image '%s' not found\n", "\033[0;33m", nc, image)
		if !nonInteractive {
			fmt.Println()
			fmt.Println("    This image is the AI runtime — every agent runs inside it.")
			fmt.Println("    Building takes 5-10 minutes the first time; future runs are instant.")
			ans := strings.ToLower(prompt("    Build openclaw:local now? (Y/n)", "y"))
			if ans == "y" || ans == "yes" {
				fmt.Println()
				if err := cmdImageBootstrap([]string{"--yes"}); err != nil {
					return errorf("image bootstrap failed: %v", err)
				}
			} else {
				warn("agents won't start without the image — run `claws image bootstrap --yes` later")
			}
		} else {
			warn("Image not present and --non-interactive set — run `claws image bootstrap --yes` first")
		}
	}

	// Disk space
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
		fmt.Printf("    %s✓%s %.0f GB free disk\n", green, nc, freeGB)
	}
	fmt.Println()

	// -----------------------------------------------------------------------
	// Step 2: Create workspace (init + policy + access)
	// -----------------------------------------------------------------------
	fmt.Printf("  %s[2/6] Creating workspace...%s\n", bold, nc)

	// Run init logic inline (silent — we print our own output)
	if err := os.MkdirAll(paths.Root, 0755); err != nil {
		return errorf("failed to create %s: %v", paths.Root, err)
	}
	fmt.Printf("    %s✓%s %s\n", green, nc, paths.Root)

	// Subdirs
	for _, sub := range []string{"shared/skills", "shared/workspace"} {
		os.MkdirAll(filepath.Join(paths.Root, sub), 0755)
	}

	// Port registry
	if _, err := os.Stat(paths.PortRegistry); os.IsNotExist(err) {
		os.WriteFile(paths.PortRegistry, nil, credentialFileMode)
	}

	// Compose template
	composeDest := filepath.Join(paths.Root, "docker-compose.yml")
	if _, err := os.Stat(composeDest); os.IsNotExist(err) {
		setupCopyComposeTemplate(composeDest)
	}

	// Defaults skeleton
	defaultsPath := filepath.Join(paths.Root, "defaults.json")
	if _, err := os.Stat(defaultsPath); os.IsNotExist(err) {
		os.WriteFile(defaultsPath, []byte("{\n  \"tools\": {},\n  \"agents\": {\n    \"defaults\": {}\n  }\n}\n"), 0644)
	}

	// Policy
	if !policyExists(paths) {
		p := Policy{
			AllowedBindModes:         []string{"loopback"},
			MaxInstances:             8,
			MemoryLimitMB:            2048,
			CPULimit:                 2.0,
			AllowDockerSocket:        false,
			RequireSandbox:           false,
			RequireDmPairing:         true,
			RequireOutboundAllowlist: true,
			AllowedImages:            []string{"openclaw:*"},
			AuditLog:                 true,
		}
		if err := writePolicy(paths, p); err != nil {
			return errorf("failed to create policy: %v", err)
		}
	}
	fmt.Printf("    %s✓%s Security policy (loopback-only, DM pairing required)\n", green, nc)

	// Access
	if !accessExists(paths) {
		username := os.Getenv("USER")
		if username == "" {
			username = "ubuntu"
		}
		ac := AccessConfig{
			Roles: map[string]Role{
				"admin": {
					Users:    []string{username},
					Commands: []string{"*"},
				},
				"operator": {
					Users: []string{},
					Commands: []string{
						"start", "stop", "restart", "logs", "exec", "health",
						"status", "list", "dashboard", "activity", "stats",
						"config show", "channel status", "tunnel", "backup",
					},
				},
				"user": {
					Users:    []string{},
					Commands: []string{"status", "health", "logs", "list"},
				},
			},
		}
		if err := writeAccessConfig(paths, ac); err != nil {
			return errorf("failed to create access config: %v", err)
		}
	}
	fmt.Printf("    %s✓%s Access control (you are admin)\n", green, nc)
	fmt.Printf("    %s✓%s Audit logging enabled\n", green, nc)
	fmt.Println()

	// -----------------------------------------------------------------------
	// Step 3: Team name
	// -----------------------------------------------------------------------
	fmt.Printf("  %s[3/6] Team name%s\n", bold, nc)
	if teamName == "" {
		if nonInteractive {
			return errorf("--team=<name> required in non-interactive mode")
		}
		teamName = prompt("What's your team name?", "my-team")
	} else {
		fmt.Printf("    Team: %s\n", teamName)
	}
	if err := validateName(teamName); err != nil {
		return errorf("invalid team name: %v", err)
	}

	// Create group
	groupDir := filepath.Join(paths.Root, teamName)
	if !IsGroup(groupDir) {
		if err := os.MkdirAll(groupDir, 0755); err != nil {
			return errorf("failed to create group dir: %v", err)
		}
		gc := GroupConfig{Name: teamName}
		gcData, _ := json.MarshalIndent(gc, "", "  ")
		os.WriteFile(filepath.Join(groupDir, groupConfigFile), append(gcData, '\n'), 0644)
		// Group shared dirs
		for _, sub := range []string{"shared/skills", "shared/workspace", "shared/hooks", "tasks"} {
			os.MkdirAll(filepath.Join(groupDir, sub), 0755)
		}
	}
	fmt.Printf("    %s✓%s Group '%s' ready\n", green, nc, teamName)
	fmt.Println()

	// -----------------------------------------------------------------------
	// Agent creation loop
	// -----------------------------------------------------------------------
	agentNum := 0
	for {
		agentNum++

		// Step 4: Agent name
		if agentNum == 1 {
			fmt.Printf("  %s[4/6] Create your first agent%s\n", bold, nc)
		} else {
			fmt.Printf("  %sCreate agent #%d%s\n", bold, agentNum, nc)
		}
		currentAgent := agentName
		if currentAgent == "" {
			if nonInteractive {
				if agentNum == 1 {
					return errorf("--agent=<name> required in non-interactive mode")
				}
				break // no more agents specified
			}
			currentAgent = prompt("Agent name", fmt.Sprintf("agent-%d", agentNum))
		} else if agentNum > 1 {
			break // non-interactive only creates one agent
		}

		if err := validateName(currentAgent); err != nil {
			return errorf("invalid agent name: %v", err)
		}

		fullName := teamName + "/" + currentAgent

		// Create instance via cmdCreate (quiet mode — setup handles its own output)
		quietCreate = true
		createArgs := []string{fullName}
		if err := cmdCreate(createArgs); err != nil {
			quietCreate = false
			return errorf("failed to create agent: %v", err)
		}
		quietCreate = false
		fmt.Printf("    %s✓%s Agent '%s' created\n", green, nc, currentAgent)
		fmt.Println()

		// Step 5: Auth
		if agentNum == 1 {
			fmt.Printf("  %s[5/6] Authentication for %s%s\n", bold, currentAgent, nc)
		} else {
			fmt.Printf("  %sAuthentication for %s%s\n", bold, currentAgent, nc)
		}
		currentAuth := authMode
		var detectedProvider, detectedKey string // populated if user picks a detected env key

		if currentAuth == "" {
			if nonInteractive {
				fmt.Println("    Skipping auth (not specified)")
			} else {
				// v1.6.6: pre-flight scan for credentials already on this
				// host. Non-technical users often have ANTHROPIC_API_KEY
				// in env, an existing claws agent, or sibling-CLI OAuth.
				// Surfacing those first means we don't re-ask for what
				// they already have.
				det := detectExistingAuth()

				// Build the menu dynamically. Detected options come first
				// (lowest friction), then the manual choices.
				type menuItem struct {
					label string
					auth  string // "codex", "apikey", or "skip"
					prov  string // for apikey-from-env: which provider
					key   string // for apikey-from-env: the actual key
				}
				var items []menuItem

				if det.EnvAnthropicKey != "" {
					items = append(items, menuItem{
						label: "Use $ANTHROPIC_API_KEY from your environment",
						auth:  "apikey-env", prov: "anthropic", key: det.EnvAnthropicKey,
					})
				}
				if det.EnvOpenAIKey != "" {
					items = append(items, menuItem{
						label: "Use $OPENAI_API_KEY from your environment",
						auth:  "apikey-env", prov: "openai", key: det.EnvOpenAIKey,
					})
				}
				if det.EnvOpenRouterKey != "" {
					items = append(items, menuItem{
						label: "Use $OPENROUTER_API_KEY from your environment",
						auth:  "apikey-env", prov: "openrouter", key: det.EnvOpenRouterKey,
					})
				}
				// Standard options always present.
				items = append(items,
					menuItem{label: "Codex (OAuth — recommended)", auth: "codex"},
					menuItem{label: "API key (paste manually)", auth: "apikey"},
					menuItem{label: "Skip for now", auth: "skip"},
				)

				// Informational hints (not selectable items) — surface
				// OAuth tokens and existing-agent presence so the
				// operator knows what's already done.
				if det.ClaudeCodeOAuth != "" || det.CodexOAuth != "" || len(det.ExistingAgents) > 0 {
					fmt.Println("    Detected on this system:")
					if det.ClaudeCodeOAuth != "" {
						fmt.Printf("      • Claude Code OAuth (%s) — not auto-reused (tied to your CLI identity)\n", shortenHome(det.ClaudeCodeOAuth))
					}
					if det.CodexOAuth != "" {
						fmt.Printf("      • Codex CLI OAuth (%s) — pick option for fresh agent OAuth if you want the same\n", shortenHome(det.CodexOAuth))
					}
					if len(det.ExistingAgents) > 0 {
						for _, a := range det.ExistingAgents {
							fmt.Printf("      • Existing agent %s/%s (%s) — already has: %s\n",
								a.Team, a.Name, shortenHome(a.Workspace), strings.Join(a.HasKeys, ", "))
						}
					}
					fmt.Println()
				}

				for i, it := range items {
					fmt.Printf("    %d. %s\n", i+1, it.label)
				}
				choice := prompt("Choice", "1")
				idx := 0
				if _, perr := fmt.Sscanf(choice, "%d", &idx); perr != nil || idx < 1 || idx > len(items) {
					idx = 1
				}
				picked := items[idx-1]
				currentAuth = picked.auth
				if picked.auth == "apikey-env" {
					detectedProvider, detectedKey = picked.prov, picked.key
				}
			}
		}

		switch currentAuth {
		case "codex":
			fmt.Println("    Starting OAuth flow...")
			if err := cmdAuth([]string{fullName, "codex"}); err != nil {
				warn(fmt.Sprintf("Auth failed: %v — you can retry later: claws auth %s codex", err, fullName))
			} else {
				fmt.Printf("    %s✓%s Auth complete.\n", green, nc)
			}
		case "apikey-env":
			fmt.Printf("    Using detected %s key from environment...\n", detectedProvider)
			if err := cmdAuth([]string{fullName, "apikey", detectedProvider, detectedKey}); err != nil {
				warn(fmt.Sprintf("Auth failed: %v", err))
			} else {
				fmt.Printf("    %s✓%s API key configured.\n", green, nc)
			}
		case "apikey":
			if nonInteractive {
				// Look for provider key in remaining args
				for i := 0; i < len(args); i++ {
					if strings.HasPrefix(args[i], "--anthropic-key=") {
						key := args[i][16:]
						cmdAuth([]string{fullName, "apikey", "anthropic", key})
						fmt.Printf("    %s✓%s API key configured.\n", green, nc)
						break
					}
				}
			} else {
				provider := prompt("Provider (anthropic/openai)", "anthropic")
				fmt.Printf("    API key: ")
				line, _ := reader.ReadString('\n')
				key := strings.TrimSpace(line)
				if key != "" {
					if err := cmdAuth([]string{fullName, "apikey", provider, key}); err != nil {
						warn(fmt.Sprintf("Auth failed: %v", err))
					} else {
						fmt.Printf("    %s✓%s API key configured.\n", green, nc)
					}
				}
			}
		}
		fmt.Println()

		// Step 6: Channel
		if agentNum == 1 {
			fmt.Printf("  %s[6/6] Connect a channel for %s%s\n", bold, currentAgent, nc)
		} else {
			fmt.Printf("  %sConnect a channel for %s%s\n", bold, currentAgent, nc)
		}
		currentChannel := channelType
		if currentChannel == "" {
			if nonInteractive {
				fmt.Println("    Skipping channel (not specified)")
			} else {
				fmt.Println("    1. Telegram  (get token: t.me/BotFather)")
				fmt.Println("    2. Discord   (get token: discord.com/developers/applications)")
				fmt.Println("    3. Slack     (get tokens: api.slack.com/apps)")
				fmt.Println("    4. WhatsApp  (QR scan — need a dedicated phone number)")
				fmt.Println("    5. Skip for now")
				choice := prompt("Choice", "1")
				switch choice {
				case "1":
					currentChannel = "telegram"
				case "2":
					currentChannel = "discord"
				case "3":
					currentChannel = "slack"
				case "4":
					currentChannel = "whatsapp"
				}
			}
		}

		if currentChannel != "" && currentChannel != "skip" {
			var token string
			if len(channelTokens) > 0 {
				token = channelTokens[0]
				channelTokens = channelTokens[1:]
			} else if !nonInteractive {
				if currentChannel != "whatsapp" {
					// v1.6.4: tokens are 46+ chars from BotFather/etc — typing
					// them into SSH is painful. Offer the paste-secret bridge:
					// open a URL on phone, paste there, server receives.
					fmt.Println()
					fmt.Println("    How do you want to enter the bot token?")
					fmt.Println("      1. Paste here  (good if you've got the token in your clipboard)")
					fmt.Println("      2. Phone-paste (open a URL on your phone, paste there — easier from BotFather)")
					how := prompt("    Choice", "1")
					if how == "2" {
						secretName := currentChannel + ".token"
						secretsDir := "/tmp/claws-secrets"
						_ = os.MkdirAll(secretsDir, 0700)
						fmt.Println()
						fmt.Printf("    Starting paste-secret listener for %s ...\n", secretName)
						fmt.Println("    Open the URL printed below on your phone, enter the code, paste the token.")
						fmt.Println()
						if err := cmdPasteSecret([]string{secretName, "--secrets-dir=" + secretsDir, "--timeout=10m"}); err != nil {
							warn(fmt.Sprintf("paste-secret failed: %v — falling back to inline paste", err))
						} else {
							// Read it back
							token = readSecretFile(filepath.Join(secretsDir, secretName))
						}
					}
					if token == "" {
						fmt.Printf("    Paste token here: ")
						line, _ := reader.ReadString('\n')
						token = strings.TrimSpace(line)
					}
				}
			}

			channelArgs := []string{"add", fullName, currentChannel}
			if token != "" {
				channelArgs = append(channelArgs, "--token="+token)
			}
			if err := cmdChannel(channelArgs); err != nil {
				warn(fmt.Sprintf("Channel setup failed: %v — retry later: claws channel add %s %s", err, fullName, currentChannel))
			} else {
				fmt.Printf("    %s✓%s %s configured.\n", green, nc, currentChannel)
			}
		}
		fmt.Println()

		// Start the agent
		fmt.Printf("  Starting %s...\n", currentAgent)
		if err := cmdStart([]string{fullName}); err != nil {
			warn(fmt.Sprintf("Start failed: %v — start manually: claws start %s", err, fullName))
		} else {
			ref, _ := ParseRef(fullName)
			envFile := filepath.Join(ref.Dir(paths), "instance.env")
			port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
			fmt.Printf("    %s✓%s Healthy on :%s\n", green, nc, port)
			if currentChannel != "" && currentChannel != "skip" {
				fmt.Printf("    %s✓%s %s connected\n", green, nc, currentChannel)
				fmt.Println()
				fmt.Printf("    Approve the pairing code: claws approve %s %s <CODE>\n", fullName, currentChannel)
			}
		}
		fmt.Println()

		// Add another?
		if nonInteractive {
			break
		}
		another := prompt("Add another agent? (y/N)", "n")
		if strings.ToLower(another) != "y" {
			break
		}
		fmt.Println()
		// Reset per-agent values for the next iteration
		agentName = ""
		authMode = ""
		channelType = ""
	}

	// -----------------------------------------------------------------------
	// Summary
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Printf("  %sDone. Your team:%s\n", bold, nc)

	// List instances in this team
	entries, _ := readRegistry(paths)
	for _, e := range entries {
		if strings.HasPrefix(e.Name, teamName+"/") {
			ref, _ := ParseRef(e.Name)
			dir := ref.Dir(paths)
			envFile := filepath.Join(dir, "instance.env")
			port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
			h := probeInstance(paths, e.Name)
			// Detect configured channels
			var channels []string
			rt := mustResolveRuntime(paths, e.Name)
			if cfg, err := readInstanceConfig(rt.ConfigPath(dir)); err == nil {
				if chs, ok := cfg["channels"].(map[string]any); ok {
					for ch, v := range chs {
						if chMap, ok := v.(map[string]any); ok {
							if enabled, _ := chMap["enabled"].(bool); enabled {
								channels = append(channels, ch)
							}
						}
					}
				}
			}
			chStr := ""
			if len(channels) > 0 {
				chStr = strings.Join(channels, ",")
			}
			fmt.Printf("    %-25s :%s  %-10s %s\n", e.Name, port, h.Verdict, chStr)
		}
	}

	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    claws list              — see all agents\n")
	fmt.Printf("    claws status            — system overview\n")
	fmt.Printf("    claws dashboard         — live status view\n")
	fmt.Printf("    claws audit             — security check\n")
	fmt.Printf("    claws setup             — add more agents\n")
	fmt.Println()

	return nil
}

// setupCopyComposeTemplate finds and copies docker-compose.yml into OPENCLAW_ROOT.
func setupCopyComposeTemplate(dest string) {
	exe, _ := os.Executable()
	candidates := []string{}
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "docker-compose.yml"))
	}
	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, "docker-compose.yml"))
	}
	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			os.WriteFile(dest, data, 0644)
			return
		}
	}
}
