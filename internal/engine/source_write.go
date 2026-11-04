package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
)

// externalNormChanges lists documents of norm sources outside the repository whose fixed
// copies differ between two references.
func (e *Engine) externalNormChanges(ctx context.Context, before, after string) ([]string, error) {
	if e.workRoot() == "" {
		return nil, nil
	}
	external, err := e.externalSources(ctx, after)
	if err != nil || len(external) == 0 {
		return nil, err
	}
	old, err := e.pin(ctx, before, false)
	if err != nil {
		return nil, err
	}
	current, err := e.pin(ctx, after, false)
	if err != nil {
		return nil, err
	}
	changed := []string{}
	for name, item := range external {
		if !item.Norm {
			continue
		}
		for _, id := range differentDocuments(old[name], current[name]) {
			changed = append(changed, name+"/"+id)
		}
	}
	slices.Sort(changed)
	return changed, nil
}

func differentDocuments(before, after sourceManifest) []string {
	ids := []string{}
	for id, hash := range after {
		if before[id] != hash {
			ids = append(ids, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// sourceChangeText lists changed outside documents for a loop-edit question.
func (e *Engine) sourceChangeText(ctx context.Context, before, after string) (string, error) {
	live, err := e.externalNormChanges(ctx, before, after)
	if err != nil {
		return "", err
	}
	if len(live) == 0 {
		return "", nil
	}
	var text strings.Builder
	text.WriteString("\nChanged documents of norm sources outside the repository:\n")
	for _, id := range live {
		fmt.Fprintf(&text, "- %s (changed in the source since the confirmed norms were fixed)\n", id)
	}
	return text.String(), nil
}

// readOnlyChanges lists changed files inside read-only sources of the repository. Improvements
// to read-only sources are recorded as lessons and handed over to their maintainers instead.
func (e *Engine) readOnlyChanges(settings config.Config, paths []string) []string {
	blocked := []string{}
	for name, item := range settings.KnowledgeSources() {
		location, inside := e.repositoryLocation(item)
		if !inside || settings.ValidateImprovementDestination(name) == nil {
			continue
		}
		for _, changed := range paths {
			if changed == location || strings.HasPrefix(changed, location+"/") {
				blocked = append(blocked, changed+" (source "+name+")")
			}
		}
	}
	slices.Sort(blocked)
	return blocked
}
