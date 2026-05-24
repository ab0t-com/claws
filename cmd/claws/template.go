package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const templateHelp = `Usage: claws template <list|show|resolve> [name]

Manage and inspect claws templates.

Subcommands:
  list                    List all discoverable templates with metadata
  show <name>             Print the JSON profile for the named template
  resolve <name>          Print the absolute path of the named template

Templates are resolved in this order (first match wins):
  1. ./templates/<name>.json
  2. $XDG_DATA_HOME/claws/templates/<name>.json (or ~/.local/share/claws/templates/<name>.json)
  3. Bundled templates (those installed alongside the binary)

To apply a template directly:
  claws apply --template=<name>           # uses the resolver above
  claws apply --file=<path>               # explicit file path
`

// templateSearchPaths returns the directories searched for `--template=<name>`,
// in priority order. First match wins.
func templateSearchPaths() []string {
	var dirs []string

	// 1. CWD ./templates/
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "templates"))
	}

	// 2. XDG data dir — where install.sh places bundled templates
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		if home, _ := os.UserHomeDir(); home != "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome != "" {
		dirs = append(dirs, filepath.Join(dataHome, "claws", "templates"))
	}

	// 3. Next to the binary (covers extracted-tarball local installs)
	if exe, _ := os.Executable(); exe != "" {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "templates"))
	}

	return dirs
}

// resolveTemplate finds a template by name and returns its absolute path.
// Empty name → error. Unknown name → error listing the directories searched.
//
// Naming forms:
//   - "telegram-coder"        — bare name; recursive search, ambiguity errors
//   - "solo/telegram-coder"   — namespaced; resolves only that path
//   - "solo/telegram-coder.json" — extension stripped
//
// Search order across templateSearchPaths(); first match per name wins.
func resolveTemplate(name string) (string, error) {
	if name == "" {
		return "", errorf("template name required")
	}
	name = strings.TrimSuffix(name, ".json")
	if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return "", errorf("invalid template name: %q", name)
	}

	// Namespaced form: try the exact path under each search dir.
	if strings.Contains(name, "/") {
		var searched []string
		for _, dir := range templateSearchPaths() {
			searched = append(searched, dir)
			path := filepath.Join(dir, name+".json")
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		return "", errorf("template %q not found. Searched:\n  %s",
			name, strings.Join(searched, "\n  "))
	}

	// Bare name: walk each search dir recursively. Collect ALL matches so we
	// can error clearly on ambiguity. First-priority search dir wins on ties.
	var hits []string
	var searched []string
	for _, dir := range templateSearchPaths() {
		searched = append(searched, dir)
		// Direct hit at the top level (back-compat for flat layout).
		direct := filepath.Join(dir, name+".json")
		if _, err := os.Stat(direct); err == nil {
			hits = append(hits, direct)
		}
		// Recursive walk one level deep (namespace dirs).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			nested := filepath.Join(dir, e.Name(), name+".json")
			if _, err := os.Stat(nested); err == nil {
				hits = append(hits, nested)
			}
		}
		// If we found at least one in this search dir, prefer it (priority order).
		if len(hits) > 0 {
			break
		}
	}

	switch len(hits) {
	case 0:
		return "", errorf("template %q not found. Searched:\n  %s",
			name, strings.Join(searched, "\n  "))
	case 1:
		return hits[0], nil
	default:
		var qualified []string
		for _, h := range hits {
			// Display as namespace/name relative to the search dir.
			for _, dir := range templateSearchPaths() {
				if rel, err := filepath.Rel(dir, h); err == nil && !strings.HasPrefix(rel, "..") {
					qualified = append(qualified, strings.TrimSuffix(rel, ".json"))
					break
				}
			}
		}
		return "", errorf("template name %q is ambiguous. Matches:\n  %s\nUse the namespaced form (e.g. claws apply --template=<namespace>/%s)",
			name, strings.Join(qualified, "\n  "), name)
	}
}

// templateInfo summarises a discoverable template for `claws template list`.
type templateInfo struct {
	Name        string
	Namespace   string // "" if flat, e.g. "solo", "teams"
	Path        string
	Version     string
	Description string
	Tags        []string
}

// QualifiedName returns "namespace/name" or just "name" for flat templates.
func (t templateInfo) QualifiedName() string {
	if t.Namespace == "" {
		return t.Name
	}
	return t.Namespace + "/" + t.Name
}

// listTemplates walks every search path and returns a deduplicated list,
// preferring the highest-priority path on name collisions. Searches both
// flat (templates/foo.json) and namespaced (templates/<ns>/foo.json) layouts.
func listTemplates() []templateInfo {
	seen := map[string]bool{}
	var out []templateInfo

	addOne := func(path, name, namespace string) {
		// Dedup key includes namespace so solo/foo and foo aren't conflated.
		key := name
		if namespace != "" {
			key = namespace + "/" + name
		}
		if seen[key] {
			return
		}
		seen[key] = true
		info := templateInfo{Name: name, Path: path, Namespace: namespace}
		if data, err := os.ReadFile(path); err == nil {
			var p Profile
			if err := json.Unmarshal(data, &p); err == nil {
				info.Version = p.Metadata.Version
				info.Description = p.Metadata.Description
				info.Tags = p.Metadata.Tags
			}
		}
		out = append(out, info)
	}

	for _, dir := range templateSearchPaths() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				// Recurse one level for namespace dirs.
				nsEntries, err := os.ReadDir(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				for _, nse := range nsEntries {
					if nse.IsDir() || !strings.HasSuffix(nse.Name(), ".json") {
						continue
					}
					addOne(filepath.Join(dir, e.Name(), nse.Name()),
						strings.TrimSuffix(nse.Name(), ".json"), e.Name())
				}
				continue
			}
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			addOne(filepath.Join(dir, e.Name()), strings.TrimSuffix(e.Name(), ".json"), "")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func cmdTemplate(args []string) error {
	if len(args) == 0 {
		fmt.Print(templateHelp)
		return nil
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(templateHelp)
			return nil
		}
	}

	switch args[0] {
	case "list", "ls":
		templates := listTemplates()
		if len(templates) == 0 {
			fmt.Println("No templates found.")
			fmt.Println("Search paths:")
			for _, p := range templateSearchPaths() {
				fmt.Printf("  %s\n", p)
			}
			return nil
		}
		const (
			bold = "\033[1m"
			dim  = "\033[0;90m"
			nc   = "\033[0m"
		)
		fmt.Printf("%s%-36s %-10s %s%s\n", bold, "NAME", "VERSION", "DESCRIPTION", nc)
		currentNS := "__nope__"
		for _, t := range templates {
			if t.Namespace != currentNS {
				currentNS = t.Namespace
				label := currentNS
				if label == "" {
					label = "(flat)"
				}
				fmt.Printf("\n%s%s%s/\n", dim, label, nc)
			}
			desc := t.Description
			if len(desc) > 48 {
				desc = desc[:47] + "…"
			}
			ver := t.Version
			if ver == "" {
				ver = "—"
			}
			fmt.Printf("  %-34s %-10s %s\n", t.QualifiedName(), ver, desc)
		}
		fmt.Printf("\n%sApply one:%s claws apply --template=<name>\n", dim, nc)
		return nil

	case "show":
		if len(args) < 2 {
			return errorf("usage: claws template show <name>")
		}
		path, err := resolveTemplate(args[1])
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return errorf("read template: %v", err)
		}
		fmt.Print(string(data))
		if len(data) > 0 && data[len(data)-1] != '\n' {
			fmt.Println()
		}
		return nil

	case "resolve":
		if len(args) < 2 {
			return errorf("usage: claws template resolve <name>")
		}
		path, err := resolveTemplate(args[1])
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil

	default:
		return errorf("unknown subcommand %q (use list, show, or resolve)", args[0])
	}
}
