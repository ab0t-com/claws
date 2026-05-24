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
func resolveTemplate(name string) (string, error) {
	if name == "" {
		return "", errorf("template name required")
	}
	// Strip .json suffix if user added it, then re-add.
	name = strings.TrimSuffix(name, ".json")
	if strings.ContainsAny(name, "/\\") {
		return "", errorf("template name must not contain path separators: %q", name)
	}

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

// templateInfo summarises a discoverable template for `claws template list`.
type templateInfo struct {
	Name        string
	Path        string
	Version     string
	Description string
	Tags        []string
}

// listTemplates walks every search path and returns a deduplicated list,
// preferring the highest-priority path on name collisions.
func listTemplates() []templateInfo {
	seen := map[string]bool{}
	var out []templateInfo

	for _, dir := range templateSearchPaths() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			if seen[name] {
				continue // earlier search path won
			}
			seen[name] = true

			path := filepath.Join(dir, e.Name())
			info := templateInfo{Name: name, Path: path}

			// Try to read metadata for richer output. Best-effort.
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
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
		fmt.Printf("%s%-28s %-10s %s%s\n", bold, "NAME", "VERSION", "DESCRIPTION", nc)
		for _, t := range templates {
			desc := t.Description
			if len(desc) > 56 {
				desc = desc[:55] + "…"
			}
			ver := t.Version
			if ver == "" {
				ver = "—"
			}
			fmt.Printf("%-28s %-10s %s\n", t.Name, ver, desc)
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
