package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// cmdCron dispatches `claws cron <list|tail> <agent>`.
func cmdCron(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws cron <list|tail> <agent>

  list <agent>   Show jobs (name, schedule, next-run, last-run, last-status)
  tail <agent>   Stream cron/runs/*.jsonl events as they're written

Both read the live runtime state under <instance>/cron/jobs.json and
<instance>/cron/runs/*.jsonl — no runtime probe, no mutation.`)
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) < 2 {
			return errorf("usage: claws cron list <agent>")
		}
		return cronList(args[1])
	case "tail":
		if len(args) < 2 {
			return errorf("usage: claws cron tail <agent>")
		}
		return cronTail(args[1])
	default:
		return errorf("unknown cron subcommand %q (use list, tail)", args[0])
	}
}

func cronList(full string) error {
	paths := resolvePaths()
	jobsPath := filepath.Join(paths.Root, full, "cron", "jobs.json")
	data, err := os.ReadFile(jobsPath)
	if err != nil {
		return errorf("no cron jobs for %s (file: %s)", full, jobsPath)
	}
	var jf CronJobsFile
	if err := json.Unmarshal(data, &jf); err != nil {
		return errorf("malformed jobs.json: %v", err)
	}

	const (
		bold = "\033[1m"
		dim  = "\033[0;90m"
		ok   = "\033[0;32m"
		warn = "\033[0;33m"
		bad  = "\033[0;31m"
		nc   = "\033[0m"
	)
	fmt.Printf("%s%s%s — %d job(s)\n\n", bold, full, nc, len(jf.Jobs))
	fmt.Printf("%s%-26s %-14s %-22s %-22s %s%s\n", bold, "NAME", "SCHEDULE", "NEXT RUN", "LAST RUN", "STATUS", nc)
	for _, j := range jf.Jobs {
		schedDesc := "every " + duration(j.Schedule.EveryMs)
		if j.Schedule.Kind == "cron" {
			schedDesc = j.Schedule.CronExpr
		}
		next, last := "—", "—"
		status := "—"
		if j.State != nil {
			if v, ok := j.State["nextRunAtMs"].(float64); ok && v > 0 {
				next = msToHuman(int64(v))
			}
			if v, ok := j.State["lastRunAtMs"].(float64); ok && v > 0 {
				last = msToHuman(int64(v))
			}
			if v, ok := j.State["lastStatus"].(string); ok {
				status = v
			}
		}
		statusColor := dim
		switch status {
		case "ok":
			statusColor = ok
		case "error", "failed":
			statusColor = bad
		case "pending", "running":
			statusColor = warn
		}
		enabledMark := ""
		if !j.Enabled {
			enabledMark = dim + " [disabled]" + nc
		}
		fmt.Printf("%-26s %-14s %-22s %-22s %s%s%s%s\n", j.Name, schedDesc, next, last, statusColor, status, nc, enabledMark)
	}
	fmt.Println()
	return nil
}

func cronTail(full string) error {
	paths := resolvePaths()
	runsDir := filepath.Join(paths.Root, full, "cron", "runs")
	if _, err := os.Stat(runsDir); err != nil {
		return errorf("no cron runs dir for %s (%s)", full, runsDir)
	}
	// Collect existing run files newest-first, then poll for changes.
	type entry struct {
		path string
		mod  time.Time
	}
	listRuns := func() []entry {
		var out []entry
		_ = filepath.Walk(runsDir, func(p string, info os.FileInfo, _ error) error {
			if info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".jsonl") {
				return nil
			}
			out = append(out, entry{p, info.ModTime()})
			return nil
		})
		sort.Slice(out, func(i, j int) bool { return out[i].mod.Before(out[j].mod) })
		return out
	}

	// Track per-file seek offset.
	pos := map[string]int64{}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	fmt.Printf("# tailing %s (Ctrl-C to exit)\n", runsDir)
	for {
		for _, e := range listRuns() {
			f, err := os.Open(e.path)
			if err != nil {
				continue
			}
			if seek, ok := pos[e.path]; ok {
				_, _ = f.Seek(seek, 0)
			}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 65536), 1024*1024)
			for sc.Scan() {
				fmt.Println(sc.Text())
			}
			cur, _ := f.Seek(0, 1)
			pos[e.path] = cur
			f.Close()
		}
		<-tick.C
	}
}

func duration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7))
	}
}

func msToHuman(ms int64) string {
	if ms == 0 {
		return "—"
	}
	t := time.UnixMilli(ms).UTC()
	delta := time.Since(t)
	switch {
	case delta < 0:
		return "in " + duration(-delta.Milliseconds())
	case delta < 10*time.Second:
		return "just now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	default:
		return t.Format("2026-01-02 15:04Z")
	}
}
