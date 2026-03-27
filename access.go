package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AccessConfig defines who can do what.
type AccessConfig struct {
	Roles map[string]Role `json:"roles"`
}

// Role defines a set of permissions.
type Role struct {
	Users     []string `json:"users"`               // OS usernames
	Commands  []string `json:"commands"`             // allowed commands ("*" = all)
	Instances []string `json:"instances,omitempty"`   // allowed instances (empty = all)
}

const accessFile = ".access.json"

func readAccessConfig(paths Paths) *AccessConfig {
	data, err := os.ReadFile(filepath.Join(paths.Root, accessFile))
	if err != nil {
		return nil // no access control = unrestricted
	}
	var ac AccessConfig
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil
	}
	return &ac
}

func writeAccessConfig(paths Paths, ac AccessConfig) error {
	data, err := json.MarshalIndent(ac, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.Root, accessFile), append(data, '\n'), credentialFileMode)
}

func accessExists(paths Paths) bool {
	_, err := os.Stat(filepath.Join(paths.Root, accessFile))
	return err == nil
}

// resolveRole finds the role for the current OS user.
func resolveRole(ac *AccessConfig, username string) (string, *Role) {
	if ac == nil {
		return "", nil
	}
	for roleName, role := range ac.Roles {
		for _, u := range role.Users {
			if u == username {
				return roleName, &role
			}
		}
	}
	return "", nil
}

// enforceAccess checks if the current user can run a command on a target instance.
func enforceAccess(paths Paths, cmd string, args []string) error {
	ac := readAccessConfig(paths)
	if ac == nil {
		return nil // no access config = unrestricted
	}

	username := os.Getenv("USER")
	if username == "" {
		username = "unknown"
	}

	roleName, role := resolveRole(ac, username)
	if role == nil {
		return fmt.Errorf("access denied: user '%s' has no role assigned. Contact an admin.", username)
	}

	// Check command permission
	if !roleCanRunCommand(role, cmd) {
		return fmt.Errorf("access denied: user '%s' (role: %s) cannot run '%s'", username, roleName, cmd)
	}

	// Check instance scope
	if len(role.Instances) > 0 && len(args) > 0 {
		target := args[0]
		if !strings.HasPrefix(target, "--") {
			if !roleCanAccessInstance(role, target) {
				return fmt.Errorf("access denied: user '%s' (role: %s) cannot access instance '%s'", username, roleName, target)
			}
		}
	}

	return nil
}

func roleCanRunCommand(role *Role, cmd string) bool {
	for _, c := range role.Commands {
		if c == "*" || c == cmd {
			return true
		}
	}
	return false
}

func roleCanAccessInstance(role *Role, instance string) bool {
	if len(role.Instances) == 0 {
		return true // no restriction
	}
	for _, i := range role.Instances {
		if i == instance || matchGlob(i, instance) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Audit logging
// ---------------------------------------------------------------------------

const auditLogFile = ".audit.log"

func writeAuditLog(paths Paths, cmd string, args []string, result string) {
	policy := readPolicy(paths)
	if !policy.AuditLog {
		return
	}

	username := os.Getenv("USER")
	if username == "" {
		username = "unknown"
	}

	entry := map[string]any{
		"ts":     time.Now().UTC().Format(time.RFC3339),
		"user":   username,
		"cmd":    cmd,
		"args":   args,
		"result": result,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	logPath := filepath.Join(paths.Root, auditLogFile)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, credentialFileMode)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// ---------------------------------------------------------------------------
// Token rotation
// ---------------------------------------------------------------------------

func cmdToken(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl token <rotate|show> <instance>")
	}
	switch args[0] {
	case "rotate":
		return cmdTokenRotate(args[1:])
	case "show":
		return cmdTokenShow(args[1:])
	default:
		return errorf("unknown token subcommand: %s", args[0])
	}
}

func cmdTokenRotate(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl token rotate <instance>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	envFile := filepath.Join(ref.Dir(paths), "instance.env")
	configPath := mustResolveRuntime(paths, name).ConfigPath(ref.Dir(paths))

	oldToken := readEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN")
	newToken := generateToken()

	// Update instance.env
	updateEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN", newToken)

	// Update config gateway.auth.token
	if cfg, err := readInstanceConfig(configPath); err == nil {
		setNestedConfig(cfg, "gateway.auth.token", newToken)
		writeInstanceConfig(configPath, cfg)
	}

	info(fmt.Sprintf("Token rotated for '%s'", name))
	if len(oldToken) > 8 {
		fmt.Printf("  Old: %s...%s\n", oldToken[:4], oldToken[len(oldToken)-4:])
	}
	fmt.Printf("  New: %s...%s\n", newToken[:4], newToken[len(newToken)-4:])
	fmt.Printf("  Restart to apply: clawctl restart %s\n", name)

	return nil
}

func cmdTokenShow(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl token show <instance>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	ref, _ := ParseRef(name)
	envFile := filepath.Join(ref.Dir(paths), "instance.env")
	token := readEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN")

	if hasFlag(args[1:], "--full") {
		fmt.Println(token)
	} else if len(token) > 16 {
		fmt.Printf("%s...%s\n", token[:8], token[len(token)-8:])
		fmt.Println("  Use --full to show the complete token")
	} else {
		fmt.Println(token)
	}
	return nil
}

// ---------------------------------------------------------------------------
// clawctl access — manage access control
// ---------------------------------------------------------------------------

func cmdAccess(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl access <init|show|grant|revoke|audit>")
	}
	switch args[0] {
	case "init":
		return cmdAccessInit(args[1:])
	case "show":
		return cmdAccessShow(args[1:])
	case "grant":
		return cmdAccessGrant(args[1:])
	case "revoke":
		return cmdAccessRevoke(args[1:])
	case "audit":
		return cmdAccessAudit(args[1:])
	default:
		return errorf("unknown access subcommand: %s", args[0])
	}
}

func cmdAccessInit(args []string) error {
	paths := resolvePaths()
	if accessExists(paths) && !hasFlag(args, "--force") {
		return errorf(".access.json already exists. Use --force to overwrite.")
	}

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
		return err
	}

	info("Access control initialized.")
	fmt.Printf("  Admin: %s\n", username)
	fmt.Printf("  File:  %s\n", filepath.Join(paths.Root, accessFile))
	fmt.Println()
	fmt.Println("  Grant access:  clawctl access grant <user> <admin|operator|user>")
	fmt.Println("  Scope to instance: edit .access.json and add \"instances\": [\"team/alice\"]")
	return nil
}

func cmdAccessShow(args []string) error {
	paths := resolvePaths()
	if !accessExists(paths) {
		fmt.Println("No access control configured. Run: clawctl access init")
		return nil
	}
	ac := readAccessConfig(paths)
	data, _ := json.MarshalIndent(ac, "", "  ")
	fmt.Println(string(data))
	return nil
}

func cmdAccessGrant(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl access grant <username> <admin|operator|user>")
	}
	paths := resolvePaths()
	username := args[0]
	roleName := args[1]

	if !accessExists(paths) {
		return errorf("no access control configured — run: clawctl access init")
	}

	ac := readAccessConfig(paths)
	role, ok := ac.Roles[roleName]
	if !ok {
		return errorf("unknown role '%s' — use admin, operator, or user", roleName)
	}

	// Check not already in role
	for _, u := range role.Users {
		if u == username {
			info(fmt.Sprintf("'%s' already has role '%s'", username, roleName))
			return nil
		}
	}

	// Remove from any existing role first
	for rn, r := range ac.Roles {
		var filtered []string
		for _, u := range r.Users {
			if u != username {
				filtered = append(filtered, u)
			}
		}
		r.Users = filtered
		ac.Roles[rn] = r
	}

	// Add to new role
	role.Users = append(role.Users, username)
	ac.Roles[roleName] = role

	if err := writeAccessConfig(paths, *ac); err != nil {
		return err
	}

	info(fmt.Sprintf("Granted '%s' → %s", username, roleName))
	return nil
}

func cmdAccessRevoke(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl access revoke <username>")
	}
	paths := resolvePaths()
	username := args[0]

	if !accessExists(paths) {
		return errorf("no access control configured")
	}

	ac := readAccessConfig(paths)
	found := false
	for rn, r := range ac.Roles {
		var filtered []string
		for _, u := range r.Users {
			if u == username {
				found = true
			} else {
				filtered = append(filtered, u)
			}
		}
		r.Users = filtered
		ac.Roles[rn] = r
	}

	if !found {
		return errorf("user '%s' not found in any role", username)
	}

	if err := writeAccessConfig(paths, *ac); err != nil {
		return err
	}

	info(fmt.Sprintf("Revoked access for '%s'", username))
	return nil
}

func cmdAccessAudit(args []string) error {
	paths := resolvePaths()
	logPath := filepath.Join(paths.Root, auditLogFile)

	data, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Println("No audit log found. Enable with: clawctl policy init (sets auditLog: true)")
		return nil
	}

	since := 24 * time.Hour
	for _, a := range args {
		if strings.HasPrefix(a, "--since=") {
			if d, err := time.ParseDuration(a[8:]); err == nil {
				since = d
			}
		}
	}
	cutoff := time.Now().Add(-since)

	bold := "\033[1m"
	nc := "\033[0m"
	fmt.Printf("%sAudit log (last %s)%s\n\n", bold, since, nc)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	shown := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		ts, _ := entry["ts"].(string)
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			if t.Before(cutoff) {
				continue
			}
		}
		user, _ := entry["user"].(string)
		cmd, _ := entry["cmd"].(string)
		result, _ := entry["result"].(string)
		entryArgs, _ := entry["args"].([]any)
		argStrs := make([]string, len(entryArgs))
		for i, a := range entryArgs {
			argStrs[i], _ = a.(string)
		}
		fmt.Printf("  %s  %-10s  %-10s  %-6s  %s\n", ts, user, cmd, result, strings.Join(argStrs, " "))
		shown++
	}

	if shown == 0 {
		fmt.Println("  No entries in time range.")
	}
	return nil
}
