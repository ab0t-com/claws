package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cmdTeamTree renders a team's topology as an ASCII tree.
// `--json` emits machine-readable form.
// Wired as `claws team tree <team>` (the existing `team show` keeps its
// table-style member listing).
//
// Reads each agent's workspace/topology.json (written by applyTopology in
// v1.5) to discover manager/peers/workers links. Falls back to walking the
// port registry + instance dirs if topology.json is missing.
func cmdTeamTree(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws team show <team> [--json]")
	}
	team := args[0]
	jsonOut := false
	for _, a := range args[1:] {
		if a == "--json" {
			jsonOut = true
		}
	}

	paths := resolvePaths()
	entries, err := readRegistry(paths)
	if err != nil {
		return errorf("read registry: %v", err)
	}

	// Gather all agents in the team and their topology.
	type agentNode struct {
		Name    string   `json:"name"`
		Role    string   `json:"role"`
		Manager string   `json:"manager"`
		Peers   []string `json:"peers"`
		Workers []string `json:"workers"`
	}
	nodes := map[string]*agentNode{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, team+"/") {
			continue
		}
		_, name := splitFull(e.Name)
		n := &agentNode{Name: name}
		// Read topology.json if present.
		if data, err := os.ReadFile(filepath.Join(paths.Root, e.Name, "workspace", "topology.json")); err == nil {
			var t struct {
				Role    string   `json:"role"`
				Manager string   `json:"manager"`
				Peers   []string `json:"peers"`
				Workers []string `json:"workers"`
			}
			if err := json.Unmarshal(data, &t); err == nil {
				n.Role = t.Role
				n.Manager = t.Manager
				n.Peers = t.Peers
				n.Workers = t.Workers
			}
		}
		nodes[name] = n
	}

	if len(nodes) == 0 {
		return errorf("team %q has no agents (or is not a team)", team)
	}

	if jsonOut {
		out := map[string]interface{}{
			"team":   team,
			"agents": nodes,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Render ASCII tree. Roots = agents with no manager (or manager not in team).
	const (
		bold = "\033[1m"
		dim  = "\033[0;90m"
		nc   = "\033[0m"
	)
	fmt.Printf("%s%s%s/\n", bold, team, nc)

	// Sort for stable output.
	var roots []string
	for name, n := range nodes {
		if n.Manager == "" || nodes[n.Manager] == nil {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)

	var render func(name, prefix string, isLast bool)
	render = func(name, prefix string, isLast bool) {
		n := nodes[name]
		if n == nil {
			return
		}
		marker := "├── "
		nextPrefix := prefix + "│   "
		if isLast {
			marker = "└── "
			nextPrefix = prefix + "    "
		}
		role := n.Role
		if role == "" {
			role = "—"
		}
		peerInfo := ""
		if len(n.Peers) > 0 {
			peerInfo = fmt.Sprintf(" %s(peers: %s)%s", dim, strings.Join(n.Peers, ", "), nc)
		}
		fmt.Printf("%s%s%s%s%s (%s)%s\n", prefix, marker, bold, name, nc, role, peerInfo)

		// Walk workers (sorted).
		workers := append([]string(nil), n.Workers...)
		sort.Strings(workers)
		for i, w := range workers {
			render(w, nextPrefix, i == len(workers)-1)
		}
	}

	for i, r := range roots {
		render(r, "", i == len(roots)-1)
	}
	fmt.Println()
	return nil
}
