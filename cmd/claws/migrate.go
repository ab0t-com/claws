package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const migrateHelp = `Usage: claws migrate <subcommand>

One-time data migrations for upgrades. Each subcommand is idempotent —
safe to re-run.

Subcommands:
  cron      v1.5 → v1.6 — convert <instance>/workspace/cron/claws.crontab
            (legacy crontab format) to <instance>/cron/jobs.json (runtime
            jobs format). Best-effort: only converts entries the new
            schema can express; leaves the legacy file in place so the
            operator can verify and remove it.
  uuids     Populate CLAWS_INSTANCE_UUID in every agent's instance.env that
            doesn't have one. Idempotent.
  all       Run every migration in order.
`

// cmdMigrateData dispatches the v1.6+ data migrations (cron, uuids).
// The legacy storage migration (`claws migrate <instance> --to s3`) lives
// in storage.go as cmdMigrate; the unified `migrate` verb in main.go
// routes based on the first argument.
func cmdMigrateData(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(migrateHelp)
		return nil
	}
	paths := resolvePaths()

	switch args[0] {
	case "cron":
		return migrateCron(paths)
	case "uuids":
		return migrateUUIDs(paths)
	case "all":
		if err := migrateCron(paths); err != nil {
			return err
		}
		return migrateUUIDs(paths)
	default:
		return errorf("unknown migrate subcommand %q (use cron, uuids, all)", args[0])
	}
}

// isDataMigration returns true when the first arg names one of the v1.6+
// data migrations rather than an instance name for the storage migration.
func isDataMigration(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "cron", "uuids", "all", "-h", "--help":
		return true
	}
	return false
}

// migrateCron walks every registered agent, looks for a legacy
// workspace/cron/claws.crontab, and converts it to the runtime's
// cron/jobs.json shape. Pre-existing jobs.json entries are preserved.
//
// The legacy file is NOT deleted — operator removes after verifying.
func migrateCron(paths Paths) error {
	entries, err := readRegistry(paths)
	if err != nil {
		return errorf("read registry: %v", err)
	}
	converted, skipped := 0, 0
	for _, e := range entries {
		legacy := filepath.Join(paths.Root, e.Name, "workspace", "cron", "claws.crontab")
		data, err := os.ReadFile(legacy)
		if err != nil {
			skipped++
			continue
		}
		jobs := parseLegacyCrontab(string(data))
		if len(jobs) == 0 {
			fmt.Printf("  %s — empty/no parseable jobs in legacy file, skipping\n", e.Name)
			skipped++
			continue
		}
		// Build a synthetic ProfileAgent so we can reuse applyCronJobsJSON.
		ag := ProfileAgent{Name: e.Name, Cron: jobs}
		if err := applyCronJobsJSON(paths, e.Name, ag); err != nil {
			fmt.Printf("  %s — convert failed: %v\n", e.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s — converted %d job(s); legacy file preserved at %s\n",
			e.Name, len(jobs), legacy)
		converted++
	}
	fmt.Printf("\nDone. %d converted, %d skipped (no legacy file or empty).\n", converted, skipped)
	return nil
}

// parseLegacyCrontab parses a v1.5-style claws.crontab into ProfileCronJob
// entries. Best-effort: lines like "@daily echo foo" or "every 30m cmd"
// are recognised; comments and DISABLED markers preserved.
func parseLegacyCrontab(body string) []ProfileCronJob {
	var out []ProfileCronJob
	disabled := map[string]bool{}
	nextJob := ""
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# DISABLED:") {
			disabled[strings.TrimSpace(strings.TrimPrefix(line, "# DISABLED:"))] = true
			continue
		}
		if strings.HasPrefix(line, "# job:") {
			nextJob = strings.TrimSpace(strings.TrimPrefix(line, "# job:"))
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Schedule + command. Determine where the schedule ends.
		var schedule, command string
		switch {
		case strings.HasPrefix(line, "@"):
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				schedule, command = parts[0], parts[1]
			}
		case strings.HasPrefix(line, "every "):
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 {
				schedule = parts[0] + " " + parts[1]
				command = parts[2]
			}
		default:
			parts := strings.Fields(line)
			if len(parts) >= 6 {
				schedule = strings.Join(parts[:5], " ")
				command = strings.Join(parts[5:], " ")
			}
		}
		if schedule == "" {
			continue
		}
		name := nextJob
		if name == "" {
			name = fmt.Sprintf("legacy-%d", len(out)+1)
		}
		enabled := !disabled[name]
		out = append(out, ProfileCronJob{
			Name:     name,
			Schedule: schedule,
			Command:  command,
			Enabled:  &enabled,
		})
		nextJob = ""
	}
	return out
}

// migrateUUIDs walks every agent and ensures CLAWS_INSTANCE_UUID is set in
// instance.env. Idempotent: agents that already have one are skipped.
func migrateUUIDs(paths Paths) error {
	entries, err := readRegistry(paths)
	if err != nil {
		return errorf("read registry: %v", err)
	}
	added, skipped := 0, 0
	for _, e := range entries {
		envPath := filepath.Join(paths.Root, e.Name, "instance.env")
		data, err := os.ReadFile(envPath)
		if err != nil {
			fmt.Printf("  %s — instance.env not readable: %v\n", e.Name, err)
			continue
		}
		body := string(data)
		if strings.Contains(body, "CLAWS_INSTANCE_UUID=") {
			skipped++
			continue
		}
		uuid := randomUUIDv4()
		body = strings.TrimRight(body, "\n") + "\nCLAWS_INSTANCE_UUID=" + uuid + "\n"
		if err := os.WriteFile(envPath, []byte(body), 0600); err != nil {
			fmt.Printf("  %s — write failed: %v\n", e.Name, err)
			continue
		}
		// Also mirror into openclaw.json meta.id if file exists.
		cfgPath := filepath.Join(paths.Root, e.Name, "openclaw.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg map[string]interface{}
			if err := json.Unmarshal(data, &cfg); err == nil {
				meta, _ := cfg["meta"].(map[string]interface{})
				if meta == nil {
					meta = map[string]interface{}{}
					cfg["meta"] = meta
				}
				if _, ok := meta["id"]; !ok {
					meta["id"] = uuid
					if updated, err := json.MarshalIndent(cfg, "", "  "); err == nil {
						_ = os.WriteFile(cfgPath, append(updated, '\n'), 0644)
					}
				}
			}
		}
		fmt.Printf("  ✓ %s — uuid: %s\n", e.Name, uuid)
		added++
	}
	fmt.Printf("\nDone. %d added, %d already had UUID.\n", added, skipped)
	return nil
}
