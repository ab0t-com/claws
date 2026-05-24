// Package hints provides a small, profile-driven, modular system for
// emitting "what to run next" suggestions alongside CLI command output.
//
// Lifted from sharedwatch (../sharedwatch/src/internal/hints) under its
// "this package is portable; the second project adopting it should lift
// the package" extraction note. Identical engine, claws-specific
// Context fields + providers.
//
// The package has zero dependencies on other claws packages — it consumes
// only the Context value passed in by the caller. Future move: extract
// to ab0t-com/cli-hints when a third consumer appears.
//
// Usage from a command handler:
//
//	ctx := hints.Context{
//	    Profile:           hints.ResolveProfile(flagValue, os.Getenv("CLAWS_HINTS"), isJSON),
//	    AgentTotal:        5,
//	    AgentHealthy:      1,
//	    AgentNeverStarted: 4,
//	}
//	set := hints.For("list", ctx)
//	hints.RenderText(os.Stdout, set)
package hints

import "sync"

// Hint is one next-step suggestion. Stable across releases — additive only.
type Hint struct {
	// Name is a stable identifier (e.g. "start_all_agents"). Use snake_case.
	// Consumers may key behaviour off Name; keep it semantic, not pretty.
	Name string `json:"name"`
	// Command is the full runnable command line a caller can copy-paste or exec.
	Command string `json:"command"`
	// Reason is a short, English, half-sentence explanation. Optional —
	// omitted in terse profile output.
	Reason string `json:"reason,omitempty"`
}

// HintSet is the envelope returned by For.
type HintSet struct {
	Profile Profile `json:"profile"`
	Hints   []Hint  `json:"hints"`
}

// AgentRef is a minimal projection of an agent, carried in Context so
// providers can suggest per-agent actions without importing the rest of
// the CLI.
type AgentRef struct {
	Name   string // "team/agent"
	Status string // "healthy" | "created" | "stopped" | "error"
}

// Context is the state snapshot a Provider inspects. Additive — new fields
// land with zero-value defaults so existing providers keep compiling.
type Context struct {
	// Profile is the resolved profile to emit under.
	Profile Profile

	// Fleet-wide counts (populated for top-level / list-style commands).
	AgentTotal        int
	AgentHealthy      int
	AgentNeverStarted int // status=created, never started
	AgentStopped      int
	AgentError        int // crashed / unhealthy

	// All agents, for providers that want to suggest per-agent actions.
	// Order: healthy first, then never-started, then stopped/error.
	Agents []AgentRef

	// Per-agent context (when a command operates on a specific agent).
	AgentName     string // "team/agent"
	AgentStatus   string // current status of AgentName
	AgentHasAuth  bool
	AgentHasChan  bool

	// For setup wizard / team-scoped commands.
	TeamName       string
	ExistingTeams  []string
	TeamAgents     []AgentRef // agents within TeamName

	// For `claws create` — the agent just created.
	CreatedName string

	// For `claws update`.
	NewerExists bool
	Latest      string

	// For `claws image bootstrap` and other one-shots — the verb that
	// just completed, so the provider can suggest follow-ups specific
	// to the outcome.
	JustDid string
}

// Provider is a pure function: state in, hints out. Providers must NOT
// truncate to the profile limit themselves — return everything they would
// usefully suggest; For() applies the cap.
type Provider func(ctx Context) []Hint

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Provider)
)

// Register wires a provider for a command name. Idempotent: a second
// registration replaces the first. Command names are space-separated for
// multi-word commands (e.g. "channel add"). The empty string is reserved
// for the top-level "command with no args" provider — registered by
// passing "" or the Toplevel constant.
func Register(command string, p Provider) {
	if p == nil {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[command] = p
}

// Toplevel is the registry key for the no-args invocation provider.
// Callers use For(Toplevel, ctx) and Register(Toplevel, …).
const Toplevel = ""

// For resolves the registered provider for command, runs it against ctx,
// then truncates to ctx.Profile.Limit(). Returns an empty HintSet (not nil)
// when the command has no provider or the profile is off.
func For(command string, ctx Context) HintSet {
	set := HintSet{Profile: ctx.Profile, Hints: nil}
	if ctx.Profile == ProfileOff {
		return set
	}
	registryMu.RLock()
	p := registry[command]
	registryMu.RUnlock()
	if p == nil {
		return set
	}
	raw := p(ctx)
	limit := ctx.Profile.Limit()
	if limit > 0 && len(raw) > limit {
		raw = raw[:limit]
	}
	// Terse strips reasons (single-line UX).
	if ctx.Profile == ProfileTerse {
		for i := range raw {
			raw[i].Reason = ""
		}
	}
	set.Hints = raw
	return set
}

// Clear is a test helper that wipes the registry. Production code should
// not call this; it exists so unit tests can isolate provider behaviour.
func Clear() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]Provider)
}
