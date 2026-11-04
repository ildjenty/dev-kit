package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var hostFragments = []struct {
	source string
	target string
	merge  func([]byte, []byte) ([]byte, error)
}{
	{"hosts/claude/settings.json", ".claude/settings.json", mergeJSON},
	{"hosts/codex/hooks.json", ".codex/hooks.json", mergeJSON},
	{"hosts/codex/config.toml", ".codex/config.toml", mergeTOML},
	{"hosts/codex/rules/devkit.rules", ".codex/rules/devkit.rules", mergeLines},
}

func decodeJSON(data []byte) (map[string]any, error) {
	value := make(map[string]any)
	if len(data) == 0 {
		return value, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("settings must be an object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("settings must contain a single JSON object")
	}
	return value, nil
}

func mergeJSON(before, fragment []byte) ([]byte, error) {
	current, err := decodeJSON(before)
	if err != nil {
		return nil, err
	}
	required, err := decodeJSON(fragment)
	if err != nil {
		return nil, err
	}
	changed, err := mergeSettings(current, required, "")
	if err != nil || !changed {
		return before, err
	}
	after, err := json.MarshalIndent(current, "", "  ")
	return append(after, '\n'), err
}

func mergeTOML(before, fragment []byte) ([]byte, error) {
	current := make(map[string]any)
	if err := toml.Unmarshal(before, &current); err != nil {
		return nil, err
	}
	required := make(map[string]any)
	if err := toml.Unmarshal(fragment, &required); err != nil {
		return nil, err
	}
	changed, err := mergeSettings(current, required, "")
	if err != nil || !changed {
		return before, err
	}
	var after bytes.Buffer
	if err := toml.NewEncoder(&after).Encode(current); err != nil {
		return nil, err
	}
	return after.Bytes(), nil
}

// mergeSettings adds missing values and array members without replacing scalars.
func mergeSettings(current, required map[string]any, prefix string) (bool, error) {
	changed := false
	keys := make([]string, 0, len(required))
	for key := range required {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		want := required[key]
		have, exists := current[key]
		if !exists {
			current[key] = want
			changed = true
			continue
		}
		if containsSetting(have, want) {
			continue
		}
		switch want := want.(type) {
		case map[string]any:
			if object, ok := have.(map[string]any); ok {
				added, err := mergeSettings(object, want, prefix+key+".")
				if err != nil {
					return false, err
				}
				changed = changed || added
				continue
			}
		case []any:
			if array, ok := have.([]any); ok {
				merged, added := mergeArray(array, want)
				array = merged
				changed = changed || added
				current[key] = array
				continue
			}
		}
		return false, fmt.Errorf("existing setting %s%s conflicts with the kit fragment; resolve it manually", prefix, key)
	}
	return changed, nil
}

func containsSetting(have, want any) bool {
	switch want := want.(type) {
	case map[string]any:
		object, ok := have.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range want {
			existing, exists := object[key]
			if !exists || !containsSetting(existing, value) {
				return false
			}
		}
		return true
	case []any:
		array, ok := have.([]any)
		if !ok {
			return false
		}
		for _, value := range want {
			found := false
			for _, existing := range array {
				if reflect.DeepEqual(existing, value) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(have, want)
	}
}

func mergeLines(before, fragment []byte) ([]byte, error) {
	return appendLines(before, fragment), nil
}

func appendLines(before, fragment []byte) []byte {
	after := bytes.Clone(before)
	for _, line := range strings.Split(string(fragment), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		found := false
		for _, existing := range strings.Split(string(after), "\n") {
			if strings.TrimSpace(existing) == line {
				found = true
				break
			}
		}
		if !found {
			if len(after) != 0 && after[len(after)-1] != '\n' {
				after = append(after, '\n')
			}
			after = append(after, line...)
			after = append(after, '\n')
		}
	}
	return after
}

func mergeArray(array, required []any) ([]any, bool) {
	changed := false
	for _, item := range required {
		found := false
		for _, existing := range array {
			if reflect.DeepEqual(existing, item) {
				found = true
				break
			}
		}
		if !found {
			array = append(array, item)
			changed = true
		}
	}
	return array, changed
}
