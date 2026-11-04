package kit

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/ildjenty/dev-kit/assets"
)

// Version identifies the record format and binary compatibility version.
const Version = "v3"

// FS returns the assets embedded in the binary.
func FS() fs.FS {
	return assets.FS()
}

// Norms returns distributable repository norms keyed by logical path.
func Norms() (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, root := range []string{"principles", "stages", "plans", "templates", "examples"} {
		if err := fs.WalkDir(FS(), root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := fs.ReadFile(FS(), name)
			if err != nil {
				return err
			}
			files[name] = data
			return nil
		}); err != nil {
			return nil, err
		}
	}
	rules, err := fs.ReadFile(FS(), "rules/rules.md")
	if err != nil {
		return nil, err
	}
	files["rules.md"] = rules
	return files, nil
}

// Schema returns an embedded JSON schema by its name without the .json suffix.
func Schema(name string) ([]byte, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "/") || name == "." {
		return nil, fmt.Errorf("kit: invalid schema name: %q", name)
	}
	return fs.ReadFile(FS(), "schemas/"+name+".json")
}

// RouterDefaults returns the embedded default classification and role table.
func RouterDefaults() ([]byte, error) {
	return fs.ReadFile(FS(), "router/default.toml")
}

// HostFragment returns an embedded fragment relative to hosts/.
func HostFragment(name string) ([]byte, error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, fmt.Errorf("kit: invalid host fragment name: %q", name)
	}
	return fs.ReadFile(FS(), "hosts/"+name)
}
