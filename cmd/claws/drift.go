package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// claws drift — umbrella over orphans (forward) + orphans --reverse +
// filesystem-vs-registry consistency checks.
//
// Four detections, one screen:
//   1. Forward orphans   — Docker containers not in the registry
//   2. Reverse orphans   — registry entries with no container
//   3. Disk drift        — instance dirs on disk not in the registry
//   4. Registry drift    — registry entries pointing at missing instance dirs
//
// Read-only. No `fix` action; each section emits per-finding directive
// commands and the operator decides. Building blocks all exist; this is
// the composition layer.
// ---------------------------------------------------------------------------

type driftReport struct {
	Forward       []orphanInfo     `json:"forward"`       // discoverOrphans
	Reverse       []reverseOrphan  `json:"reverse"`       // discoverReverseOrphans
	DiskDrift     []driftDiskEntry `json:"diskDrift"`     // instance dir, not in registry
	RegistryDrift []driftRegEntry  `json:"registryDrift"` // registry entry, missing dir
}

type driftDiskEntry struct {
	Path         string `json:"path"`
	LastModified string `json:"lastModified,omitempty"`
}

type driftRegEntry struct {
	Name        string `json:"name"`
	InstanceDir string `json:"instanceDir"`
}

func cmdDrift(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")

	rep := gatherDriftReport(paths)

	if jsonMode {
		data, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"

	fmt.Printf("%sclaws drift%s — state consistency check\n\n", bold, nc)

	fmt.Printf("%sForward orphans (containers not in registry) — %d%s\n", bold, len(rep.Forward), nc)
	for _, o := range rep.Forward {
		fmt.Printf("  %s    (status: %s)\n", o.Container, o.Status)
	}
	if len(rep.Forward) == 0 {
		fmt.Printf("  %s✓ none%s\n", green, nc)
	}
	fmt.Println()

	fmt.Printf("%sReverse orphans (registry entries without containers) — %d%s\n", bold, len(rep.Reverse), nc)
	for _, r := range rep.Reverse {
		fmt.Printf("  %s    (expected project: %s)\n", r.Name, r.Project)
	}
	if len(rep.Reverse) == 0 {
		fmt.Printf("  %s✓ none%s\n", green, nc)
	}
	fmt.Println()

	fmt.Printf("%sDisk drift (instance dirs on disk, not in registry) — %d%s\n", bold, len(rep.DiskDrift), nc)
	for _, d := range rep.DiskDrift {
		fmt.Printf("  %s    (last modified: %s)\n", d.Path, d.LastModified)
	}
	if len(rep.DiskDrift) == 0 {
		fmt.Printf("  %s✓ none%s\n", green, nc)
	}
	fmt.Println()

	fmt.Printf("%sRegistry drift (entries pointing at missing instance dirs) — %d%s\n", bold, len(rep.RegistryDrift), nc)
	for _, e := range rep.RegistryDrift {
		fmt.Printf("  %s    (expected dir: %s)\n", e.Name, e.InstanceDir)
	}
	if len(rep.RegistryDrift) == 0 {
		fmt.Printf("  %s✓ none%s\n", green, nc)
	}
	fmt.Println()

	total := len(rep.Forward) + len(rep.Reverse) + len(rep.DiskDrift) + len(rep.RegistryDrift)
	if total == 0 {
		fmt.Printf("%s✓ Mostly clean.%s\n", green, nc)
		return nil
	}

	fmt.Printf("%sFix paths%s\n", bold, nc)
	if len(rep.Forward) > 0 {
		fmt.Println("  claws orphans clean --all --yes")
	}
	for _, r := range rep.Reverse {
		if _, err := os.Stat(r.InstanceDir); err == nil {
			fmt.Printf("  claws start %s\n", r.Name)
		} else {
			fmt.Printf("  %sclaws remove %s --purge --yes%s    # instance dir is missing\n", yellow, r.Name, nc)
		}
	}
	for _, d := range rep.DiskDrift {
		fmt.Printf("  # disk drift: %s exists outside the registry — investigate before removing\n", d.Path)
	}
	for _, e := range rep.RegistryDrift {
		fmt.Printf("  claws remove %s --purge --yes    # registry-only entry, no data on disk\n", e.Name)
	}
	return nil
}

// gatherDriftReport runs the four checks and assembles the report. Each
// check is independent; a failure on one does not poison the others.
func gatherDriftReport(paths Paths) driftReport {
	rep := driftReport{}

	// Forward orphans.
	if o, err := discoverOrphans(paths); err == nil {
		rep.Forward = o
	}
	// Reverse orphans.
	if r, err := discoverReverseOrphans(paths); err == nil {
		rep.Reverse = r
	}

	// Build the set of "known instance directories" (per registry) so we
	// can detect both kinds of filesystem drift.
	entries, _ := readRegistry(paths)
	knownDirs := map[string]string{} // path → instance name
	for _, e := range entries {
		ref, err := ParseRef(e.Name)
		if err != nil {
			continue
		}
		knownDirs[ref.Dir(paths)] = e.Name
	}

	// Registry drift: each known dir whose directory is missing.
	for dir, name := range knownDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			rep.RegistryDrift = append(rep.RegistryDrift, driftRegEntry{
				Name: name, InstanceDir: dir,
			})
		}
	}

	// Disk drift: each non-system directory under OPENCLAW_ROOT that
	// looks like an instance dir (has instance.env) and isn't in
	// knownDirs. Walk top-level + one level deep (for grouped instances).
	rep.DiskDrift = scanDiskForUnknownInstances(paths, knownDirs)

	sort.Slice(rep.Forward, func(i, j int) bool { return rep.Forward[i].Container < rep.Forward[j].Container })
	sort.Slice(rep.Reverse, func(i, j int) bool { return rep.Reverse[i].Name < rep.Reverse[j].Name })
	sort.Slice(rep.DiskDrift, func(i, j int) bool { return rep.DiskDrift[i].Path < rep.DiskDrift[j].Path })
	sort.Slice(rep.RegistryDrift, func(i, j int) bool { return rep.RegistryDrift[i].Name < rep.RegistryDrift[j].Name })
	return rep
}

// scanDiskForUnknownInstances walks OPENCLAW_ROOT and finds directories
// containing instance.env that are not in knownDirs. Walks top-level for
// ungrouped instances, then one level deep through any directory that
// looks like a group (has .group.json), to catch grouped instances.
//
// Skips obvious non-instance dirs: hidden dirs ("." prefix), `shared`,
// `runtimes`, anything outside the documented layout.
func scanDiskForUnknownInstances(paths Paths, knownDirs map[string]string) []driftDiskEntry {
	var out []driftDiskEntry
	root := paths.Root

	check := func(dir string) {
		envFile := filepath.Join(dir, "instance.env")
		if _, err := os.Stat(envFile); err != nil {
			return
		}
		if _, known := knownDirs[dir]; known {
			return
		}
		// Found: dir has instance.env, not in registry.
		info, err := os.Stat(dir)
		lm := ""
		if err == nil {
			lm = info.ModTime().Format("2006-01-02")
		}
		out = append(out, driftDiskEntry{Path: dir, LastModified: lm})
	}

	topLevel, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range topLevel {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "shared" || name == "runtimes" {
			continue
		}
		topDir := filepath.Join(root, name)
		// Could be an ungrouped instance (has instance.env directly).
		check(topDir)
		// Could be a group (has .group.json) — recurse one level for grouped instances.
		if _, err := os.Stat(filepath.Join(topDir, ".group.json")); err == nil {
			inner, err := os.ReadDir(topDir)
			if err != nil {
				continue
			}
			for _, ie := range inner {
				if !ie.IsDir() {
					continue
				}
				if strings.HasPrefix(ie.Name(), ".") || ie.Name() == "shared" {
					continue
				}
				check(filepath.Join(topDir, ie.Name()))
			}
		}
	}
	return out
}
