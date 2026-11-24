package decisions

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"gopkg.in/yaml.v3"
)

type Item = map[string]any
type Data struct {
	Decisions []Item `json:"decisions"`
}

func Citations(text string) []string {
	result := []string{}
	for _, id := range regexp.MustCompile(`\bD-[0-9a-f]{6,64}\b`).FindAllString(text, -1) {
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}
func Parse(text string) (Item, error) {
	parts := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---(?:\r?\n|$)(.*)$`).FindStringSubmatch(text)
	if parts == nil {
		return nil, fmt.Errorf("missing front matter")
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(parts[1]), &node); err != nil {
		return nil, fmt.Errorf("invalid front matter: %w", err)
	}
	result := Item{}
	if len(node.Content) > 0 {
		mapping := node.Content[0]
		if mapping.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("invalid front matter")
		}
		for i := 0; i < len(mapping.Content); i += 2 {
			key, value := mapping.Content[i], mapping.Content[i+1]
			if !regexp.MustCompile(`^[a-z_]+$`).MatchString(key.Value) {
				return nil, fmt.Errorf("invalid front matter: %s", key.Value)
			}
			if _, exists := result[key.Value]; exists {
				return nil, fmt.Errorf("invalid front matter: %s", strings.Split(parts[1], "\n")[key.Line-1])
			}
			switch value.Kind {
			case yaml.ScalarNode:
				if value.Value == "" {
					result[key.Value] = []any{}
				} else {
					result[key.Value] = value.Value
				}
			case yaml.SequenceNode:
				list := []any{}
				for _, entry := range value.Content {
					if entry.Kind != yaml.ScalarNode {
						return nil, fmt.Errorf("invalid front matter: %s", key.Value)
					}
					list = append(list, entry.Value)
				}
				result[key.Value] = list
			default:
				return nil, fmt.Errorf("invalid front matter: %s", key.Value)
			}
		}
	}
	result["body"] = strings.TrimSpace(parts[2])
	return result, nil
}
func MarkdownFiles(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		path := filepath.Join(folder, entry.Name())
		if entry.IsDir() {
			nested, err := MarkdownFiles(path)
			if err != nil {
				return nil, err
			}
			files = append(files, nested...)
		} else if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, path)
		}
	}
	return files, nil
}
func List(cwd string) (Data, error) {
	c, err := config.Load(cwd)
	if err != nil {
		return Data{}, err
	}
	read := func(folder, pattern string) ([]Item, error) {
		paths, err := MarkdownFiles(filepath.Join(cwd, folder))
		if err != nil {
			return nil, err
		}
		items := []Item{}
		for _, path := range paths {
			if !regexp.MustCompile(pattern).MatchString(filepath.Base(path)) {
				continue
			}
			bytes, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			item, err := Parse(string(bytes))
			if err != nil {
				return nil, err
			}
			relative, err := filepath.Rel(cwd, path)
			if err != nil {
				return nil, err
			}
			item["path"] = filepath.ToSlash(relative)
			items = append(items, item)
		}
		return items, nil
	}
	data := Data{}
	data.Decisions, err = read(c.Decisions, `^D-[0-9a-f]{6,64}\.md$`)
	if err != nil {
		return Data{}, err
	}
	return data, nil
}
func text(value any) string { s, _ := value.(string); return s }
func list(value any) ([]string, bool) {
	if ss, ok := value.([]string); ok {
		return ss, true
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	ss := []string{}
	valid := true
	for _, item := range items {
		s, ok := item.(string)
		valid = valid && ok
		ss = append(ss, s)
	}
	return ss, valid
}
func has(value any, id string) bool { items, _ := list(value); return slices.Contains(items, id) }
func GraphErrors(items []Item) []string {
	errors := []string{}
	byID := map[string]Item{}
	for _, item := range items {
		byID[text(item["id"])] = item
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if visiting[id] {
			errors = append(errors, "depends_on cycle at "+id)
			return
		}
		if visited[id] {
			return
		}
		visiting[id] = true
		deps, _ := list(byID[id]["depends_on"])
		for _, dep := range deps {
			if byID[dep] == nil {
				errors = append(errors, id+": missing dependency "+dep)
			} else {
				visit(dep)
			}
		}
		delete(visiting, id)
		visited[id] = true
	}
	for _, item := range items {
		visit(text(item["id"]))
	}
	return errors
}
func Check(data Data) []string {
	errors := []string{}
	known := map[string]bool{}
	for _, d := range data.Decisions {
		known[text(d["id"])] = true
	}
	seen := map[string]bool{}
	for _, d := range data.Decisions {
		id, path := text(d["id"]), text(d["path"])
		if seen[id] {
			errors = append(errors, "duplicate id "+id)
		}
		seen[id] = true
		if !regexp.MustCompile(`^D-[0-9a-f]{6,64}$`).MatchString(id) || filepath.Base(path) != id+".md" {
			errors = append(errors, path+": invalid id")
		}
		if strings.TrimSpace(text(d["title"])) == "" || strings.TrimSpace(text(d["body"])) == "" {
			errors = append(errors, path+": title and body are required")
		}
		for _, field := range []string{"depends_on", "supersedes"} {
			if _, ok := list(d[field]); !ok {
				errors = append(errors, id+": "+field+" must be a list")
			}
		}
		if !slices.Contains([]string{"proposed", "agreed", "superseded"}, text(d["status"])) {
			errors = append(errors, id+": invalid status")
		}
		if !slices.Contains([]string{"owner", "principle", "provisional"}, text(d["decided_by"])) {
			errors = append(errors, id+": invalid decided_by")
		}
		if strings.TrimSpace(text(d["request"])) == "" {
			errors = append(errors, id+": request is required")
		}
		supersedes, _ := list(d["supersedes"])
		for _, ref := range supersedes {
			if !known[ref] || ref == id {
				errors = append(errors, id+": invalid supersedes "+ref)
			}
		}
		if d["status"] == "superseded" && !slices.ContainsFunc(data.Decisions, func(other Item) bool { return other["id"] != id && has(other["supersedes"], id) }) {
			errors = append(errors, id+": superseded without a successor")
		}
		for _, ref := range Citations(text(d["body"])) {
			if !known[ref] {
				errors = append(errors, id+": missing citation "+ref)
			}
		}
	}
	return append(errors, GraphErrors(data.Decisions)...)
}
func Affected(data Data) Data {
	superseded := map[string]bool{}
	for _, d := range data.Decisions {
		if d["status"] == "superseded" {
			superseded[text(d["id"])] = true
		}
	}
	var reaches func(string, map[string]bool) bool
	reaches = func(id string, seen map[string]bool) bool {
		if superseded[id] {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		for _, d := range data.Decisions {
			if d["id"] == id {
				deps, _ := list(d["depends_on"])
				for _, dep := range deps {
					if reaches(dep, seen) {
						return true
					}
				}
				break
			}
		}
		return false
	}
	result := Data{Decisions: []Item{}}
	for _, d := range data.Decisions {
		if d["status"] == "agreed" && reaches(text(d["id"]), map[string]bool{}) {
			result.Decisions = append(result.Decisions, d)
		}
	}
	return result
}
func Dependents(id string, data Data) []string {
	found := []string{}
	pending := []string{id}
	items := data.Decisions
	for len(pending) > 0 {
		target := pending[0]
		pending = pending[1:]
		for _, item := range items {
			key := text(item["id"])
			if key != id && !slices.Contains(found, key) && (has(item["depends_on"], target) || has(item["decisions"], target)) {
				found = append(found, key)
				pending = append(pending, key)
			}
		}
	}
	return found
}
func NextID(kind, branch, key string, data Data) (string, error) {
	prefix := "D"
	items := data.Decisions
	switch kind {
	case "decision", "decisions", "D", "":
	default:
		return "", fmt.Errorf("unknown id kind %s", kind)
	}
	if branch == "" || key == "" {
		return "", fmt.Errorf("branch and draft key are required")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(branch+"\x00"+key)))
	for n := 6; n <= len(hash); n++ {
		id := prefix + "-" + hash[:n]
		if !slices.ContainsFunc(items, func(item Item) bool { return item["id"] == id }) {
			return id, nil
		}
	}
	return "", fmt.Errorf("no collision-free %s id for %s", prefix, key)
}
func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, name := range []string{"list", "check", "affected", "dependents", "next-id"} {
		usage := "devkit decisions " + name
		if name == "dependents" {
			usage += " <id>"
		}
		if name == "next-id" {
			usage += " <decision> <draft-key>"
		}
		commands = append(commands, cli.Command{Area: "decisions", Name: name, Usage: usage, Run: func(args []string, env cli.Env) error {
			if name == "dependents" && len(args) == 0 || name == "next-id" && len(args) != 2 {
				return &cli.ExitError{Code: 1, Message: "Usage: devkit decisions list | check | affected | dependents <id> | next-id <decision> <draft-key>"}
			}
			data, err := List(env.Cwd)
			if err != nil {
				return err
			}
			var output any
			switch name {
			case "list":
				output = data
			case "check":
				problems := Check(data)
				if err := env.JSON(problems); err != nil {
					return err
				}
				if len(problems) > 0 {
					return &cli.ExitError{Code: 1}
				}
				return nil
			case "affected":
				output = Affected(data)
			case "dependents":
				output = Dependents(args[0], data)
			case "next-id":
				kind := ""
				if len(args) > 0 {
					kind = args[0]
				}
				branch, e := gitx.Run(env.Cwd, "rev-parse", "--abbrev-ref", "HEAD")
				if e != nil {
					return e
				}
				output, err = NextID(kind, branch, args[1], data)
				if err != nil {
					return err
				}
			}
			return env.JSON(output)
		}})
	}
	commands = append(commands, cli.Command{Area: "decisions", Usage: "devkit decisions list | check | affected | dependents <id> | next-id <decision> <draft-key>", Run: func(args []string, env cli.Env) error {
		return &cli.ExitError{Code: 1, Message: "Usage: devkit decisions list | check | affected | dependents <id> | next-id <decision> <draft-key>"}
	}})
	return commands
}
