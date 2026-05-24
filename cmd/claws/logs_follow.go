package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// tailLine is the single record type carried over the multiplex channel.
// Named (not anonymous) so the per-member reader helpers can take it as
// a parameter; Go's anonymous-struct types don't compare structurally
// across function signatures.
type tailLine struct {
	member string
	line   string
	isErr  bool // stderr lines get a [err] marker; stdout is the default
}

// logsGroupFollow streams `docker compose logs -f` from every member of a
// group, multiplexed onto stdout with a per-member ANSI color prefix.
// Lines are line-buffered through a 1MB scanner buffer (OpenClaw log lines
// can be long). Ctrl-C cancels the context, which cancels every member's
// subprocess (CommandContext kills via SIGKILL on cancel), and the
// function returns cleanly once all per-member readers drain.
//
// `grep` is an optional case-insensitive substring filter; empty means
// pass everything through.
func logsGroupFollow(paths Paths, members []RegistryEntry, grep string) error {
	if len(members) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Forward SIGINT/SIGTERM to the context so per-member docker subprocesses
	// receive SIGKILL via CommandContext's cancel propagation.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	// Color palette cycled per member. Six visually-distinct foreground
	// colors covers a single-host fleet (cap is 8 per project policy) with
	// enough headroom that two members rarely collide in adjacent rows.
	palette := []string{
		"\033[36m", // cyan
		"\033[33m", // yellow
		"\033[35m", // magenta
		"\033[32m", // green
		"\033[34m", // blue
		"\033[31m", // red
	}
	nc := "\033[0m"
	bold := "\033[1m"

	// Channel buffer sized 4× member count: enough headroom for short bursts
	// without dropping lines, small enough that backpressure exists if a
	// renderer can't keep up (better to slow the source than to OOM the
	// process by buffering forever).
	lineCh := make(chan tailLine, len(members)*4)

	var wg sync.WaitGroup
	for i, m := range members {
		wg.Add(1)
		go followMember(ctx, &wg, paths, m.Name, palette[i%len(palette)], lineCh)
	}
	// Closer goroutine: once every member's reader exits (either via EOF
	// when its docker subprocess dies, or via ctx cancellation), close the
	// channel so the renderer loop terminates.
	go func() {
		wg.Wait()
		close(lineCh)
	}()

	pat := strings.ToLower(grep)
	// Print a one-line banner so operators know which members they're
	// tailing and what color each maps to.
	fmt.Printf("%sTailing %d instance(s):%s ", bold, len(members), nc)
	for i, m := range members {
		fmt.Printf("%s%s%s ", palette[i%len(palette)], m.Name, nc)
	}
	fmt.Println("(Ctrl-C to stop)")

	for l := range lineCh {
		if pat != "" && !strings.Contains(strings.ToLower(l.line), pat) {
			continue
		}
		// Find the color this member was assigned. Simple linear search;
		// member count is bounded at 8.
		color := nc
		for i, m := range members {
			if m.Name == l.member {
				color = palette[i%len(palette)]
				break
			}
		}
		errMarker := ""
		if l.isErr {
			errMarker = "\033[31m[err]\033[0m "
		}
		fmt.Printf("%s%s%s %s%s\n", color, l.member, nc, errMarker, l.line)
	}

	return ctx.Err()
}

// followMember spawns `docker compose -p <project> logs -f` for one
// instance and feeds its stdout/stderr line-by-line into lineCh. Exits
// when the subprocess exits or ctx is cancelled.
func followMember(ctx context.Context, wg *sync.WaitGroup, paths Paths, name, color string, lineCh chan<- tailLine) {
	defer wg.Done()

	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	override := rt.OverridePath(dir)
	composeTemplate := rt.ComposeTemplatePath(paths)

	allArgs := []string{"compose", "-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		allArgs = append(allArgs, "-f", override)
	}
	allArgs = append(allArgs, "--env-file", envFile, "-p", rt.MakeProjectName(ref))
	// --no-color so our own ANSI prefix is the only color; --timestamps so
	// the operator can see when each line actually fired even though we
	// interleave across members.
	allArgs = append(allArgs, "logs", "-f", "--no-color", "--timestamps", rt.GatewayService)

	cmd := exec.CommandContext(ctx, "docker", allArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		safeSend(lineCh, name, fmt.Sprintf("(setup error: %v)", err), true)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		safeSend(lineCh, name, fmt.Sprintf("(setup error: %v)", err), true)
		return
	}
	if err := cmd.Start(); err != nil {
		safeSend(lineCh, name, fmt.Sprintf("(failed to start: %v)", err), true)
		return
	}

	// Two reader goroutines per member: stdout and stderr. Lines feed into
	// the same channel so they interleave naturally with everyone else's
	// output and the renderer sees a single ordered stream.
	var streams sync.WaitGroup
	streams.Add(2)
	go scanPipe(&streams, stdout, name, false, lineCh)
	go scanPipe(&streams, stderr, name, true, lineCh)

	// Wait for the docker subprocess to exit (EOF on pipes) OR ctx cancel.
	// CommandContext handles the cancel side: when ctx is cancelled, cmd
	// receives SIGKILL and Wait returns "signal: killed", which is fine.
	_ = cmd.Wait()
	streams.Wait()
}

// scanPipe reads a pipe line-by-line and forwards to lineCh. Uses a 1MB
// buffer to handle long OpenClaw log lines (some Baileys / Telegram
// payloads exceed the default 64KB scanner buffer).
func scanPipe(wg *sync.WaitGroup, r io.ReadCloser, name string, isErr bool, lineCh chan<- tailLine) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		safeSend(lineCh, name, scanner.Text(), isErr)
	}
}

// safeSend pushes onto lineCh but never blocks the producer indefinitely
// if the channel is full and the consumer has gone away (e.g., context
// cancelled, renderer exiting). Select-default semantics — drop rather
// than back up.
func safeSend(lineCh chan<- tailLine, name, line string, isErr bool) {
	select {
	case lineCh <- tailLine{member: name, line: line, isErr: isErr}:
	default:
		// Buffer full — drop. This only happens when the renderer is
		// slower than the source, which on a small fleet during normal
		// ops is rare. If it becomes a problem, the buffer in the caller
		// can be scaled up.
	}
}
