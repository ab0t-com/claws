package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// claws orphans — drift detection between Docker state and claws's
// registry. Surfaces containers that match claws's naming convention but
// have no matching entry in .port-registry (the bob case). Read-only by
// default; `orphans clean` removes (with confirmation).
//
// Scope: today we recognise the OpenClaw runtime project prefix only. Custom
// runtimes with non-default `projectPrefix` are not detected here yet — see
// the worklog note for the follow-up plan.
// ---------------------------------------------------------------------------

// orphanInfo is the per-orphan record produced by discoverOrphans and
// consumed by the renderers. Project is the Docker compose project name
// (e.g., "openclaw-bob") extracted from the container name. Mounts list
// host-side mount sources so the operator can see at a glance whether the
// orphan is pointing at a now-deleted path (the test-harness case).
type orphanInfo struct {
	Container string   `json:"container"`
	Project   string   `json:"project"`
	Status    string   `json:"status"`     // "running", "restarting", "exited", ...
	Created   string   `json:"created"`    // RFC3339 from Docker
	Mounts    []string `json:"mounts"`     // host source paths
	MountsBad []string `json:"mountsBad"`  // subset of Mounts that no longer exist
}

// containerProject returns the Docker compose project name implied by a
// container name. claws-managed containers always look like
// "<project>-<service>-<replica>", e.g.
// "openclaw-team-sarah-openclaw-gateway-1". The service+replica suffix is
// the runtime's gatewayService (or cliService) + "-1". We strip that to
// recover the project. Returns "" if the name doesn't match the convention.
func containerProject(containerName string) string {
	// Try gateway and CLI service suffixes in order of likelihood. The CLI
	// sidecar exists only for runtimes that declare cliService; the gateway
	// is universal.
	for _, suffix := range []string{
		"-openclaw-gateway-1",
		"-openclaw-cli-1",
	} {
		if strings.HasSuffix(containerName, suffix) {
			return strings.TrimSuffix(containerName, suffix)
		}
	}
	return ""
}

// knownProjects returns the set of project names corresponding to instances
// currently in the port registry. Used to determine which containers are
// orphans (i.e., projects with no matching registry entry).
func knownProjects(paths Paths) map[string]bool {
	known := map[string]bool{}
	entries, err := readRegistry(paths)
	if err != nil {
		return known
	}
	for _, e := range entries {
		ref, err := ParseRef(e.Name)
		if err != nil {
			continue
		}
		// We resolve per-instance because a custom runtime can define its
		// own projectPrefix. The default openclaw runtime uses "openclaw-".
		rt := mustResolveRuntime(paths, e.Name)
		known[rt.MakeProjectName(ref)] = true
	}
	return known
}

// discoverOrphans returns containers whose project is not in the known set.
// Implemented over `docker ps -a` so both running and exited orphans are
// surfaced — operators care about restart-looping containers (like bob) and
// stale stopped ones equally.
func discoverOrphans(paths Paths) ([]orphanInfo, error) {
	known := knownProjects(paths)

	// List all containers whose name starts with the openclaw prefix.
	// Filtering at the docker layer keeps the output small and avoids us
	// pulling every container on the host.
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--filter", "name=openclaw-",
		"--format", "{{.Names}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var orphans []orphanInfo
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		project := containerProject(name)
		if project == "" {
			// Doesn't match our naming convention; probably someone else's
			// container that happens to share the prefix. Skip rather than
			// claim ownership.
			continue
		}
		if known[project] {
			continue
		}
		// It's an orphan. Pull additional metadata.
		orphans = append(orphans, inspectOrphan(name, project))
	}
	sort.Slice(orphans, func(i, j int) bool {
		return orphans[i].Container < orphans[j].Container
	})
	return orphans, nil
}

// inspectOrphan fills in status/created/mounts via a single `docker inspect`.
// Failures are tolerated — the orphan record is still returned with whatever
// fields we managed to populate.
func inspectOrphan(name, project string) orphanInfo {
	info := orphanInfo{Container: name, Project: project}

	out, err := exec.Command("docker", "inspect", name).Output()
	if err != nil {
		return info
	}
	var raw []struct {
		Created string `json:"Created"`
		State   struct {
			Status     string `json:"Status"`
			Restarting bool   `json:"Restarting"`
		} `json:"State"`
		Mounts []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(out, &raw); err != nil || len(raw) == 0 {
		return info
	}
	r := raw[0]
	info.Created = r.Created
	info.Status = r.State.Status
	if r.State.Restarting {
		info.Status = "restarting"
	}
	for _, m := range r.Mounts {
		info.Mounts = append(info.Mounts, m.Source)
		if _, statErr := os.Stat(m.Source); statErr != nil {
			info.MountsBad = append(info.MountsBad, m.Source)
		}
	}
	return info
}

// ---------------------------------------------------------------------------
// CLI surface
// ---------------------------------------------------------------------------

func cmdOrphans(args []string) error {
	if len(args) == 0 {
		return cmdOrphansList(args)
	}
	switch args[0] {
	case "list", "ls":
		return cmdOrphansList(args[1:])
	case "clean":
		return cmdOrphansClean(args[1:])
	default:
		// No subcommand given but extra args present — treat as flags to list.
		// e.g. `claws orphans --json` or `claws orphans --reverse`.
		if strings.HasPrefix(args[0], "-") {
			return cmdOrphansList(args)
		}
		return errorf("unknown orphans subcommand: %s (use 'list' or 'clean')", args[0])
	}
}

// reverseOrphan represents a registry entry whose Docker container is
// missing. The inverse of orphanInfo. Distinct type because the fields
// that make sense are different — a reverse orphan has no Container name
// (that's the point) but has Project (computed) and Name (from registry).
type reverseOrphan struct {
	Name        string `json:"name"`        // registry entry name
	Project     string `json:"project"`     // expected Docker project name
	InstanceDir string `json:"instanceDir"` // host path
}

// discoverReverseOrphans walks the registry and returns entries whose
// expected Docker project has no corresponding container. The inverse of
// discoverOrphans (which walks Docker and finds registry-less containers).
func discoverReverseOrphans(paths Paths) ([]reverseOrphan, error) {
	entries, err := readRegistry(paths)
	if err != nil {
		return nil, err
	}
	// Build the set of project names that DO have a container.
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--filter", "name=openclaw-",
		"--format", "{{.Names}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}
	haveContainer := map[string]bool{}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if project := containerProject(strings.TrimSpace(name)); project != "" {
			haveContainer[project] = true
		}
	}

	var reverse []reverseOrphan
	for _, e := range entries {
		ref, err := ParseRef(e.Name)
		if err != nil {
			continue
		}
		rt := mustResolveRuntime(paths, e.Name)
		project := rt.MakeProjectName(ref)
		if haveContainer[project] {
			continue
		}
		reverse = append(reverse, reverseOrphan{
			Name:        e.Name,
			Project:     project,
			InstanceDir: ref.Dir(paths),
		})
	}
	sort.Slice(reverse, func(i, j int) bool { return reverse[i].Name < reverse[j].Name })
	return reverse, nil
}

func cmdOrphansList(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	reverse := hasFlag(args, "--reverse")

	if reverse {
		return cmdOrphansListReverse(paths, jsonMode)
	}

	orphans, err := discoverOrphans(paths)
	if err != nil {
		return err
	}

	if jsonMode {
		data, _ := json.MarshalIndent(orphans, "", "  ")
		// Empty slice should serialize as [] not null.
		if orphans == nil {
			fmt.Println("[]")
			return nil
		}
		fmt.Println(string(data))
		return nil
	}

	if len(orphans) == 0 {
		info("No orphan containers found. Docker state matches the port registry.")
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	fmt.Printf("%sOrphan containers (%d) — running on the host but not in claws's registry%s\n\n", bold, len(orphans), nc)
	for _, o := range orphans {
		fmt.Printf("  %s%s%s\n", bold, o.Container, nc)
		fmt.Printf("    project:  %s\n", o.Project)
		fmt.Printf("    status:   %s\n", colorStatus(o.Status, yellow, red, nc))
		if o.Created != "" {
			fmt.Printf("    created:  %s\n", o.Created)
		}
		if len(o.Mounts) > 0 {
			fmt.Printf("    mounts:\n")
			for _, m := range o.Mounts {
				marker := "  "
				if contains(strings.Join(o.MountsBad, "|"), m) {
					marker = " " + red + "✗" + nc + " "
				}
				fmt.Printf("    %s%s\n", marker, m)
			}
			if len(o.MountsBad) > 0 {
				fmt.Printf("    %s%d mount path(s) no longer exist on the host%s\n", yellow, len(o.MountsBad), nc)
			}
		}
		fmt.Printf("    %sclean:%s    claws orphans clean %s\n", bold, nc, o.Container)
		fmt.Println()
	}
	fmt.Printf("  Or remove all at once: claws orphans clean --all [--yes]\n")
	return nil
}

func cmdOrphansClean(args []string) error {
	paths := resolvePaths()
	autoYes := hasFlag(args, "--yes")
	all := hasFlag(args, "--all")
	target := firstPositional(args)

	if !all && target == "" {
		return errorf("usage: claws orphans clean <container> | claws orphans clean --all [--yes]")
	}
	if all && target != "" {
		return errorf("specify either a container name or --all, not both")
	}

	orphans, err := discoverOrphans(paths)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		info("No orphan containers to clean.")
		return nil
	}

	var toRemove []orphanInfo
	if all {
		toRemove = orphans
	} else {
		for _, o := range orphans {
			if o.Container == target {
				toRemove = append(toRemove, o)
				break
			}
		}
		if len(toRemove) == 0 {
			return errorf("no orphan container named '%s' — run: claws orphans", target)
		}
	}

	if !autoYes {
		warn(fmt.Sprintf("This will `docker rm -f` %d container(s):", len(toRemove)))
		for _, o := range toRemove {
			fmt.Printf("    %s (project: %s, status: %s)\n", o.Container, o.Project, o.Status)
		}
		fmt.Print("  Continue? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			info("Aborted.")
			return nil
		}
	}

	var failed []string
	for _, o := range toRemove {
		if err := exec.Command("docker", "rm", "-f", o.Container).Run(); err != nil {
			warn(fmt.Sprintf("failed to remove %s: %v", o.Container, err))
			failed = append(failed, o.Container)
			continue
		}
		info(fmt.Sprintf("Removed orphan: %s", o.Container))
	}
	if len(failed) > 0 {
		return errorf("%d container(s) failed to remove: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// cmdOrphansListReverse renders the inverse view: registry entries whose
// Docker container is missing. Different presentation from forward
// orphans because the per-finding action is different (the instance dir
// might be intact but the container needs a `claws start`, or the
// instance dir might be gone too — manual investigation).
func cmdOrphansListReverse(paths Paths, jsonMode bool) error {
	reverse, err := discoverReverseOrphans(paths)
	if err != nil {
		return err
	}
	if jsonMode {
		if reverse == nil {
			fmt.Println("[]")
			return nil
		}
		data, _ := json.MarshalIndent(reverse, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if len(reverse) == 0 {
		info("No reverse orphans found. Every registered instance has a corresponding container.")
		return nil
	}
	bold := "\033[1m"
	nc := "\033[0m"
	yellow := "\033[0;33m"
	fmt.Printf("%sReverse orphans (%d) — registered in claws but no Docker container%s\n\n", bold, len(reverse), nc)
	for _, r := range reverse {
		fmt.Printf("  %s%s%s\n", bold, r.Name, nc)
		fmt.Printf("    project:       %s\n", r.Project)
		fmt.Printf("    instance dir:  %s\n", r.InstanceDir)
		// Distinguish "instance dir exists" (likely just needs start) from
		// "instance dir gone" (manual cleanup) so the fix command differs.
		if _, err := os.Stat(r.InstanceDir); err == nil {
			fmt.Printf("    %sfix:%s           claws start %s\n", bold, nc, r.Name)
		} else {
			fmt.Printf("    %sfix:%s           %sinstance dir is missing — investigate, possibly: claws remove %s --purge%s\n",
				bold, nc, yellow, r.Name, nc)
		}
		fmt.Println()
	}
	return nil
}

// colorStatus picks a color for an orphan's container state. Restarting and
// running orphans are yellow (something's actively wasting CPU); exited/dead
// orphans are red (stale state, easy to clean).
func colorStatus(status, yellow, red, nc string) string {
	switch status {
	case "running", "restarting":
		return yellow + status + nc
	case "exited", "dead", "removing":
		return red + status + nc
	default:
		return status
	}
}
