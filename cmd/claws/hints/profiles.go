package hints

import "strings"

// Profile controls how many hints emit and in what register.
type Profile string

const (
	// ProfileDefault is the human-facing register: 1–4 contextual hints
	// with reasons. Used for text output in interactive shells.
	ProfileDefault Profile = "default"

	// ProfileAgent is the machine register: up to 8 hints with reasons,
	// designed for AI agents / scripts consuming JSON envelopes.
	ProfileAgent Profile = "agent"

	// ProfileTerse is the single-line register: 0 or 1 hint, no reason.
	ProfileTerse Profile = "terse"

	// ProfileOff suppresses hints entirely.
	ProfileOff Profile = "off"
)

// Limit returns the maximum number of hints a profile will emit.
// 0 means "unlimited" (no profile currently uses 0).
func (p Profile) Limit() int {
	switch p {
	case ProfileAgent:
		return 8
	case ProfileDefault:
		return 4
	case ProfileTerse:
		return 1
	case ProfileOff:
		return 0
	default:
		return 4
	}
}

// IsValid reports whether p is one of the recognised profiles.
func (p Profile) IsValid() bool {
	switch p {
	case ProfileDefault, ProfileAgent, ProfileTerse, ProfileOff:
		return true
	}
	return false
}

// ResolveProfile picks the effective profile from three sources of input,
// in order of precedence:
//
//  1. The CLI flag value (`--hints <profile>`), if non-empty.
//  2. The environment variable value (CLAWS_HINTS), if non-empty.
//  3. When isJSON is true, promote to ProfileAgent.
//  4. Default to ProfileDefault.
//
// Unrecognised values fall through to the next step rather than failing.
func ResolveProfile(flag, env string, isJSON bool) Profile {
	if p := normalizeProfile(flag); p.IsValid() {
		return p
	}
	if p := normalizeProfile(env); p.IsValid() {
		return p
	}
	if isJSON {
		return ProfileAgent
	}
	return ProfileDefault
}

func normalizeProfile(s string) Profile {
	return Profile(strings.ToLower(strings.TrimSpace(s)))
}
