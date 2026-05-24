package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Cron jobs.json — the runtime's actual cron contract.
//
// Reverse-engineered from ~/.openclaw/team/sarah/cron/jobs.json.bak:
//   <instance>/cron/jobs.json holds { version, jobs: [...] }.
//   Each job is a "send this prompt to the agent on a schedule" entry —
//   NOT a shell command. The runtime image walks the jobs on its own
//   cron cadence and dispatches the payload as a systemEvent to the agent.
//
// claws v1.6 owns the EDITABLE shape (the input to the runtime). The
// runtime fills in state.* fields at runtime — we never write those.
// ---------------------------------------------------------------------------

// CronJobsFile is the top-level shape claws writes to <instance>/cron/jobs.json.
type CronJobsFile struct {
	Version int       `json:"version"`
	Jobs    []CronJob `json:"jobs"`
}

// CronJob mirrors the runtime's editable shape. We only touch the fields
// claws is allowed to set — state.* is runtime-owned and we never overwrite.
type CronJob struct {
	ID            string         `json:"id"`
	AgentID       string         `json:"agentId"`
	SessionKey    string         `json:"sessionKey,omitempty"`
	Name          string         `json:"name"`
	Enabled       bool           `json:"enabled"`
	CreatedAtMs   int64          `json:"createdAtMs"`
	UpdatedAtMs   int64          `json:"updatedAtMs"`
	Schedule      CronSchedule   `json:"schedule"`
	SessionTarget string         `json:"sessionTarget,omitempty"`
	WakeMode      string         `json:"wakeMode,omitempty"`
	Payload       CronPayload    `json:"payload"`
	State         map[string]any `json:"state,omitempty"` // runtime-owned; preserved on rewrite
}

type CronSchedule struct {
	Kind     string `json:"kind"`               // "every" | "cron"
	EveryMs  int64  `json:"everyMs,omitempty"`  // for kind=every
	CronExpr string `json:"cronExpr,omitempty"` // for kind=cron (runtime may or may not honour)
	AnchorMs int64  `json:"anchorMs,omitempty"` // reference time for first fire
}

type CronPayload struct {
	Kind string `json:"kind"`           // "systemEvent" is the only known kind
	Text string `json:"text,omitempty"` // the prompt sent to the agent
}

// ---------------------------------------------------------------------------
// Translation: ProfileCronJob (claws schema) → CronJob (runtime shape).
// ---------------------------------------------------------------------------

// jobIDFor returns a stable deterministic-ish UUID-v4-style ID for a
// (agent, jobName) pair. Deterministic means re-applying produces the
// same ID so existing state (lastRunAtMs, etc.) isn't lost.
//
// We derive: first 16 hex chars of sha256(agent+"/"+jobName), then
// formatted as UUID-v4 with version + variant bits stamped.
func jobIDFor(agentFull, jobName string) string {
	// Use a hex of agent+name; we don't need crypto strength here — uniqueness.
	const prefix = "claws-job:"
	raw := prefix + agentFull + "/" + jobName
	h := hashHex([]byte(raw)) // from fetch.go (sha256 hex)
	// Format as 8-4-4-4-12 UUID — set version=4 and variant=10 bits.
	if len(h) < 32 {
		return h
	}
	hex32 := h[:32]
	// Stamp v4 nibble at position 12, variant nibble at position 16.
	b := []byte(hex32)
	b[12] = '4'
	// variant: 10xx → 8,9,a,b
	switch b[16] {
	case '0', '1', '2', '3':
		b[16] = '8'
	case '4', '5', '6', '7':
		b[16] = '9'
	case 'c', 'd', 'e', 'f':
		b[16] = 'b'
	default:
		b[16] = 'a'
	}
	return string(b[0:8]) + "-" + string(b[8:12]) + "-" + string(b[12:16]) + "-" + string(b[16:20]) + "-" + string(b[20:32])
}

// scheduleToRuntime converts a claws schedule string into the runtime's
// schedule shape. Returns the schedule and the next-anchor ms timestamp.
func scheduleToRuntime(s string, now time.Time) (CronSchedule, error) {
	s = strings.TrimSpace(s)
	nowMs := now.UnixMilli()

	if strings.HasPrefix(s, "@") {
		var everyMs int64
		switch s {
		case "@hourly":
			everyMs = int64(time.Hour / time.Millisecond)
		case "@daily":
			everyMs = int64(24 * time.Hour / time.Millisecond)
		case "@weekly":
			everyMs = int64(7 * 24 * time.Hour / time.Millisecond)
		case "@monthly":
			everyMs = int64(30 * 24 * time.Hour / time.Millisecond) // approximation
		case "@yearly", "@annually":
			everyMs = int64(365 * 24 * time.Hour / time.Millisecond)
		case "@reboot":
			// One-shot at startup — represent as a very large interval with
			// anchor = now, runtime decides whether to honour. Pragmatic fallback.
			everyMs = int64(365 * 100 * 24 * time.Hour / time.Millisecond)
		default:
			return CronSchedule{}, errorf("unknown @-alias %q", s)
		}
		return CronSchedule{Kind: "every", EveryMs: everyMs, AnchorMs: nowMs}, nil
	}

	if strings.HasPrefix(s, "every ") {
		dur := strings.TrimSpace(strings.TrimPrefix(s, "every "))
		d, err := time.ParseDuration(dur)
		if err != nil {
			return CronSchedule{}, errorf("invalid duration after 'every': %v", err)
		}
		return CronSchedule{Kind: "every", EveryMs: d.Milliseconds(), AnchorMs: nowMs}, nil
	}

	// Plain crontab — pass through; the runtime decides whether to honour.
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return CronSchedule{}, errorf("crontab must have 5 fields (got %d): %q", len(fields), s)
	}
	return CronSchedule{Kind: "cron", CronExpr: s, AnchorMs: nowMs}, nil
}

// payloadFromAction builds the runtime payload from a claws cron action
// (one of: prompt, command, hook, exec). The runtime's only known payload
// kind is "systemEvent" with a text prompt — so non-prompt actions get
// wrapped as honest "execute this on behalf of the agent" prompts.
func payloadFromAction(cj ProfileCronJob) CronPayload {
	switch {
	case cj.Prompt != "":
		return CronPayload{Kind: "systemEvent", Text: cj.Prompt}
	case cj.Command != "":
		return CronPayload{Kind: "systemEvent", Text: "Execute shell command: " + cj.Command}
	case cj.Hook != "":
		return CronPayload{Kind: "systemEvent", Text: "Trigger lifecycle hook: " + cj.Hook}
	case len(cj.Exec) > 0:
		return CronPayload{Kind: "systemEvent", Text: "Execute: " + strings.Join(cj.Exec, " ")}
	}
	return CronPayload{Kind: "systemEvent", Text: "(no action specified)"}
}

// ---------------------------------------------------------------------------
// applyCronJobsJSON — v1.6 path. Writes <instance>/cron/jobs.json in the
// runtime's editable shape. Preserves existing state.* for jobs that already
// exist (matched by stable ID).
// ---------------------------------------------------------------------------
func applyCronJobsJSON(paths Paths, full string, ag ProfileAgent) error {
	cronDir := filepath.Join(paths.Root, full, "cron")
	if err := os.MkdirAll(cronDir, 0755); err != nil {
		return err
	}
	jobsPath := filepath.Join(cronDir, "jobs.json")

	// Read existing to preserve state.* — we never overwrite runtime-owned fields.
	existing := map[string]CronJob{}
	if data, err := os.ReadFile(jobsPath); err == nil {
		var cur CronJobsFile
		if err := json.Unmarshal(data, &cur); err == nil {
			for _, j := range cur.Jobs {
				existing[j.ID] = j
			}
		}
	}

	now := time.Now().UTC()
	nowMs := now.UnixMilli()
	_, agentName := splitFull(full)

	out := CronJobsFile{Version: 1}
	for _, cj := range ag.Cron {
		schedule, err := scheduleToRuntime(cj.Schedule, now)
		if err != nil {
			return errorf("cron %q: %v", cj.Name, err)
		}
		id := jobIDFor(full, cj.Name)
		enabled := true
		if cj.Enabled != nil {
			enabled = *cj.Enabled
		}
		job := CronJob{
			ID:            id,
			AgentID:       agentName,
			SessionKey:    "agent:" + agentName + ":claws-template",
			Name:          cj.Name,
			Enabled:       enabled,
			CreatedAtMs:   nowMs,
			UpdatedAtMs:   nowMs,
			Schedule:      schedule,
			SessionTarget: "main",
			WakeMode:      "now",
			Payload:       payloadFromAction(cj),
		}
		// Preserve runtime-owned state + createdAtMs on existing jobs.
		if prev, ok := existing[id]; ok {
			job.CreatedAtMs = prev.CreatedAtMs
			job.State = prev.State
			// If the user disabled it via the template, we update Enabled;
			// otherwise preserve runtime's view.
			if cj.Enabled == nil {
				job.Enabled = prev.Enabled
			}
		}
		out.Jobs = append(out.Jobs, job)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	// Idempotence: skip write if content matches.
	if cur, err := os.ReadFile(jobsPath); err == nil && string(cur) == string(data) {
		return nil
	}
	return os.WriteFile(jobsPath, data, 0644)
}

// splitFull breaks "team/agent" into its parts.
func splitFull(full string) (team, name string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) != 2 {
		return "", full
	}
	return parts[0], parts[1]
}

// randomUUIDv4 returns a fresh UUID v4 string. Used by P2.1.
func randomUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: time-based hex (still unique within a process; not collision-proof)
		s := strconv.FormatInt(time.Now().UnixNano(), 16)
		for len(s) < 32 {
			s = "0" + s
		}
		copy(b[:], []byte(s[:16]))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
