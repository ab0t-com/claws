package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- cron --------------------------------------------------------------

func TestValidateCronSchedule(t *testing.T) {
	ok := []string{
		"0 9 * * 1",
		"*/15 * * * *",
		"@hourly", "@daily", "@weekly", "@monthly", "@yearly", "@annually", "@reboot",
		"every 30m",
		"every 1h",
		"every 90s",
	}
	for _, s := range ok {
		if err := validateCronSchedule(s); err != nil {
			t.Errorf("validateCronSchedule(%q): unexpected error: %v", s, err)
		}
	}
	bad := []string{
		"",
		"   ",
		"@frequently",
		"every always",
		"every 30",          // duration missing unit
		"0 9 * *",           // 4 fields
		"0 9 * * 1 extra",   // 6 fields
	}
	for _, s := range bad {
		if err := validateCronSchedule(s); err == nil {
			t.Errorf("validateCronSchedule(%q): expected error", s)
		}
	}
}

// v1.6 — apply with cron writes <instance>/cron/jobs.json in the runtime's
// JSON shape (NOT workspace/cron/claws.crontab as v1.5 did).
func TestIntegration_ApplyCron(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "cron.json")
	body := `{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "ct", "version": "0.1"},
  "team": {"name": "ct"},
  "agents": [{
    "name": "a",
    "hooks": {"onIdle": "echo idle"},
    "cron": [
      {"name": "daily",  "schedule": "@daily",       "prompt":  "Daily summary please"},
      {"name": "beat",   "schedule": "every 30m",    "hook":    "onIdle"},
      {"name": "shell",  "schedule": "@hourly",      "command": "echo do-it"},
      {"name": "off",    "schedule": "@hourly",      "command": "echo off", "enabled": false}
    ]
  }]
}`
	_ = os.WriteFile(profile, []byte(body), 0644)
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	jobsData, err := os.ReadFile(filepath.Join(root, "ct", "a", "cron", "jobs.json"))
	if err != nil {
		t.Fatalf("v1.6 jobs.json not written at <instance>/cron/jobs.json: %v", err)
	}
	var jf CronJobsFile
	if err := json.Unmarshal(jobsData, &jf); err != nil {
		t.Fatalf("jobs.json malformed: %v\n%s", err, jobsData)
	}
	if jf.Version != 1 {
		t.Errorf("expected version=1, got %d", jf.Version)
	}
	if len(jf.Jobs) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(jf.Jobs))
	}
	byName := map[string]CronJob{}
	for _, j := range jf.Jobs {
		byName[j.Name] = j
	}
	if j := byName["daily"]; j.Schedule.Kind != "every" || j.Schedule.EveryMs != 86400000 {
		t.Errorf("daily schedule wrong: %+v", j.Schedule)
	}
	if j := byName["daily"]; j.Payload.Text != "Daily summary please" {
		t.Errorf("daily prompt wrong: %q", j.Payload.Text)
	}
	if j := byName["beat"]; j.Schedule.EveryMs != 1800000 {
		t.Errorf("beat schedule wrong: %+v", j.Schedule)
	}
	if j := byName["beat"]; !strings.Contains(j.Payload.Text, "lifecycle hook: onIdle") {
		t.Errorf("hook reference not in payload: %q", j.Payload.Text)
	}
	if j := byName["shell"]; !strings.Contains(j.Payload.Text, "Execute shell command") {
		t.Errorf("shell command not wrapped as systemEvent: %q", j.Payload.Text)
	}
	if j := byName["off"]; j.Enabled {
		t.Errorf("disabled job should have enabled=false")
	}
	_ = out
}

// Bad cron schedule is rejected at apply.
func TestIntegration_ApplyCronRejectsBad(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "bad.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "x", "version": "0"},
  "team": {"name": "x"},
  "agents": [{
    "name": "a",
    "cron": [{"name": "bad", "schedule": "not a schedule", "command": "echo"}]
  }]
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile, "--skip-audit"); err == nil {
		t.Error("expected error for bad cron schedule")
	}
}

// Cron with both command and exec is rejected.
func TestIntegration_ApplyCronRejectsMultipleActions(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "bad.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "x", "version": "0"},
  "team": {"name": "x"},
  "agents": [{
    "name": "a",
    "cron": [{"name": "x", "schedule": "@daily", "command": "echo", "exec": ["echo"]}]
  }]
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile, "--skip-audit"); err == nil {
		t.Error("expected error for ambiguous cron action")
	}
}

// --- events ------------------------------------------------------------

func TestIntegration_ApplyEvents(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "ev.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "ev", "version": "0"},
  "team": {"name": "ev"},
  "agents": [{
    "name": "a",
    "events": {"enabled": true, "digestMode": true, "endpoint": "/events/a", "allowFromIps": ["10.0.0.0/8"]}
  }]
}`), 0644)
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	cfgData, err := os.ReadFile(filepath.Join(root, "ev", "a", "openclaw.json"))
	if err != nil {
		t.Fatalf("openclaw.json missing: %v", err)
	}
	cfg := string(cfgData)
	for _, want := range []string{`"enabled": true`, `"digestMode": true`, `"endpoint": "/events/a"`, `"10.0.0.0/8"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("events config missing %q in:\n%s", want, cfg)
		}
	}
}

// --- sidecars ----------------------------------------------------------

func TestIntegration_ApplySidecarSharedwatch(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "sc.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "sc", "version": "0"},
  "team": {"name": "sc"},
  "agents": [{
    "name": "a",
    "sidecars": [
      {"name": "watcher", "kind": "sharedwatch", "config": {"watchDir": "/w", "actor": "a"}}
    ]
  }]
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile, "--skip-audit"); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "sc", "a", "workspace", "sidecars", "watcher.json"))
	if err != nil {
		t.Fatalf("sidecar declaration missing: %v", err)
	}
	var sc map[string]interface{}
	_ = json.Unmarshal(data, &sc)
	if sc["kind"] != "sharedwatch" {
		t.Errorf("expected kind=sharedwatch, got %v", sc["kind"])
	}
	cfg, _ := sc["config"].(map[string]interface{})
	if cfg["watchDir"] != "/w" {
		t.Errorf("expected watchDir=/w, got %v", cfg["watchDir"])
	}
}

func TestIntegration_ApplySidecarUnknownKind(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "sc.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "sc", "version": "0"},
  "team": {"name": "sc"},
  "agents": [{
    "name": "a",
    "sidecars": [{"name": "x", "kind": "bogus-helper"}]
  }]
}`), 0644)
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	// apply doesn't fail (sidecar is best-effort), but the line should warn
	if err != nil {
		t.Logf("apply did fail (acceptable): %v", err)
	}
	if !strings.Contains(out, "unknown sidecar kind") && !strings.Contains(out, "bogus-helper") {
		t.Errorf("expected warning about unknown sidecar kind, got:\n%s", out)
	}
}

// --- topology ----------------------------------------------------------

func TestValidateTopology_RejectsCycles(t *testing.T) {
	agents := []ProfileAgent{
		{Name: "a", Manager: "b"},
		{Name: "b", Manager: "a"},
	}
	if err := validateTopology(agents); err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestValidateTopology_RejectsSelfManager(t *testing.T) {
	agents := []ProfileAgent{{Name: "a", Manager: "a"}}
	if err := validateTopology(agents); err == nil {
		t.Error("expected self-manager error")
	}
}

func TestValidateTopology_RejectsUnknownManager(t *testing.T) {
	agents := []ProfileAgent{{Name: "a", Manager: "ghost"}}
	if err := validateTopology(agents); err == nil {
		t.Error("expected unknown-manager error")
	}
}

func TestValidateTopology_RejectsUnknownPeer(t *testing.T) {
	agents := []ProfileAgent{{Name: "a", Peers: []string{"ghost"}}}
	if err := validateTopology(agents); err == nil {
		t.Error("expected unknown-peer error")
	}
}

func TestValidateTopology_AllowsMultiTier(t *testing.T) {
	agents := []ProfileAgent{
		{Name: "lead"},
		{Name: "mid", Manager: "lead"},
		{Name: "low", Manager: "mid"},
	}
	if err := validateTopology(agents); err != nil {
		t.Errorf("multi-tier chain should validate: %v", err)
	}
}

// Apply with multi-tier topology writes topology.json on each agent.
func TestIntegration_ApplyTopologyArtefacts(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "tp.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "tp", "version": "0"},
  "team": {"name": "tp"},
  "agents": [
    {"name": "lead", "role": "manager"},
    {"name": "wa",   "role": "worker", "manager": "lead", "peers": ["wb"]},
    {"name": "wb",   "role": "worker", "manager": "lead", "peers": ["wa"]}
  ]
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile, "--skip-audit"); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	leadTopo, err := os.ReadFile(filepath.Join(root, "tp", "lead", "workspace", "topology.json"))
	if err != nil {
		t.Fatalf("lead topology.json missing: %v", err)
	}
	if !strings.Contains(string(leadTopo), `"workers"`) || !strings.Contains(string(leadTopo), `"wa"`) {
		t.Errorf("lead.topology should list workers including wa: %s", leadTopo)
	}
	waTopo, err := os.ReadFile(filepath.Join(root, "tp", "wa", "workspace", "topology.json"))
	if err != nil {
		t.Fatalf("wa topology.json missing: %v", err)
	}
	if !strings.Contains(string(waTopo), `"manager": "lead"`) || !strings.Contains(string(waTopo), `"wb"`) {
		t.Errorf("wa.topology should list manager=lead + peer=wb: %s", waTopo)
	}
}
