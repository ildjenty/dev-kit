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
	base, _ := root["$id"].(string)
	bundle := schemaBundle{resources: resources, definitions: definitions, locations: map[string]string{base: ""}}
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
	if err := bundle.rewrite(root, base, ""); err != nil {
		return nil, err
	}
	for _, value := range originals {
		if err := bundle.rewrite(value, base, ""); err != nil {
			return nil, err
		}
	}
	root["$defs"] = definitions
	return json.MarshalIndent(root, "", "  ")
}

type schemaBundle struct {
	resources   map[string][]byte
	definitions map[string]any
	locations   map[string]string
}

func (b schemaBundle) rewrite(value any, base, prefix string) error {
	switch value := value.(type) {
	case map[string]any:
		return b.rewriteObject(value, base, prefix)
	case []any:
		for _, child := range value {
			if err := b.rewrite(child, base, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b schemaBundle) rewriteObject(value map[string]any, base, prefix string) error {
	if id, ok := value["$id"].(string); ok {
		base = id
		delete(value, "$id")
	}
	if ref, ok := value["$ref"].(string); ok {
		pointer, err := b.referencePointer(ref, base, prefix)
		if err != nil {
			return err
		}
		value["$ref"] = pointer
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if err := b.rewrite(value[key], base, prefix); err != nil {
			return err
		}
	}
	return nil
}

func (b schemaBundle) referencePointer(ref, base, prefix string) (string, error) {
	resource, fragment, _ := strings.Cut(ref, "#")
	targetPrefix := prefix
	if resource != "" {
		if origin, err := url.Parse(base); err == nil {
			if relative, err := url.Parse(resource); err == nil {
				resource = origin.ResolveReference(relative).String()
			}
		}
		var err error
		targetPrefix, err = b.embedResource(resource)
		if err != nil {
			return "", err
		}
	}
	if targetPrefix != "" && fragment != "" && !strings.HasPrefix(fragment, "/") {
		return "", fmt.Errorf("schema reference must use a JSON pointer: %s", ref)
	}
	return "#" + targetPrefix + fragment, nil
}

func (b schemaBundle) embedResource(resource string) (string, error) {
	if prefix, found := b.locations[resource]; found {
		return prefix, nil
	}
	body, ok := b.resources[resource]
	if !ok {
		return "", fmt.Errorf("schema reference resource is missing: %s", resource)
	}
	name := "devkit_resource_" + strconv.Itoa(len(b.locations))
	for b.definitions[name] != nil {
		name += "_"
	}
	prefix := "/$defs/" + name
	b.locations[resource] = prefix
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return "", err
	}
	b.definitions[name] = document
	if err := b.rewrite(document, resource, prefix); err != nil {
		return "", err
	}
	return prefix, nil
}
