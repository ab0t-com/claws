package hints

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// engine
// ---------------------------------------------------------------------------

func TestProfile_Limit(t *testing.T) {
	cases := map[Profile]int{
		ProfileDefault: 4,
		ProfileAgent:   8,
		ProfileTerse:   1,
		ProfileOff:     0,
		Profile("xyz"): 4, // unknown falls back to default
	}
	for p, want := range cases {
		if got := p.Limit(); got != want {
			t.Errorf("%q.Limit() = %d, want %d", p, got, want)
		}
	}
}

func TestProfile_IsValid(t *testing.T) {
	for _, p := range []Profile{ProfileDefault, ProfileAgent, ProfileTerse, ProfileOff} {
		if !p.IsValid() {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []Profile{"", "verbose", "DEFAULT"} {
		if p.IsValid() {
			t.Errorf("%q should NOT be valid", p)
		}
	}
}

func TestResolveProfile(t *testing.T) {
	cases := []struct {
		flag, env string
		isJSON    bool
		want      Profile
	}{
		{"agent", "", false, ProfileAgent},                    // flag wins
		{"AGENT", "", false, ProfileAgent},                    // normalised
		{"  terse  ", "", false, ProfileTerse},                // trimmed
		{"", "off", false, ProfileOff},                        // env wins when flag empty
		{"bogus", "default", false, ProfileDefault},           // unknown flag falls through
		{"", "", true, ProfileAgent},                          // JSON auto-promote
		{"", "", false, ProfileDefault},                       // last-resort default
		{"default", "agent", true, ProfileDefault},            // flag beats env beats json-promotion
	}
	for _, c := range cases {
		got := ResolveProfile(c.flag, c.env, c.isJSON)
		if got != c.want {
			t.Errorf("Resolve(%q, %q, %v) = %q, want %q", c.flag, c.env, c.isJSON, got, c.want)
		}
	}
}

func TestFor_Truncates(t *testing.T) {
	// Register under a test-only key so we don't clobber any production
	// provider. Per the package's README: never call Clear() in tests.
	Register("__test_many", func(Context) []Hint {
		out := make([]Hint, 10)
		for i := range out {
			out[i] = Hint{Name: "h", Command: "x"}
		}
		return out
	})
	if got := For("__test_many", Context{Profile: ProfileDefault}); len(got.Hints) != 4 {
		t.Errorf("default profile truncation: got %d, want 4", len(got.Hints))
	}
	if got := For("__test_many", Context{Profile: ProfileTerse}); len(got.Hints) != 1 {
		t.Errorf("terse profile truncation: got %d, want 1", len(got.Hints))
	}
	if got := For("__test_many", Context{Profile: ProfileOff}); len(got.Hints) != 0 {
		t.Errorf("off profile: got %d, want 0", len(got.Hints))
	}
}

func TestFor_TerseStripsReason(t *testing.T) {
	Register("__test_withreason", func(Context) []Hint {
		return []Hint{{Name: "x", Command: "y", Reason: "should be stripped under terse"}}
	})
	got := For("__test_withreason", Context{Profile: ProfileTerse})
	if len(got.Hints) != 1 {
		t.Fatalf("got %d hints, want 1", len(got.Hints))
	}
	if got.Hints[0].Reason != "" {
		t.Errorf("terse should strip reason, got %q", got.Hints[0].Reason)
	}
	// Sanity: default keeps it.
	got = For("__test_withreason", Context{Profile: ProfileDefault})
	if got.Hints[0].Reason == "" {
		t.Error("default should preserve reason")
	}
}

func TestFor_UnknownCommand(t *testing.T) {
	// Don't clear — providers init() registers built-ins; we just check
	// an unknown name returns an empty set without panicking.
	got := For("no-such-command", Context{Profile: ProfileDefault})
	if len(got.Hints) != 0 {
		t.Errorf("unknown command should return 0 hints, got %d", len(got.Hints))
	}
}

// ---------------------------------------------------------------------------
// providers (claws-specific)
// ---------------------------------------------------------------------------

func TestProvider_Toplevel_Empty(t *testing.T) {
	hs := For(Toplevel, Context{Profile: ProfileDefault, AgentTotal: 0})
	if len(hs.Hints) == 0 {
		t.Fatal("empty-fleet toplevel should suggest setup")
	}
	// Should lead with guided_setup.
	if hs.Hints[0].Name != "guided_setup" {
		t.Errorf("first hint should be guided_setup, got %q", hs.Hints[0].Name)
	}
}

func TestProvider_Toplevel_MixedFleet(t *testing.T) {
	hs := For(Toplevel, Context{
		Profile:           ProfileDefault,
		AgentTotal:        5,
		AgentHealthy:      1,
		AgentNeverStarted: 4,
	})
	names := names(hs.Hints)
	if !contains(names, "start_all") {
		t.Errorf("want start_all hint when AgentNeverStarted>0; got %v", names)
	}
	if !contains(names, "live_status") {
		t.Errorf("want live_status hint when AgentHealthy>0; got %v", names)
	}
	if !contains(names, "list_agents") {
		t.Errorf("want list_agents always; got %v", names)
	}
}

func TestProvider_List_EmptyVsPopulated(t *testing.T) {
	empty := For("list", Context{Profile: ProfileDefault})
	if len(empty.Hints) != 1 || empty.Hints[0].Name != "create_first" {
		t.Errorf("empty list should suggest create_first, got %+v", empty.Hints)
	}

	healthy := For("list", Context{
		Profile:      ProfileDefault,
		AgentTotal:   2,
		AgentHealthy: 1,
		Agents: []AgentRef{
			{Name: "team/alpha", Status: "healthy"},
			{Name: "team/beta", Status: "created"},
		},
		AgentNeverStarted: 1,
	})
	names := names(healthy.Hints)
	if !contains(names, "ping_one_healthy") {
		t.Errorf("want ping_one_healthy when at least one is healthy: %v", names)
	}
	if !contains(names, "start_all") {
		t.Errorf("want start_all when some never started: %v", names)
	}
}

func TestProvider_Start_RequiresAgentName(t *testing.T) {
	hs := For("start", Context{Profile: ProfileDefault})
	if len(hs.Hints) != 0 {
		t.Errorf("start without AgentName should emit nothing, got %+v", hs.Hints)
	}
	hs = For("start", Context{Profile: ProfileDefault, AgentName: "team/alpha"})
	if len(hs.Hints) < 2 {
		t.Errorf("start with AgentName should emit ping + logs, got %+v", hs.Hints)
	}
}

func TestProvider_Update_FleetDoctorOnlyIfAgents(t *testing.T) {
	empty := For("update", Context{Profile: ProfileDefault})
	if contains(names(empty.Hints), "fleet_doctor") {
		t.Errorf("update should not suggest fleet_doctor when no agents: %v", names(empty.Hints))
	}
	withAgents := For("update", Context{Profile: ProfileDefault, AgentTotal: 3})
	if !contains(names(withAgents.Hints), "fleet_doctor") {
		t.Errorf("update SHOULD suggest fleet_doctor when there are agents: %v", names(withAgents.Hints))
	}
}

// ---------------------------------------------------------------------------
// render
// ---------------------------------------------------------------------------

func TestRenderText_OffEmits(t *testing.T) {
	var buf bytes.Buffer
	RenderText(&buf, HintSet{Profile: ProfileOff, Hints: []Hint{{Name: "x", Command: "y"}}})
	if buf.Len() != 0 {
		t.Errorf("off profile should emit nothing, got %q", buf.String())
	}
}

func TestRenderText_EmptySetEmits(t *testing.T) {
	var buf bytes.Buffer
	RenderText(&buf, HintSet{Profile: ProfileDefault})
	if buf.Len() != 0 {
		t.Errorf("empty hint set should emit nothing, got %q", buf.String())
	}
}

func TestRenderText_FormatsBlock(t *testing.T) {
	var buf bytes.Buffer
	RenderText(&buf, HintSet{Profile: ProfileDefault, Hints: []Hint{
		{Name: "a", Command: "claws a", Reason: "do A"},
		{Name: "b", Command: "claws b", Reason: "do B"},
	}})
	out := buf.String()
	if !strings.Contains(out, "Next:") {
		t.Errorf("expected Next: header, got %q", out)
	}
	if !strings.Contains(out, "claws a") || !strings.Contains(out, "claws b") {
		t.Errorf("expected both commands in output, got %q", out)
	}
}

func TestHint_JSONShape(t *testing.T) {
	h := Hint{Name: "n", Command: "c", Reason: "r"}
	data, _ := json.Marshal(h)
	want := `{"name":"n","command":"c","reason":"r"}`
	if string(data) != want {
		t.Errorf("got %s want %s", data, want)
	}
	// Reason omitempty.
	h.Reason = ""
	data, _ = json.Marshal(h)
	if strings.Contains(string(data), "reason") {
		t.Errorf("empty reason should be omitted: %s", data)
	}
}

// helpers ---------------------------------------------------------------------

func names(hs []Hint) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.Name
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
