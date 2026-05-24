package main

import (
	"testing"
	"time"
)

// TestSafeSendDoesNotBlock confirms that a producer hitting a full buffer
// drops the line instead of blocking forever. The motivating concern: if
// the renderer exits while a fast member is mid-burst, we don't want the
// reader goroutine to hang holding the docker subprocess pipe open.
func TestSafeSendDoesNotBlock(t *testing.T) {
	ch := make(chan tailLine, 2)
	safeSend(ch, "alpha", "line-1", false)
	safeSend(ch, "alpha", "line-2", false)

	// Third call must not block. If safeSend were implemented with a plain
	// `ch <- ...`, this test would deadlock and hit testing.T's timeout.
	done := make(chan struct{})
	go func() {
		safeSend(ch, "alpha", "line-3", false)
		close(done)
	}()
	select {
	case <-done:
		// Good — returned without blocking.
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("safeSend blocked when channel was full; expected drop semantics")
	}

	// First two lines preserved; third was dropped.
	got1 := <-ch
	got2 := <-ch
	if got1.line != "line-1" || got2.line != "line-2" {
		t.Errorf("expected first two lines preserved, got %q + %q", got1.line, got2.line)
	}
	select {
	case extra := <-ch:
		t.Errorf("expected no third line in buffer; got %q", extra.line)
	default:
		// Good — buffer drained.
	}
}
