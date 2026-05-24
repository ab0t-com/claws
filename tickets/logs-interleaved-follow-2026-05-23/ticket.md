# Ticket: `claws logs --group=<name> -f` (interleaved multi-instance follow)

**Created:** 2026-05-23
**Status:** Open
**Priority:** P2 — Medium (useful during multi-agent incidents; non-follow `--group=` already shipped)
**Owner:** unassigned

---

## Problem

`claws logs --group=<team>` (non-follow) ships in Task F+H of the fleet-team-control ticket — it iterates members sequentially with section headers (`=== team/sarah ===`). Operators investigating a *past* incident can use this.

But `claws logs --group=<team> -f` (live interleaved tail) is rejected with a directive error today:

> `--group= with -f (interleaved follow) is not yet supported — run -f per instance, or omit -f for sequential tail`

This is what operators actually want during a *live* incident — see logs from all team members streaming in real time, prefixed by member name, in one terminal.

## Design

Per-member goroutine pattern:

1. For each member, fork a `docker compose -p <project> logs -f --no-color --timestamps <service>` subprocess.
2. Each subprocess pipes stdout into a bufio.Scanner running in its own goroutine.
3. Each goroutine sends `prefixed-line` strings into a shared `chan string`.
4. A single output goroutine reads from the chan and writes to stdout, applying a per-member color prefix.
5. Signal handler on SIGINT — close the chan, kill all subprocesses, exit cleanly.

Subtleties:
- **Line-buffering**: docker compose log output is line-buffered already; the scanner just needs a 1MB buffer for long lines (same as non-follow grep path).
- **Color cycling**: assign each member a stable ANSI color (cycle through a small palette) so the operator can eyeball which line came from where without reading the prefix every time.
- **Graceful shutdown**: every subprocess gets a `cmd.Process.Kill()` on SIGINT. Use `context.WithCancel` to plumb cancellation through.
- **Grep interaction**: `logs --group= -f --grep=401` should filter each line through the grep before printing. Easy: filter in the output goroutine.

## Reference implementation skeleton

```go
type tailLine struct {
    member string
    line   string
}

func cmdLogsGroupFollow(paths Paths, members []RegistryEntry, grep string) error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() { <-sigCh; cancel() }()
    
    lineCh := make(chan tailLine, 100)
    var wg sync.WaitGroup
    for _, m := range members {
        wg.Add(1)
        go followOne(ctx, &wg, m, lineCh) // builds docker compose command, streams, filters
    }
    go func() { wg.Wait(); close(lineCh) }()
    
    pat := strings.ToLower(grep)
    palette := []string{"\033[36m", "\033[33m", "\033[35m", "\033[32m"} // cyan/yellow/magenta/green
    colorOf := assignColors(members, palette)
    
    for l := range lineCh {
        if grep != "" && !strings.Contains(strings.ToLower(l.line), pat) {
            continue
        }
        fmt.Printf("%s[%s]\033[0m %s\n", colorOf[l.member], l.member, l.line)
    }
    return ctx.Err()
}
```

## Acceptance criteria

- [ ] `claws logs --group=<team> -f` streams interleaved live logs from every team member, prefixed by member name with a stable color per member.
- [ ] `Ctrl-C` cleanly stops all subprocesses (no orphaned `docker compose logs -f` processes).
- [ ] `--grep=<pattern>` composes with `-f --group=` — filter applied per line.
- [ ] Buffer sized for 1MB lines (some OpenClaw log lines are long).
- [ ] Help text on `claws logs --help` documents the combined form.
- [ ] Integration test that fires up a real `docker compose` and asserts that at least one line from each member is received within a short window. Gated behind `testing.Short()` like the existing `TestIntegration_StartGroupExpansion`.

## Related

- `tickets/fleet-team-control-surface-2026-05-23/` Task F+H worklog: documents why this was deferred from the parent ticket.
- The non-follow `--group=` path is the fallback today and remains useful.

## Effort

~half day.
