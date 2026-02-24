package envx

import (
	"sort"
	"strings"
)

func FromMap(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}

	return env
}

func Merge(base []string, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}

	overrideKeys := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			overrideKeys[key] = struct{}{}
		}
	}

	merged := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, exists := overrideKeys[key]; exists {
				continue
			}
		}
		merged = append(merged, entry)
	}

	merged = append(merged, overrides...)
	return merged
}

func MergeMap(base []string, overrides map[string]string) []string {
	return Merge(base, FromMap(overrides))
}
