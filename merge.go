package main

import (
	"encoding/json"
	"os"
)

// deepMerge merges override into base. Override wins on conflicts.
// Both must be map[string]any. Modifies base in place.
func deepMerge(base, override map[string]any) map[string]any {
	for k, v := range override {
		if baseVal, ok := base[k]; ok {
			baseMap, baseIsMap := baseVal.(map[string]any)
			overMap, overIsMap := v.(map[string]any)
			if baseIsMap && overIsMap {
				deepMerge(baseMap, overMap)
				continue
			}
		}
		base[k] = v
	}
	return base
}

// mergeConfigLayers builds openclaw.json with 4 layers:
// global defaults → group defaults → template → gateway skeleton.
func mergeConfigLayers(defaultsPath, groupDefaultsPath, templatePath, templateFlag, skeletonPath, outputPath string) error {
	result := make(map[string]any)

	// Layer 1: global defaults
	if data, err := os.ReadFile(defaultsPath); err == nil {
		var defaults map[string]any
		if json.Unmarshal(data, &defaults) == nil {
			deepMerge(result, defaults)
		}
	}

	// Layer 2: group defaults
	if groupDefaultsPath != "" {
		if data, err := os.ReadFile(groupDefaultsPath); err == nil {
			var groupDefaults map[string]any
			if json.Unmarshal(data, &groupDefaults) == nil {
				deepMerge(result, groupDefaults)
			}
		}
	}

	// Layer 3: template instance config (--from)
	if templateFlag != "" && templatePath != "" {
		if data, err := os.ReadFile(templatePath); err == nil {
			var tmpl map[string]any
			if json.Unmarshal(data, &tmpl) == nil {
				delete(tmpl, "gateway")
				if channels, ok := tmpl["channels"].(map[string]any); ok {
					for _, ch := range channels {
						if chMap, ok := ch.(map[string]any); ok {
							delete(chMap, "groups")
							delete(chMap, "allowFrom")
							delete(chMap, "groupAllowFrom")
						}
					}
				}
				deepMerge(result, tmpl)
			}
		}
	}

	// Layer 4: gateway skeleton (always wins)
	if data, err := os.ReadFile(skeletonPath); err == nil {
		var skeleton map[string]any
		if json.Unmarshal(data, &skeleton) == nil {
			deepMerge(result, skeleton)
		}
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(out, '\n'), 0600)
}

