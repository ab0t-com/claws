package main

import (
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"sarah", "my-bot", "a", "test-bot-1", "abcdefghijklmnopqrstuvwxyz1234"}
	for _, name := range valid {
		if err := validateName(name); err != nil {
			t.Errorf("'%s' should be valid, got: %v", name, err)
		}
	}

	invalid := []struct {
		name   string
		reason string
	}{
		{"MyBot", "uppercase"},
		{"1bot", "starts with number"},
		{"-bot", "starts with hyphen"},
		{"my bot", "space"},
		{"my_bot", "underscore"},
		{"my.bot", "dot"},
		{"shared", "reserved"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "too long (33 chars)"},
	}
	for _, tc := range invalid {
		if err := validateName(tc.name); err == nil {
			t.Errorf("'%s' should be invalid (%s)", tc.name, tc.reason)
		}
	}
}

func TestBasePort(t *testing.T) {
	// Default
	t.Setenv("CLAWCTL_BASE_PORT", "")
	if p := basePort(); p != 18789 {
		t.Errorf("default base port should be 18789, got %d", p)
	}

	// Override
	t.Setenv("CLAWCTL_BASE_PORT", "28789")
	if p := basePort(); p != 28789 {
		t.Errorf("overridden base port should be 28789, got %d", p)
	}
}
