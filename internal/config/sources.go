package config

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Source connects a named knowledge source to its files.
type Source struct {
	Adapter  string `toml:"adapter"`
	Path     string `toml:"path"`
	Norm     bool   `toml:"norm"`
	Writable *bool  `toml:"writable"`
}

// CanWrite reports the files adapter's effective write capability.
func (s Source) CanWrite() bool {
	return s.Writable == nil && !path.IsAbs(s.Path) || s.Writable != nil && *s.Writable
}

// Selection names a source and an adapter-specific selector.
type Selection struct {
	Source string `toml:"source" json:"source"`
	Select string `toml:"select" json:"select"`
}

var sourceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// KnowledgeSources returns effective sources without sharing mutable settings.
func (c Config) KnowledgeSources() map[string]Source {
	sources := make(map[string]Source)
	for _, name := range []string{"principles", "stages", "plans", "templates", "rules", "git", "authority", "requirements", "structure"} {
		location := ".devkit/" + name
		switch name {
		case "rules", "git", "authority", "requirements":
			location += ".md"
		case "structure":
			location += ".json"
		}
		sources[name] = Source{Adapter: "files", Path: location, Norm: name != "requirements" && name != "structure"}
	}
	for name, source := range c.Sources {
		if source.Adapter == "" {
			source.Adapter = "files"
		}
		sources[name] = source
	}
	for name, source := range sources {
		writable := source.CanWrite()
		source.Writable = &writable
		sources[name] = source
	}
	return sources
}

func (c *Config) applySourceDefaults(metadata toml.MetaData) {
	defaults := (Config{}).KnowledgeSources()
	for name, source := range c.Sources {
		fallback := defaults[name]
		if !metadata.IsDefined("sources", name, "adapter") {
			source.Adapter = "files"
		}
		if !metadata.IsDefined("sources", name, "path") {
			source.Path = fallback.Path
		}
		if !metadata.IsDefined("sources", name, "norm") {
			source.Norm = fallback.Norm
		}
		defaults[name] = source
	}
	c.Sources = defaults
	for name, source := range c.Sources {
		writable := source.CanWrite()
		source.Writable = &writable
		c.Sources[name] = source
	}
}

// ValidateSelection checks that the named source is configured.
// The adapter, rather than config, interprets Select.
func (c Config) ValidateSelection(selection Selection) error {
	if !sourceName.MatchString(selection.Source) {
		return fmt.Errorf("invalid source name: %q", selection.Source)
	}
	if _, ok := c.KnowledgeSources()[selection.Source]; !ok {
		return fmt.Errorf("unknown source: %q", selection.Source)
	}
	return nil
}

// ValidateCriteria checks that every selection names a norm source.
// The caller checks that required selections actually return documents.
func (c Config) ValidateCriteria(criteria []Selection) error {
	sources := c.KnowledgeSources()
	for _, selection := range criteria {
		if err := c.ValidateSelection(selection); err != nil {
			return err
		}
		if !sources[selection.Source].Norm {
			return fmt.Errorf("criteria source %q must be a norm source", selection.Source)
		}
	}
	return nil
}

// ValidateImprovementDestination rejects unknown or read-only improvement sources.
func (c Config) ValidateImprovementDestination(name string) error {
	if err := c.ValidateSelection(Selection{Source: name}); err != nil {
		return err
	}
	if !c.KnowledgeSources()[name].CanWrite() {
		return fmt.Errorf("improvement destination source %q must be writable", name)
	}
	return nil
}

func (c Config) validateSources() error {
	names := make([]string, 0, len(c.Sources))
	for name := range c.Sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := validateSource(name, c.Sources[name]); err != nil {
			return err
		}
	}
	if err := c.validateSourceLocations(); err != nil {
		return err
	}
	return nil
}

func validateSource(name string, source Source) error {
	if !sourceName.MatchString(name) {
		return fmt.Errorf("invalid source name: %q", name)
	}
	switch source.Adapter {
	case "files":
	case "command":
		return fmt.Errorf("sources.%s.adapter: command is reserved for future support; use files", name)
	default:
		return fmt.Errorf("sources.%s.adapter must be files: %q", name, source.Adapter)
	}
	if !validRepositoryPath(source.Path) && !(path.IsAbs(source.Path) && source.Path != "/" && path.Clean(source.Path) == source.Path && !strings.ContainsAny(source.Path, "\\\x00")) {
		return fmt.Errorf("sources.%s.path must be a clean slash path outside .devkit/run: %q", name, source.Path)
	}
	return nil
}

func pathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// ValidateSourcePaths checks overlaps after resolving paths against the repository root.
// Callers with a known root use this to catch absolute aliases of relative paths.
func (c Config) ValidateSourcePaths(repositoryRoot string) error {
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return err
	}
	resolved := c
	resolved.Sources = c.KnowledgeSources()
	for name, source := range resolved.Sources {
		if !filepath.IsAbs(source.Path) {
			source.Path = filepath.Join(root, filepath.FromSlash(source.Path))
		}
		source.Path = filepath.ToSlash(source.Path)
		if !strings.HasPrefix(source.Path, filepath.ToSlash(root)+"/") && source.CanWrite() {
			return fmt.Errorf("sources.%s.writable must be false: sources outside the repository are read-only", name)
		}
		if pathsOverlap(source.Path, filepath.ToSlash(filepath.Join(root, ".devkit/run"))) {
			return fmt.Errorf("sources.%s.path must be outside .devkit/run", name)
		}
		resolved.Sources[name] = source
	}
	return resolved.validateSourceLocations()
}

func (c Config) validateSourceLocations() error {
	names := make([]string, 0, len(c.Sources))
	for name := range c.Sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for i, name := range names {
		source := c.Sources[name]
		for _, other := range names[:i] {
			if pathsOverlap(source.Path, c.Sources[other].Path) {
				return fmt.Errorf("sources.%s.path and sources.%s.path must not overlap", name, other)
			}
		}
	}
	return nil
}
