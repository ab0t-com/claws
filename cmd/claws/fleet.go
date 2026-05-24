package main

import (
	"fmt"
)

// cmdFleet dispatches `claws fleet <subcommand>`.
// Currently: `claws fleet doctor` (env + audit + drift + orphans in one).
func cmdFleet(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws fleet <subcommand>

Subcommands:
  doctor   Combined env + security + drift + orphan check (exit non-zero on any failure)

Use the per-area commands directly for narrower scopes:
  claws doctor      — environment-only
  claws audit       — security policy
  claws drift       — state consistency
  claws orphans     — stray Docker containers`)
		return nil
	}
	switch args[0] {
	case "doctor":
		return fleetDoctor()
	default:
		return errorf("unknown fleet subcommand %q (use doctor)", args[0])
	}
}

// fleetDoctor runs doctor + audit + drift + orphans, sections clearly,
// returns non-zero on any failure.
func fleetDoctor() error {
	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		gold  = "\033[0;33m"
		red   = "\033[0;31m"
		green = "\033[0;32m"
		nc    = "\033[0m"
	)

	type section struct {
		name string
		fn   func() error
	}
	sections := []section{
		{"Environment (claws doctor)", func() error { return cmdDoctor(nil) }},
		{"Security audit (claws audit)", func() error { return cmdAudit(nil) }},
		{"State drift (claws drift)", func() error { return cmdDrift(nil) }},
		{"Orphan containers (claws orphans)", func() error { return cmdOrphans(nil) }},
	}

	type result struct {
		name string
		err  error
	}
	var results []result
	for _, s := range sections {
		fmt.Printf("\n%s================================================================%s\n", bold, nc)
		fmt.Printf("%s   %s%s\n", bold, s.name, nc)
		fmt.Printf("%s================================================================%s\n\n", bold, nc)
		err := s.fn()
		results = append(results, result{s.name, err})
	}

	// Summary.
	fmt.Printf("\n%s================================================================%s\n", bold, nc)
	fmt.Printf("%s   Fleet doctor — summary%s\n", bold, nc)
	fmt.Printf("%s================================================================%s\n\n", bold, nc)
	failures := 0
	for _, r := range results {
		mark := green + "✓" + nc
		status := "ok"
		if r.err != nil {
			mark = red + "✗" + nc
			status = r.err.Error()
			failures++
		}
		fmt.Printf("  %s %s\n", mark, r.name)
		if r.err != nil {
			fmt.Printf("    %s%s%s\n", dim, status, nc)
		}
	}
	if failures > 0 {
		fmt.Printf("\n%s%d / %d sections reported issues — review the section output above.%s\n",
			gold, failures, len(results), nc)
		return errorf("%d section(s) failed", failures)
	}
	fmt.Printf("\n%sFleet is healthy. All %d sections passed.%s\n", green, len(results), nc)
	return nil
}
