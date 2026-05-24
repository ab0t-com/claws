package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type RegistryEntry struct {
	Index int
	Name  string
}

func ensureRegistry(paths Paths) error {
	if err := os.MkdirAll(filepath.Dir(paths.PortRegistry), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(paths.PortRegistry); os.IsNotExist(err) {
		return os.WriteFile(paths.PortRegistry, nil, 0600)
	}
	return nil
}

func readRegistry(paths Paths) ([]RegistryEntry, error) {
	if err := ensureRegistry(paths); err != nil {
		return nil, err
	}
	f, err := os.Open(paths.PortRegistry)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []RegistryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		idx, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		entries = append(entries, RegistryEntry{Index: idx, Name: parts[1]})
	}
	return entries, scanner.Err()
}

// nextIndex finds the lowest free index for a new agent.
//
// "Free" means BOTH conditions:
//
//   1. Not already in the registry, AND
//   2. The host port it would compute to (basePort + index*portStep) is
//      not currently bound by some other process / container.
//
// The second check prevents the bug where an orphan container is sitting
// on the port that would otherwise be allocated to a fresh agent — the
// agent would be created and immediately fail to start. By skipping
// externally-held ports here, the new agent gets a guaranteed-free port
// from the start. We cap the scan at a sensible upper bound so a
// pathological "thousands of held ports" situation surfaces as an error
// rather than spinning.
func nextIndex(paths Paths) (int, error) {
	entries, err := readRegistry(paths)
	if err != nil {
		return 0, err
	}
	used := make(map[int]bool)
	for _, e := range entries {
		used[e.Index] = true
	}
	const maxIndex = 10000
	for i := 0; i < maxIndex; i++ {
		if used[i] {
			continue
		}
		if portInUse(portForIndex(i)) {
			// Port is held by something we don't manage (orphan, foreign
			// process, etc). Skipping rather than failing — the operator
			// can investigate later via `claws orphans` or `ss -tlnp`.
			continue
		}
		return i, nil
	}
	return 0, fmt.Errorf("no free port index found in [0, %d) — too many in-use ports?", maxIndex)
}

func registerPort(paths Paths, index int, name string) error {
	if err := ensureRegistry(paths); err != nil {
		return err
	}
	f, err := os.OpenFile(paths.PortRegistry, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d:%s\n", index, name)
	return err
}

func unregisterPort(paths Paths, name string) error {
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	var lines []string
	for _, e := range entries {
		if e.Name != name {
			lines = append(lines, fmt.Sprintf("%d:%s", e.Index, e.Name))
		}
	}
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	return os.WriteFile(paths.PortRegistry, []byte(content), 0600)
}

// allocatePort atomically finds the next available index and registers it.
// Returns the allocated index. Must be called under withRegistryLock.
func allocatePort(paths Paths, name string) (int, error) {
	index, err := nextIndex(paths)
	if err != nil {
		return 0, err
	}
	if err := registerPort(paths, index, name); err != nil {
		return 0, err
	}
	return index, nil
}

// lockedAllocatePort wraps allocatePort with the registry file lock.
func lockedAllocatePort(paths Paths, name string) (int, error) {
	var index int
	err := withRegistryLock(paths, func() error {
		var allocErr error
		index, allocErr = allocatePort(paths, name)
		return allocErr
	})
	return index, err
}

// lockedUnregisterPort wraps unregisterPort with the registry file lock.
func lockedUnregisterPort(paths Paths, name string) error {
	return withRegistryLock(paths, func() error {
		return unregisterPort(paths, name)
	})
}

func portForIndex(index int) int {
	return basePort() + index*portStep
}

func instanceCount(paths Paths) int {
	var count int
	withRegistryLock(paths, func() error {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		count = len(entries)
		return nil
	})
	return count
}
