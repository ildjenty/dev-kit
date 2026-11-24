package gitx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (g *Git) UpdateTree(from, to, destination string) error {
	diff, err := g.Bytes("diff", "--name-status", "-z", "--no-renames", "--no-ext-diff", from, to)
	if err != nil {
		return err
	}
	parts := strings.Split(string(diff), "\x00")
	files := []File{}
	for i := 0; i < len(parts)-1; i += 2 {
		if i+1 >= len(parts)-1 || !strings.Contains("AMDT", parts[i]) || len(parts[i]) != 1 {
			return fmt.Errorf("invalid tree diff")
		}
		files = append(files, File{Status: parts[i], Path: parts[i+1]})
	}
	sort.Slice(files, func(i, j int) bool { return len(files[i].Path) > len(files[j].Path) })
	for _, file := range files {
		if file.Status == "A" {
			continue
		}
		name := filepath.Join(destination, filepath.FromSlash(file.Path))
		if err = os.RemoveAll(name); err != nil {
			return err
		}
		for parent := filepath.Dir(name); parent != destination; parent = filepath.Dir(parent) {
			entries, e := os.ReadDir(parent)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if len(entries) != 0 {
				break
			}
			if err = os.Remove(parent); err != nil {
				return err
			}
		}
	}
	for _, file := range files {
		if file.Status == "D" {
			continue
		}
		listing, e := g.Bytes("ls-tree", "-z", to, "--", ":(literal)"+file.Path)
		if e != nil {
			return e
		}
		metadata, path, ok := strings.Cut(strings.TrimSuffix(string(listing), "\x00"), "\t")
		fields := strings.Fields(metadata)
		if !ok || path != file.Path || len(fields) != 3 {
			return fmt.Errorf("missing tree entry: %s", file.Path)
		}
		name := filepath.Join(destination, filepath.FromSlash(file.Path))
		if fields[0] == "160000" {
			if err = os.MkdirAll(name, 0755); err != nil {
				return err
			}
			continue
		}
		data, e := g.Bytes("cat-file", "blob", fields[2])
		if e != nil {
			return e
		}
		if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		if fields[0] == "120000" {
			err = os.Symlink(string(data), name)
		} else {
			mode := os.FileMode(0644)
			if fields[0] == "100755" {
				mode = 0755
			}
			err = os.WriteFile(name, data, mode)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
