package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdID prints the UUID for a named agent. Script-friendly: one line, no decoration.
func cmdID(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws id <agent-name>

Print the CLAWS_INSTANCE_UUID for the named agent. Script-friendly: one
line, no decoration. Exits non-zero if the agent doesn't exist or has
no UUID (run `+"`claws migrate uuids`"+` to populate).`)
		return nil
	}
	paths := resolvePaths()
	full := args[0]
	uuid, err := readInstanceUUID(paths, full)
	if err != nil {
		return err
	}
	fmt.Println(uuid)
	return nil
}

// cmdByID does the reverse: UUID → "team/name" or "name" for flat agents.
func cmdByID(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws by-id <uuid>

Reverse-lookup: print the team/name of the agent with the given UUID.
Exits non-zero if no agent matches.`)
		return nil
	}
	paths := resolvePaths()
	want := strings.ToLower(strings.TrimSpace(args[0]))
	entries, err := readRegistry(paths)
	if err != nil {
		return errorf("read registry: %v", err)
	}
	for _, e := range entries {
		uuid, _ := readInstanceUUID(paths, e.Name)
		if strings.ToLower(uuid) == want {
			fmt.Println(e.Name)
			return nil
		}
	}
	return errorf("no agent with uuid %q", args[0])
}

// readInstanceUUID returns the CLAWS_INSTANCE_UUID value from instance.env.
func readInstanceUUID(paths Paths, full string) (string, error) {
	envPath := filepath.Join(paths.Root, full, "instance.env")
	f, err := os.Open(envPath)
	if err != nil {
		return "", errorf("agent %q: %v", full, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "CLAWS_INSTANCE_UUID=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "CLAWS_INSTANCE_UUID=")), nil
		}
	}
	return "", errorf("agent %q has no CLAWS_INSTANCE_UUID (run `claws migrate uuids`)", full)
}
