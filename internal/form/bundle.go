package form

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// BundleSchema embeds referenced resources and rewrites references to local JSON pointers.
func BundleSchema(source []byte, resources map[string][]byte) ([]byte, error) {
	var document any
	if err := json.Unmarshal(source, &document); err != nil {
		return nil, err
	}
	if _, ok := document.(bool); ok {
		return source, nil
	}
	root, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema must be an object or boolean")
	}
	definitions, _ := root["$defs"].(map[string]any)
	if definitions == nil {
		definitions = map[string]any{}
		root["$defs"] = definitions
	}
	locations := map[string]string{}
	base, _ := root["$id"].(string)
	locations[base] = ""
	var rewrite func(any, string, string) error
	rewrite = func(value any, base, prefix string) error {
		switch value := value.(type) {
		case map[string]any:
			if id, ok := value["$id"].(string); ok {
				base = id
				delete(value, "$id")
			}
			if ref, ok := value["$ref"].(string); ok {
				resource, fragment, _ := strings.Cut(ref, "#")
				targetPrefix := prefix
				if resource != "" {
					if origin, err := url.Parse(base); err == nil {
						if relative, err := url.Parse(resource); err == nil {
							resource = origin.ResolveReference(relative).String()
						}
					}
					var found bool
					targetPrefix, found = locations[resource]
					if !found {
						body, ok := resources[resource]
						if !ok {
							return fmt.Errorf("schema reference resource is missing: %s", resource)
						}
						name := "devkit_resource_" + strconv.Itoa(len(locations))
						for definitions[name] != nil {
							name += "_"
						}
						targetPrefix = "/$defs/" + name
						locations[resource] = targetPrefix
						var document any
						if err := json.Unmarshal(body, &document); err != nil {
							return err
						}
						definitions[name] = document
						if err := rewrite(document, resource, targetPrefix); err != nil {
							return err
						}
					}
				}
				if targetPrefix != "" && fragment != "" && !strings.HasPrefix(fragment, "/") {
					return fmt.Errorf("schema reference must use a JSON pointer: %s", ref)
				}
				value["$ref"] = "#" + targetPrefix + fragment
			}
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				if err := rewrite(value[key], base, prefix); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := rewrite(child, base, prefix); err != nil {
					return err
				}
			}
		}
		return nil
	}
	// Original definitions share the root document's reference scope.
	originals := make([]any, 0, len(definitions))
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		originals = append(originals, definitions[key])
	}
	delete(root, "$defs")
	if err := rewrite(root, base, ""); err != nil {
		return nil, err
	}
	for _, value := range originals {
		if err := rewrite(value, base, ""); err != nil {
			return nil, err
		}
	}
	root["$defs"] = definitions
	return json.MarshalIndent(root, "", "  ")
}
