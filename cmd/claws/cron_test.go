package main

import (
	"strings"
	"testing"
	"time"
)

func TestScheduleToRuntime_EveryDuration(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s, err := scheduleToRuntime("every 30m", now)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.Kind != "every" || s.EveryMs != int64(30*time.Minute/time.Millisecond) {
		t.Errorf("wrong schedule: %+v", s)
	}
	if s.AnchorMs != now.UnixMilli() {
		t.Errorf("anchor mismatch: %d vs %d", s.AnchorMs, now.UnixMilli())
	}
}

func TestScheduleToRuntime_AtAliases(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cases := map[string]int64{
		"@hourly":   int64(time.Hour / time.Millisecond),
		"@daily":    int64(24 * time.Hour / time.Millisecond),
		"@weekly":   int64(7 * 24 * time.Hour / time.Millisecond),
		"@monthly":  int64(30 * 24 * time.Hour / time.Millisecond),
		"@yearly":   int64(365 * 24 * time.Hour / time.Millisecond),
		"@annually": int64(365 * 24 * time.Hour / time.Millisecond),
	}
	for in, wantMs := range cases {
		s, err := scheduleToRuntime(in, now)
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if s.Kind != "every" || s.EveryMs != wantMs {
			t.Errorf("%s: got %+v want everyMs=%d", in, s, wantMs)
		}
	}
}

func TestScheduleToRuntime_Crontab(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s, err := scheduleToRuntime("0 9 * * 1", now)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.Kind != "cron" || s.CronExpr != "0 9 * * 1" {
		t.Errorf("crontab passthrough wrong: %+v", s)
	}
}

func TestScheduleToRuntime_Bad(t *testing.T) {
	now := time.Now()
	for _, bad := range []string{"", "@weird", "every", "every banana", "0 9 * *"} {
		if _, err := scheduleToRuntime(bad, now); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestPayloadFromAction(t *testing.T) {
	tests := []struct {
		job   ProfileCronJob
		want  string
	}{
		{ProfileCronJob{Prompt: "Send a report"}, "Send a report"},
		{ProfileCronJob{Command: "echo hi"}, "Execute shell command: echo hi"},
		{ProfileCronJob{Hook: "onIdle"}, "Trigger lifecycle hook: onIdle"},
		{ProfileCronJob{Exec: []string{"echo", "foo"}}, "Execute: echo foo"},
		{ProfileCronJob{}, "(no action specified)"},
	}
	for _, tt := range tests {
		got := payloadFromAction(tt.job).Text
		if got != tt.want {
			t.Errorf("payload for %+v: got %q want %q", tt.job, got, tt.want)
		}
	}
}

func TestJobIDFor_StableAndUUIDShaped(t *testing.T) {
	a := jobIDFor("team/agent", "job-1")
	b := jobIDFor("team/agent", "job-1")
	if a != b {
		t.Errorf("jobIDFor not stable: %s vs %s", a, b)
	}
	// Different inputs → different IDs.
	c := jobIDFor("team/agent", "job-2")
	if a == c {
		t.Errorf("jobIDFor collision across names")
	}
	// UUID v4 shape: 8-4-4-4-12, version=4 at position [14], variant in {8,9,a,b} at [19].
	if len(a) != 36 || strings.Count(a, "-") != 4 {
		t.Errorf("not UUID-shaped: %s", a)
	}
	if a[14] != '4' {
		t.Errorf("UUID version nibble != 4: %s", a)
	}
	switch a[19] {
	case '8', '9', 'a', 'b':
		// ok
	default:
		t.Errorf("UUID variant nibble unexpected: %s", a)
	}
}

func TestRandomUUIDv4_ShapeAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		u := randomUUIDv4()
		if len(u) != 36 || strings.Count(u, "-") != 4 {
			t.Fatalf("bad shape: %s", u)
		}
		if u[14] != '4' {
			t.Fatalf("version nibble != 4: %s", u)
		}
		if seen[u] {
			t.Fatalf("duplicate UUID after %d iterations: %s", i, u)
		}
		seen[u] = true
	}
}
