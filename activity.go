package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ActivityEntry struct {
	Time     time.Time
	Instance string
	Type     string // file, error, start, stop
	Detail   string
}

// ---------------------------------------------------------------------------
// clawctl activity — recent actions across instances
// ---------------------------------------------------------------------------

func cmdActivity(args []string) error {
	paths := resolvePaths()

	since := 2 * time.Hour
	var filterGroup string
	limit := 50

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--since="):
			if d, err := time.ParseDuration(a[8:]); err == nil {
				since = d
			}
		case strings.HasPrefix(a, "--group="):
			filterGroup = a[8:]
		case strings.HasPrefix(a, "--limit="):
			fmt.Sscanf(a[8:], "%d", &limit)
		}
	}

	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No instances found.")
		return nil
	}

	cutoff := time.Now().Add(-since)
	var activities []ActivityEntry

	for _, e := range entries {
		if filterGroup != "" {
			ref, _ := ParseRef(e.Name)
			if ref.Group != filterGroup {
				continue
			}
		}

		ref, _ := ParseRef(e.Name)
		dir := ref.Dir(paths)

		// Recent workspace file changes
		wsDir := filepath.Join(dir, "workspace")
		if fi, err := os.Stat(wsDir); err == nil && fi.IsDir() {
			files := recentFiles(wsDir, cutoff, 20)
			for _, f := range files {
				relPath, _ := filepath.Rel(wsDir, f.path)
				activities = append(activities, ActivityEntry{
					Time:     f.modTime,
					Instance: e.Name,
					Type:     "file",
					Detail:   relPath,
				})
			}
		}

		// Recent log errors
		logErrors := recentLogErrors(paths, e.Name, cutoff, 10)
		for _, le := range logErrors {
			activities = append(activities, ActivityEntry{
				Time:     le.Time,
				Instance: e.Name,
				Type:     "error",
				Detail:   le.Detail,
			})
		}
	}

	// Sort by time, newest first
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Time.After(activities[j].Time)
	})

	if len(activities) > limit {
		activities = activities[:limit]
	}

	if len(activities) == 0 {
		fmt.Printf("No activity in the last %s.\n", since)
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	cyan := "\033[0;36m"
	red := "\033[0;31m"

	fmt.Printf("%sRecent activity (last %s)%s\n\n", bold, since, nc)
	fmt.Printf("%s%-20s %-18s %-8s %s%s\n", bold, "TIME", "INSTANCE", "TYPE", "DETAIL", nc)
	fmt.Printf("%-20s %-18s %-8s %s\n", "────────────────────", "──────────────────", "────────", "──────────────────────────────")

	for _, a := range activities {
		timeStr := a.Time.Format("15:04:05")
		color := cyan
		if a.Type == "error" {
			color = red
		}
		fmt.Printf("%-20s %-18s %s%-8s%s %s\n", timeStr, a.Instance, color, a.Type, nc, a.Detail)
	}
	return nil
}

type fileInfo struct {
	path    string
	modTime time.Time
}

func recentFiles(dir string, cutoff time.Time, maxFiles int) []fileInfo {
	var files []fileInfo
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and large binary dirs
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == ".tools" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.ModTime().After(cutoff) {
			files = append(files, fileInfo{path: path, modTime: info.ModTime()})
		}
		return nil
	})

	// Sort newest first, limit
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	return files
}

type logEntry struct {
	Time   time.Time
	Detail string
}

func recentLogErrors(paths Paths, name string, cutoff time.Time, maxEntries int) []logEntry {
	// Grep container logs for errors (with timestamps)
	cmd := exec.Command("docker", "compose",
		"-f", paths.ComposeTemplate,
		"--env-file", filepath.Join(instanceDirFromName(paths, name), "instance.env"),
		"-p", projectNameFromName(name),
		"logs", "--tail", "200", "--no-color", "--timestamps", "openclaw-gateway",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var entries []logEntry
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "crash") {
			ts := parseDockerLogTimestamp(line)
			if !ts.IsZero() && ts.Before(cutoff) {
				continue // skip entries older than cutoff
			}
			entries = append(entries, logEntry{
				Time:   ts,
				Detail: truncate(stripLogPrefix(line), 80),
			})
		}
	}
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	return entries
}

// parseDockerLogTimestamp extracts a timestamp from a Docker compose log line.
// Docker compose logs with --timestamps format:
//   <container> | 2026-03-17T10:30:45.123456789Z <message>
//   or just: 2026-03-17T10:30:45.123456789Z <message>
func parseDockerLogTimestamp(line string) time.Time {
	// Try to find an RFC3339 timestamp in the line
	// Look for pattern: YYYY-MM-DDTHH:MM:SS
	for i := 0; i <= len(line)-20; i++ {
		if line[i] >= '2' && line[i] <= '2' && i+4 < len(line) && line[i+4] == '-' && line[i+7] == '-' && line[i+10] == 'T' {
			// Found potential timestamp start, try to parse
			end := i + 20
			// Extend to include fractional seconds and timezone
			for end < len(line) && line[end] != ' ' && line[end] != '\t' {
				end++
			}
			candidate := line[i:end]
			if t, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
				return t
			}
			if t, err := time.Parse(time.RFC3339, candidate); err == nil {
				return t
			}
		}
	}
	return time.Now() // fallback
}

// stripLogPrefix removes the Docker compose container prefix ("container | ") from a log line.
func stripLogPrefix(line string) string {
	if idx := strings.Index(line, " | "); idx >= 0 {
		return strings.TrimSpace(line[idx+3:])
	}
	return strings.TrimSpace(line)
}

func instanceDirFromName(paths Paths, name string) string {
	ref, _ := ParseRef(name)
	return ref.Dir(paths)
}

func projectNameFromName(name string) string {
	ref, _ := ParseRef(name)
	return ref.ProjectName()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
