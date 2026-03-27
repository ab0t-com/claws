package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstanceRef is a parsed instance reference: either "name" or "group/name".
type InstanceRef struct {
	Group string // empty for ungrouped instances
	Name  string // the instance name within the group (or standalone)
}

// ParseRef parses "name" or "group/name" into an InstanceRef.
func ParseRef(ref string) (InstanceRef, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		if err := validateName(parts[0]); err != nil {
			return InstanceRef{}, fmt.Errorf("invalid group name: %w", err)
		}
		if err := validateName(parts[1]); err != nil {
			return InstanceRef{}, fmt.Errorf("invalid instance name: %w", err)
		}
		return InstanceRef{Group: parts[0], Name: parts[1]}, nil
	}
	if err := validateName(ref); err != nil {
		return InstanceRef{}, err
	}
	return InstanceRef{Name: ref}, nil
}

// FullName returns "name" or "group/name".
func (r InstanceRef) FullName() string {
	if r.Group != "" {
		return r.Group + "/" + r.Name
	}
	return r.Name
}

// RegistryName returns the name used in the port registry (always "group/name" or "name").
func (r InstanceRef) RegistryName() string {
	return r.FullName()
}

// Dir returns the host directory for this instance.
func (r InstanceRef) Dir(paths Paths) string {
	if r.Group != "" {
		return filepath.Join(paths.Root, r.Group, r.Name)
	}
	return filepath.Join(paths.Root, r.Name)
}

// GroupDir returns the group directory, or empty string for ungrouped.
func (r InstanceRef) GroupDir(paths Paths) string {
	if r.Group == "" {
		return ""
	}
	return filepath.Join(paths.Root, r.Group)
}

// ProjectName returns the docker compose project name.
func (r InstanceRef) ProjectName() string {
	if r.Group != "" {
		return "openclaw-" + r.Group + "-" + r.Name
	}
	return "openclaw-" + r.Name
}

// GroupConfig holds the .group.yml settings for a group.
type GroupConfig struct {
	Name       string   `json:"name"`
	DefaultsApplied bool `json:"defaults_applied,omitempty"` // whether group defaults are used
	Members    []string `json:"members,omitempty"`           // instance names in this group
}

const groupConfigFile = ".group.json"

// ReadGroupConfig loads .group.json from a group directory.
func ReadGroupConfig(groupDir string) (GroupConfig, error) {
	data, err := os.ReadFile(filepath.Join(groupDir, groupConfigFile))
	if err != nil {
		return GroupConfig{}, err
	}
	var cfg GroupConfig
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// WriteGroupConfig writes .group.json to a group directory.
func WriteGroupConfig(groupDir string, cfg GroupConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(groupDir, groupConfigFile), append(data, '\n'), 0644)
}

// IsGroup checks if a directory is a group (has .group.json).
func IsGroup(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, groupConfigFile))
	return err == nil
}

// ListGroups returns all group names under OPENCLAW_ROOT.
func ListGroups(paths Paths) ([]string, error) {
	entries, err := os.ReadDir(paths.Root)
	if err != nil {
		return nil, err
	}
	var groups []string
	for _, e := range entries {
		if e.IsDir() && IsGroup(filepath.Join(paths.Root, e.Name())) {
			groups = append(groups, e.Name())
		}
	}
	return groups, nil
}

// ListGroupMembers returns instance names within a group.
func ListGroupMembers(paths Paths, groupName string) ([]string, error) {
	groupDir := filepath.Join(paths.Root, groupName)
	entries, err := os.ReadDir(groupDir)
	if err != nil {
		return nil, err
	}
	var members []string
	for _, e := range entries {
		if e.IsDir() {
			envFile := filepath.Join(groupDir, e.Name(), "instance.env")
			if _, err := os.Stat(envFile); err == nil {
				members = append(members, e.Name())
			}
		}
	}
	return members, nil
}

// GroupSharedDir returns the shared directory for a group.
func GroupSharedDir(paths Paths, groupName string) string {
	return filepath.Join(paths.Root, groupName, "shared")
}

// ---------------------------------------------------------------------------
// Commands: group create / group list / group add
// ---------------------------------------------------------------------------

func cmdGroup(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl group <create|list|add|remove|shared> [args...]")
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "create":
		return cmdGroupCreate(subargs)
	case "list", "ls":
		return cmdGroupList(subargs)
	case "add":
		return cmdGroupAdd(subargs)
	case "remove", "rm":
		return cmdGroupRemove(subargs)
	case "shared":
		return cmdGroupShared(subargs)
	case "role":
		return cmdGroupRole(subargs)
	default:
		return errorf("unknown group subcommand: %s", subcmd)
	}
}

func cmdGroupCreate(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl group create <name>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := validateName(name); err != nil {
		return err
	}

	groupDir := filepath.Join(paths.Root, name)
	if IsGroup(groupDir) {
		return errorf("group '%s' already exists", name)
	}

	// Don't clobber an existing ungrouped instance
	if _, err := os.Stat(filepath.Join(groupDir, "instance.env")); err == nil {
		return errorf("'%s' is an existing ungrouped instance — cannot create a group with the same name", name)
	}

	if err := os.MkdirAll(filepath.Join(groupDir, "shared", "workspace"), 0755); err != nil {
		return errorf("failed to create shared/workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(groupDir, "shared", "skills"), 0755); err != nil {
		return errorf("failed to create shared/skills: %v", err)
	}

	cfg := GroupConfig{Name: name}
	if err := WriteGroupConfig(groupDir, cfg); err != nil {
		return err
	}

	info(fmt.Sprintf("Group '%s' created.", name))
	fmt.Printf("  Directory: %s\n", groupDir)
	fmt.Printf("  Shared:    %s/shared/\n", groupDir)
	fmt.Println()
	fmt.Printf("  Add instances: clawctl create %s/<name>\n", name)
	return nil
}

func cmdGroupList(args []string) error {
	paths := resolvePaths()
	groups, err := ListGroups(paths)
	if err != nil {
		return err
	}

	if len(groups) == 0 {
		fmt.Println("No groups found.")
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	fmt.Printf("%s%-15s %-10s %s%s\n", bold, "GROUP", "MEMBERS", "DIRECTORY", nc)
	fmt.Printf("%-15s %-10s %s\n", "───────────────", "──────────", "──────────────────────")

	for _, g := range groups {
		members, _ := ListGroupMembers(paths, g)
		dir := filepath.Join(paths.Root, g)
		fmt.Printf("%-15s %-10d %s\n", g, len(members), dir)
	}
	return nil
}

func cmdGroupAdd(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl group add <group> <instance> [instance...]")
	}
	paths := resolvePaths()
	groupName := args[0]
	groupDir := filepath.Join(paths.Root, groupName)

	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist — create it first: clawctl group create %s", groupName, groupName)
	}

	for _, instanceName := range args[1:] {
		srcDir := instanceDir(paths, instanceName)
		if _, err := os.Stat(filepath.Join(srcDir, "instance.env")); err != nil {
			return errorf("instance '%s' not found", instanceName)
		}

		dstDir := filepath.Join(groupDir, instanceName)
		if _, err := os.Stat(dstDir); err == nil {
			warn(fmt.Sprintf("'%s' already in group '%s' — skipping", instanceName, groupName))
			continue
		}

		// Check if instance is running — stop the old compose project first
		oldRef, _ := ParseRef(instanceName)
		wasRunning := false
		cs := containerStatus(paths, instanceName)
		if strings.Contains(cs, "Up") {
			wasRunning = true
			info(fmt.Sprintf("Stopping '%s' (old project: %s)...", instanceName, oldRef.ProjectName()))
			dcRun(paths, instanceName, "down")
		}

		// Move instance into group
		if err := os.Rename(srcDir, dstDir); err != nil {
			return errorf("failed to move '%s' into group: %v", instanceName, err)
		}

		// Update paths in instance.env and fix permissions
		envFile := filepath.Join(dstDir, "instance.env")
		os.Chmod(envFile, credentialFileMode)
		fixCredentialPermissions(dstDir)
		updateEnvValue(envFile, "OPENCLAW_CONFIG_DIR", dstDir)
		updateEnvValue(envFile, "OPENCLAW_WORKSPACE_DIR", filepath.Join(dstDir, "workspace"))

		// Update port registry: old name → new name (under lock)
		newName := groupName + "/" + instanceName
		withRegistryLock(paths, func() error {
			entries, _ := readRegistry(paths)
			for _, e := range entries {
				if e.Name == instanceName {
					unregisterPort(paths, instanceName)
					registerPort(paths, e.Index, newName)
					break
				}
			}
			return nil
		})

		// Build group override (shared mounts)
		newRef := InstanceRef{Group: groupName, Name: instanceName}
		rebuildGroupOverride(paths, newRef)

		// Restart under new project name if it was running
		if wasRunning {
			info(fmt.Sprintf("Starting '%s' (new project: %s)...", newName, newRef.ProjectName()))
			dcRun(paths, newName, "up", "-d", gatewayService(paths, newName))
		}

		info(fmt.Sprintf("Moved '%s' into group '%s'.", instanceName, groupName))
	}
	return nil
}

func cmdGroupRemove(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl group remove <name> [--purge] [--yes]")
	}
	paths := resolvePaths()
	name := args[0]
	purge := hasFlag(args[1:], "--purge")
	confirmed := hasFlag(args[1:], "--yes")

	groupDir := filepath.Join(paths.Root, name)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", name)
	}

	members, _ := ListGroupMembers(paths, name)
	if len(members) > 0 && !purge {
		return errorf("group '%s' has %d instances — use --purge to delete everything, or remove instances first", name, len(members))
	}

	// Confirmation for purge
	if purge && !confirmed {
		warn(fmt.Sprintf("This will permanently delete group '%s' and ALL %d instance(s):", name, len(members)))
		fmt.Printf("  Directory: %s\n", groupDir)
		for _, m := range members {
			fmt.Printf("  Instance:  %s/%s\n", name, m)
		}
		fmt.Print("\n  Continue? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			info("Aborted.")
			return nil
		}
	}

	// Stop and unregister all member instances
	for _, m := range members {
		ref := InstanceRef{Group: name, Name: m}
		dcRun(paths, ref.RegistryName(), "down")
		lockedUnregisterPort(paths, ref.RegistryName())
	}

	if purge {
		os.RemoveAll(groupDir)
		info(fmt.Sprintf("Group '%s' purged (%d instances removed).", name, len(members)))
	} else {
		os.Remove(filepath.Join(groupDir, groupConfigFile))
		info(fmt.Sprintf("Group '%s' removed (directories kept).", name))
	}
	return nil
}

func cmdGroupShared(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl group shared <name> --skills|--workspace|--hooks|--all")
	}
	paths := resolvePaths()
	groupName := args[0]
	groupDir := filepath.Join(paths.Root, groupName)

	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	// Create shared dirs based on flags
	for _, flag := range args[1:] {
		switch flag {
		case "--skills":
			os.MkdirAll(filepath.Join(groupDir, "shared", "skills"), 0755)
			info(fmt.Sprintf("Group shared skills enabled at %s/shared/skills/", groupDir))
		case "--workspace":
			os.MkdirAll(filepath.Join(groupDir, "shared", "workspace"), 0755)
			info(fmt.Sprintf("Group shared workspace enabled at %s/shared/workspace/", groupDir))
		case "--hooks":
			os.MkdirAll(filepath.Join(groupDir, "shared", "hooks"), 0755)
			info(fmt.Sprintf("Group shared hooks enabled at %s/shared/hooks/", groupDir))
		case "--all":
			os.MkdirAll(filepath.Join(groupDir, "shared", "skills"), 0755)
			os.MkdirAll(filepath.Join(groupDir, "shared", "workspace"), 0755)
			os.MkdirAll(filepath.Join(groupDir, "shared", "hooks"), 0755)
			info(fmt.Sprintf("All group shared resources enabled at %s/shared/", groupDir))
		default:
			return errorf("unknown flag: %s", flag)
		}
	}

	// Rebuild overrides for all members to pick up group shares
	members, _ := ListGroupMembers(paths, groupName)
	for _, m := range members {
		ref := InstanceRef{Group: groupName, Name: m}
		rebuildGroupOverride(paths, ref)
	}

	if len(members) > 0 {
		fmt.Printf("  Restart instances to apply: clawctl restart %s/<name>\n", groupName)
	}
	return nil
}

// rebuildGroupOverride generates the compose override for a grouped instance,
// including group-level shared mounts and role-specific mounts.
func rebuildGroupOverride(paths Paths, ref InstanceRef) error {
	rt := mustResolveRuntime(paths, ref.FullName())
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	overridePath := rt.OverridePath(dir)

	// Start with instance-level shared flags
	f := readSharedFlags(envFile)

	// Check group-level shared dirs
	if ref.Group != "" {
		groupShared := GroupSharedDir(paths, ref.Group)
		if _, err := os.Stat(filepath.Join(groupShared, "skills")); err == nil {
			f.Skills = true
		}
		if _, err := os.Stat(filepath.Join(groupShared, "workspace")); err == nil {
			f.Workspace = true
		}
		if _, err := os.Stat(filepath.Join(groupShared, "hooks")); err == nil {
			f.Hooks = true
		}
	}

	// Read role
	role := readEnvValue(envFile, "INSTANCE_ROLE")
	managerName := readEnvValue(envFile, "INSTANCE_MANAGER")

	if !f.Any() && role == "" {
		os.Remove(overridePath)
		return nil
	}

	var gwVolumes, cliVolumes, gwEnv []string

	// Shared resources — use group-level shared dir if grouped, else global
	sharedBase := paths.SharedDir
	if ref.Group != "" {
		sharedBase = GroupSharedDir(paths, ref.Group)
	}

	if f.Skills {
		mount := yamlVolume(6, filepath.Join(sharedBase, "skills"), rt.MountSkills, "ro")
		gwVolumes = append(gwVolumes, mount)
		cliVolumes = append(cliVolumes, mount)
		if rt.SkillsEnvVar != "" {
		gwEnv = append(gwEnv, fmt.Sprintf("      %s: %s", rt.SkillsEnvVar, rt.MountSkills))
		}
	}
	if f.Workspace {
		mount := yamlVolume(6, filepath.Join(sharedBase, "workspace"), rt.MountWorkspace, "rw")
		gwVolumes = append(gwVolumes, mount)
		cliVolumes = append(cliVolumes, mount)
	}
	if f.Hooks {
		mount := yamlVolume(6, filepath.Join(sharedBase, "hooks"), rt.MountHooks, "ro")
		gwVolumes = append(gwVolumes, mount)
		cliVolumes = append(cliVolumes, mount)
	}

	// Manager role: mount task dispatch dir + read-only views of worker workspaces
	if role == "manager" && ref.Group != "" {
		td := filepath.Join(GroupSharedDir(paths, ref.Group), "tasks")
		os.MkdirAll(filepath.Join(td, "pending"), 0755)
		os.MkdirAll(filepath.Join(td, "claimed"), 0755)
		os.MkdirAll(filepath.Join(td, "done"), 0755)

		gwVolumes = append(gwVolumes, yamlVolume(6, td, rt.MountTasks, "rw"))

		// Mount each worker's workspace read-only
		members, _ := ListGroupMembers(paths, ref.Group)
		for _, m := range members {
			if m == ref.Name {
				continue // don't mount own workspace
			}
			memberRole := readEnvValue(filepath.Join(paths.Root, ref.Group, m, "instance.env"), "INSTANCE_ROLE")
			if memberRole == "worker" {
				workerWs := filepath.Join(paths.Root, ref.Group, m, "workspace")
				gwVolumes = append(gwVolumes, yamlVolume(6, workerWs, rt.MountWorkers+"/"+m, "ro"))
			}
		}

		// Output dir for workers to write results
		outputDir := filepath.Join(GroupSharedDir(paths, ref.Group), "output")
		os.MkdirAll(outputDir, 0755)
		gwVolumes = append(gwVolumes, yamlVolume(6, outputDir, rt.MountOutput, "rw"))
	}

	// Worker role: read-only task feed + write to output
	if role == "worker" && ref.Group != "" {
		td := filepath.Join(GroupSharedDir(paths, ref.Group), "tasks")
		gwVolumes = append(gwVolumes, yamlVolume(6, td, rt.MountTasks, "ro"))

		outputDir := filepath.Join(GroupSharedDir(paths, ref.Group), "output")
		os.MkdirAll(outputDir, 0755)
		gwVolumes = append(gwVolumes, yamlVolume(6, outputDir, rt.MountOutput, "rw"))

		// If manager specified, mount manager's workspace read-only
		if managerName != "" {
			managerWs := filepath.Join(paths.Root, ref.Group, managerName, "workspace")
			if _, err := os.Stat(managerWs); err == nil {
				gwVolumes = append(gwVolumes, yamlVolume(6, managerWs, rt.MountManager, "ro"))
			}
		}
	}

	// Write override file
	var b strings.Builder
	b.WriteString("# Auto-generated by clawctl — do not edit manually\n")
	b.WriteString("services:\n")
	b.WriteString(fmt.Sprintf("  %s:\n", rt.GatewayService))
	if len(gwEnv) > 0 {
		b.WriteString("    environment:\n")
		for _, e := range gwEnv {
			b.WriteString(e + "\n")
		}
	}
	if len(gwVolumes) > 0 {
		b.WriteString("    volumes:\n")
		for _, v := range gwVolumes {
			b.WriteString(v + "\n")
		}
	}
	if rt.HasCLI() {
		b.WriteString(fmt.Sprintf("  %s:\n", rt.CLIService))
		if len(cliVolumes) > 0 {
			b.WriteString("    volumes:\n")
			for _, v := range cliVolumes {
				b.WriteString(v + "\n")
			}
		}
	}

	return os.WriteFile(overridePath, []byte(b.String()), 0644)
}

// ---------------------------------------------------------------------------
// clawctl group role — assign/change roles for grouped instances
// ---------------------------------------------------------------------------

func cmdGroupRole(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl group role <group/instance> <manager|worker|none> [--manager=<name>]")
	}

	paths := resolvePaths()
	nameArg := args[0]
	newRole := args[1]

	if newRole != "manager" && newRole != "worker" && newRole != "none" {
		return errorf("invalid role '%s' — use 'manager', 'worker', or 'none'", newRole)
	}

	ref, err := ParseRef(nameArg)
	if err != nil {
		return err
	}
	if ref.Group == "" {
		return errorf("roles require a grouped instance — use: clawctl group role <group>/<instance> <role>")
	}

	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	if _, err := os.Stat(envFile); err != nil {
		return errorf("instance '%s' not found", nameArg)
	}

	var managerName string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--manager=") {
			managerName = a[10:]
		}
	}

	if newRole == "none" {
		// Remove role
		updateEnvValue(envFile, "INSTANCE_ROLE", "")
		updateEnvValue(envFile, "INSTANCE_MANAGER", "")
		// Remove the role line entirely by rewriting without it
		removeEnvKey(envFile, "INSTANCE_ROLE")
		removeEnvKey(envFile, "INSTANCE_MANAGER")
	} else {
		updateEnvValue(envFile, "INSTANCE_ROLE", newRole)
		if managerName != "" {
			updateEnvValue(envFile, "INSTANCE_MANAGER", managerName)
		} else if newRole != "worker" {
			removeEnvKey(envFile, "INSTANCE_MANAGER")
		}
	}

	// Rebuild override with new role mounts
	rebuildGroupOverride(paths, ref)

	// Rebuild manager's override too (so it can see this worker)
	if newRole == "worker" {
		members, _ := ListGroupMembers(paths, ref.Group)
		for _, m := range members {
			mRole := readEnvValue(filepath.Join(paths.Root, ref.Group, m, "instance.env"), "INSTANCE_ROLE")
			if mRole == "manager" {
				mRef := InstanceRef{Group: ref.Group, Name: m}
				rebuildGroupOverride(paths, mRef)
			}
		}
	}

	info(fmt.Sprintf("Role set: %s → %s", nameArg, newRole))
	if newRole != "none" {
		fmt.Printf("  Restart to apply: clawctl restart %s\n", nameArg)
	}
	return nil
}

// removeEnvKey removes a key entirely from an env file.
func removeEnvKey(envFile, key string) {
	data, err := os.ReadFile(envFile)
	if err != nil {
		return
	}
	var lines []string
	for _, line := range splitLines(string(data)) {
		if !strings.HasPrefix(line, key+"=") {
			lines = append(lines, line)
		}
	}
	os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), credentialFileMode)
}
