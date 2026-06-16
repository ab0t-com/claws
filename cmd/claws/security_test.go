package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTier(t *testing.T) {
	cases := []struct {
		in   string
		want SecurityTier
		ok   bool
	}{
		{"untrusted", TierUntrusted, true},
		{"standard", TierStandard, true},
		{"privileged", TierPrivileged, true},
		{"host-reach", TierHostReach, true},
		{"", "", false},
		{"unknown", "", false},
		{"Standard", "", false}, // case-sensitive
	}
	for _, c := range cases {
		got, ok := parseTier(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseTier(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestTierRank(t *testing.T) {
	// Promotion rank order: untrusted < standard < privileged < host-reach
	tiers := []SecurityTier{TierUntrusted, TierStandard, TierPrivileged, TierHostReach}
	for i := 1; i < len(tiers); i++ {
		if !(tierRank(tiers[i-1]) < tierRank(tiers[i])) {
			t.Errorf("tier rank order broken: %s (%d) should be < %s (%d)",
				tiers[i-1], tierRank(tiers[i-1]), tiers[i], tierRank(tiers[i]))
		}
	}
}

func TestRequiresAcceptRisk(t *testing.T) {
	cases := []struct {
		tier SecurityTier
		want bool
	}{
		{TierUntrusted, false},
		{TierStandard, false},
		{TierPrivileged, true},
		{TierHostReach, true},
	}
	for _, c := range cases {
		if got := c.tier.requiresAcceptRisk(); got != c.want {
			t.Errorf("%s.requiresAcceptRisk() = %v, want %v", c.tier, got, c.want)
		}
	}
}

func TestRequiresHostReachAck(t *testing.T) {
	if !TierHostReach.requiresHostReachAck() {
		t.Error("TierHostReach must require the pwn-the-host ack")
	}
	for _, t2 := range []SecurityTier{TierUntrusted, TierStandard, TierPrivileged} {
		if t2.requiresHostReachAck() {
			t.Errorf("%s must NOT require the pwn-the-host ack", t2)
		}
	}
}

// TestSecurityComposeYAML_Standard — standard tier emits no overlay file.
// This is the common case; not writing a file at all means docker compose
// doesn't even see a third -f arg, matching the pre-tier behavior exactly.
func TestSecurityComposeYAML_Standard(t *testing.T) {
	rt := openclawRuntime()
	yml := securityComposeYAML(rt, TierStandard)
	if yml != "" {
		t.Errorf("standard tier must emit empty yml; got %d bytes", len(yml))
	}
}

// TestSecurityComposeYAML_PrivilegedHasReset — the privileged tier MUST use
// !reset to actually clear the base file's cap_drop and security_opt; without
// it, compose's list-append merge leaves no-new-privileges in place and sudo
// silently doesn't work.
func TestSecurityComposeYAML_PrivilegedHasReset(t *testing.T) {
	rt := openclawRuntime()
	yml := securityComposeYAML(rt, TierPrivileged)
	required := []string{
		"cap_drop: !reset []",
		"security_opt: !reset []",
		"SETUID",
		"SETGID",
		"DAC_OVERRIDE",
		"# Tier: privileged",
	}
	for _, want := range required {
		if !strings.Contains(yml, want) {
			t.Errorf("privileged tier yml missing %q\n--- got ---\n%s", want, yml)
		}
	}
	// Must apply to BOTH gateway and CLI services.
	for _, svc := range []string{rt.GatewayService, rt.CLIService} {
		if !strings.Contains(yml, "  "+svc+":\n") {
			t.Errorf("privileged tier yml missing service %q", svc)
		}
	}
}

// TestSecurityComposeYAML_HostReachAddsDockerSock — host-reach must add the
// docker.sock mount + pid=host so the agent can reach other containers.
func TestSecurityComposeYAML_HostReachAddsDockerSock(t *testing.T) {
	rt := openclawRuntime()
	yml := securityComposeYAML(rt, TierHostReach)
	required := []string{
		"/var/run/docker.sock:/var/run/docker.sock",
		`pid: "host"`,
		"- ALL",
		"cap_drop: !reset []",
	}
	for _, want := range required {
		if !strings.Contains(yml, want) {
			t.Errorf("host-reach yml missing %q\n--- got ---\n%s", want, yml)
		}
	}
}

// TestSecurityComposeYAML_UntrustedReadOnly — untrusted tier adds read_only
// rootfs + tmpfs mounts. (Base hardening from docker-compose.yml stays in
// place via merge — that's the point.)
func TestSecurityComposeYAML_UntrustedReadOnly(t *testing.T) {
	rt := openclawRuntime()
	yml := securityComposeYAML(rt, TierUntrusted)
	if !strings.Contains(yml, "read_only: true") {
		t.Errorf("untrusted tier must set read_only: true\n%s", yml)
	}
	if !strings.Contains(yml, "tmpfs:") {
		t.Errorf("untrusted tier must declare tmpfs mounts\n%s", yml)
	}
}

// TestTierEnvRoundTrip — the tier persists to instance.env and reads back.
// Uses a temp dir to avoid touching a real instance.
func TestTierEnvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "instance.env")
	// Seed with some other content so we exercise the in-place rewrite.
	if err := os.WriteFile(envFile, []byte("OPENCLAW_GATEWAY_PORT=18789\nOTHER=value\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Round trip via the lower-level primitives that security.go uses.
	tiers := []SecurityTier{TierUntrusted, TierPrivileged, TierHostReach, TierStandard}
	for _, tier := range tiers {
		// Rewrite the file with the new key.
		data, _ := os.ReadFile(envFile)
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, securityTierEnvKey+"=") {
				lines = append(lines, line)
			}
		}
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if tier != TierStandard {
			lines = append(lines, securityTierEnvKey+"="+string(tier))
		}
		os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), 0600)

		// Read back via the same helper claws uses.
		got := readEnvValue(envFile, securityTierEnvKey)
		want := string(tier)
		if tier == TierStandard {
			want = "" // standard is stored as absence
		}
		if got != want {
			t.Errorf("round-trip %s: got %q, want %q", tier, got, want)
		}

		// Verify pre-existing keys survived.
		if readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT") != "18789" {
			t.Errorf("round-trip %s clobbered OPENCLAW_GATEWAY_PORT", tier)
		}
		if readEnvValue(envFile, "OTHER") != "value" {
			t.Errorf("round-trip %s clobbered OTHER", tier)
		}
	}
}

// TestTierDescribe — describe() returns non-empty for every defined tier.
func TestTierDescribe(t *testing.T) {
	for _, tier := range validTiers {
		if d := tier.describe(); d == "" {
			t.Errorf("tier %s has empty describe()", tier)
		}
	}
}

// TestTierColor — every tier maps to a non-empty ANSI sequence (used by list).
func TestTierColor(t *testing.T) {
	for _, tier := range validTiers {
		if c := tierColor(tier); c == "" {
			t.Errorf("tier %s has empty color", tier)
		}
	}
}
