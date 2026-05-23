package main

import (
	"strings"
	"testing"
)

func TestAuthErrorPatterns(t *testing.T) {
	// Should match: model-level auth errors from common providers.
	shouldMatch := []string{
		"openclaw-gateway-1  | [openai-codex] Token refresh failed: 401",
		"Error: invalid_api_key",
		"openai 401 Unauthorized",
		"anthropic returned 403 Forbidden",
		"codex client: insufficient_quota",
		"claude: authentication_error",
		"model_not_found in openai response",
		"incorrect_api_key: please check your settings",
	}
	for _, in := range shouldMatch {
		if !authErrorPatterns.MatchString(in) {
			t.Errorf("should match: %q", in)
		}
	}

	// Should NOT match: channel-level 401s (WhatsApp/Baileys), unrelated
	// 401s that aren't from an auth provider, ordinary log lines.
	shouldNotMatch := []string{
		`[whatsapp][default] channel exited: {"data":{"reason":"401","location":"frc"}}`,
		`some random log line`,
		`HTTP 500 internal error`,
		`gateway started on port 18789`,
		`agent model: openai-codex/gpt-5.4`, // descriptive, not an error
	}
	for _, in := range shouldNotMatch {
		if authErrorPatterns.MatchString(in) {
			t.Errorf("should NOT match: %q", in)
		}
	}
}

func TestMatchedLine(t *testing.T) {
	logs := "line one\nline two with 401 Unauthorized openai\nline three"
	offset := strings.Index(logs, "401")
	got := matchedLine(logs, offset)
	want := "line two with 401 Unauthorized openai"
	if got != want {
		t.Errorf("matchedLine: got %q, want %q", got, want)
	}

	// Last line (no trailing newline).
	logs2 := "first\nsecond line with error 403"
	offset2 := strings.Index(logs2, "403")
	got2 := matchedLine(logs2, offset2)
	want2 := "second line with error 403"
	if got2 != want2 {
		t.Errorf("matchedLine (last line): got %q, want %q", got2, want2)
	}
}

func TestSuggestReauthCommand(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		want     string
	}{
		{"team/sarah", "openai-codex", "clawctl auth team/sarah codex"},
		{"alpha", "codex", "clawctl auth alpha codex"},
		{"team1/ben", "anthropic", "clawctl auth team1/ben apikey anthropic <key>"},
		{"team1/ben", "claude", "clawctl auth team1/ben apikey anthropic <key>"},
		// Unknown provider falls through to the generic form.
		{"alpha", "", "clawctl auth alpha codex  (or: clawctl auth alpha apikey <provider> <key>)"},
		{"alpha", "mistral", "clawctl auth alpha codex  (or: clawctl auth alpha apikey <provider> <key>)"},
	}
	for _, tc := range cases {
		if got := suggestReauthCommand(tc.name, tc.provider); got != tc.want {
			t.Errorf("suggestReauthCommand(%q, %q) = %q, want %q", tc.name, tc.provider, got, tc.want)
		}
	}
}
