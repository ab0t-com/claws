package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileLock_Exclusive(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.lock")

	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(2)

	// Goroutine 1: acquire lock, hold 100ms, append 1
	go func() {
		defer wg.Done()
		withFileLock(lockPath, func() error {
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		})
	}()

	// Give goroutine 1 a moment to acquire the lock
	time.Sleep(10 * time.Millisecond)

	// Goroutine 2: will block until goroutine 1 releases, then append 2
	go func() {
		defer wg.Done()
		withFileLock(lockPath, func() error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		})
	}()

	wg.Wait()

	if len(order) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 {
		t.Errorf("expected [1, 2], got %v — lock did not serialize access", order)
	}
}

func TestRegistryLock_ConcurrentRegister(t *testing.T) {
	paths := testPaths(t)

	var wg sync.WaitGroup
	count := 10
	wg.Add(count)

	var errors atomic.Int32

	for i := 0; i < count; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := lockedAllocatePort(paths, fmt.Sprintf("instance-%d", idx))
			if err != nil {
				errors.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if errors.Load() > 0 {
		t.Errorf("%d allocations failed", errors.Load())
	}

	// Verify all got unique indices
	entries, err := readRegistry(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("expected %d entries, got %d", count, len(entries))
	}

	indices := make(map[int]bool)
	for _, e := range entries {
		if indices[e.Index] {
			t.Errorf("duplicate index %d", e.Index)
		}
		indices[e.Index] = true
	}
}

func TestLockedUnregisterPort(t *testing.T) {
	paths := testPaths(t)

	lockedAllocatePort(paths, "alpha")
	lockedAllocatePort(paths, "bravo")

	if err := lockedUnregisterPort(paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	entries, _ := readRegistry(paths)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "bravo" {
		t.Errorf("remaining entry should be bravo, got %s", entries[0].Name)
	}
}

func TestWithFileLock_CreatesLockFile(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "subdir", "test.lock")

	err := withFileLock(lockPath, func() error {
		// Lock file should exist while held
		if _, err := os.Stat(lockPath); err != nil {
			t.Error("lock file should exist while lock is held")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInstanceCount_Locked(t *testing.T) {
	paths := testPaths(t)

	if c := instanceCount(paths); c != 0 {
		t.Errorf("empty count should be 0, got %d", c)
	}

	lockedAllocatePort(paths, "alpha")
	lockedAllocatePort(paths, "bravo")

	if c := instanceCount(paths); c != 2 {
		t.Errorf("count should be 2, got %d", c)
	}
}
