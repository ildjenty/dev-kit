package initialize

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/doctor"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

const usage = "devkit init [--yes | --no] [--dry-run] [--provider <claude|codex>] [--sessions <claude,codex>] [--language <code>] [--skill <devkit-runs|devkit-discuss|devkit-optimize>=<name>]... [dir]"

type options struct {
	yes, no, dry bool
	dir          string
	version      string
	skills       map[string]string
	provider     string
	sessions     []string
	language     string
}

type tree struct {
	data     []byte
	children map[string]*tree
}

type change struct {
	path    string
	data    []byte
	remove  bool
	exists  bool
	allowed bool
}

type group struct {
	path    string
	warning string
	whole   bool
	changes []*change
	links   []string
	replace int
	deleted int
	ask     bool
	answer  bool
}

func Commands() []cli.Command {
	return []cli.Command{{Area: "init", Usage: usage, Run: func(args []string, env cli.Env) error {
		return run(args, env, terminal(env.Stdin))
	}}}
}

func terminal(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	cmd := exec.Command("sh", "-c", "test -t 0")
	cmd.Stdin = file
	return cmd.Run() == nil
}

func arguments(args []string) (options, error) {
	o := options{skills: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--yes":
			o.yes = true
		case arg == "--no":
			o.no = true
		case arg == "--dry-run":
			o.dry = true
		case arg == "--provider" || strings.HasPrefix(arg, "--provider=") || arg == "--sessions" || strings.HasPrefix(arg, "--sessions=") || arg == "--language" || strings.HasPrefix(arg, "--language="):
			key, value, hasValue := strings.Cut(arg, "=")
			if !hasValue {
				i++
				if i == len(args) {
					return o, cli.Usage(usage)
				}
				value = args[i]
			}
			if key == "--provider" {
				o.provider = value
				if value != "claude" && value != "codex" {
					return o, cli.Usage(usage)
				}
			} else if key == "--language" {
				data, _ := json.Marshal(map[string]string{"language": value})
				if _, err := config.Parse(data); err != nil {
					return o, err
				}
				o.language = value
			} else {
				o.sessions = strings.Split(value, ",")
			}
		case arg == "--skill" || strings.HasPrefix(arg, "--skill="):
			value := strings.TrimPrefix(arg, "--skill=")
			if arg == "--skill" {
				i++
				if i == len(args) {
					return o, cli.Usage(usage)
				}
				value = args[i]
			}
			key, name, ok := strings.Cut(value, "=")
			if !ok || !slices.Contains(config.SkillKeys, key) {
				return o, cli.Usage(usage)
			}
			o.skills[key] = name
		case strings.HasPrefix(arg, "-") || o.dir != "":
			return o, cli.Usage(usage)
		default:
			o.dir = arg
		}
	}
	if o.yes && o.no {
		return o, cli.Usage(usage)
	}
	return o, nil
}

func run(args []string, env cli.Env, interactive bool) error {
	o, err := arguments(args)
	if err != nil {
		return err
	}
	o.version = env.Registry.Version()
	root := filepath.Clean(env.Path(o.dir))
	language, source := o.language, "--language"
	if language == "" {
		language, source = localeLanguage(env)
	}
	groups, original, skillsOnly, err := compare(root, o, language)
	if err != nil {
		return err
	}
	var output strings.Builder
	if original == nil || o.language != "" {
		fmt.Fprintf(&output, "Language: %s (from %s). devkit-optimize can fit it to this repository.\n", language, source)
	}
	links := []string{}
	questions := false
	for _, g := range groups {
		if g.warning != "" {
			fmt.Fprintln(&output, g.warning)
		}
		for _, c := range g.changes {
			verb := "Missing"
			if c.remove {
				verb = "Extra"
			} else if c.exists {
				verb = "Different"
			}
			fmt.Fprintf(&output, "%s: %s\n", verb, c.path)
		}
		links = append(links, g.links...)
		questions = questions || g.ask
	}
	sort.Strings(links)
	links = slices.Compact(links)
	for _, path := range links {
		fmt.Fprintf(&output, "Warning: symbolic link: %s\n", path)
	}
	if (questions || len(links) > 0) && !interactive && !o.yes && !o.no {
		output.WriteString("Stdin is not a terminal; all questions default to No.\n")
	}
	if _, err := fmt.Fprint(env.Stdout, output.String()); err != nil {
		return err
	}
	scanner := bufio.NewScanner(env.Stdin)
	ask := func(question string) (bool, error) {
		if _, err := fmt.Fprint(env.Stdout, question+" [y/N] "); err != nil {
			return false, err
		}
		if o.dry {
			_, err := fmt.Fprintln(env.Stdout, "(dry-run)")
			return false, err
		}
		if o.yes || o.no || !interactive {
			answer := "No"
			if o.yes {
				answer = "Yes"
			}
			_, err := fmt.Fprintln(env.Stdout, answer)
			return o.yes, err
		}
		if !scanner.Scan() {
			return false, scanner.Err()
		}
		answer := strings.TrimSpace(scanner.Text())
		return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
	}
	linkAnswers := map[string]bool{}
	for _, path := range links {
		answer, err := ask("Replace symbolic link " + path + " itself (leave its target untouched)?")
		if err != nil {
			return err
		}
		linkAnswers[path] = answer
	}
	for _, g := range groups {
		g.answer = true
		if g.ask {
			question := fmt.Sprintf("%s: replace %d files and delete %d files?", g.path, g.replace, g.deleted)
			if g.whole {
				question = g.path + ": replace it with the kit's standards?"
			} else if g.path == ".devkit/config.json" || g.path == ".archon/config.yaml" {
				question = g.path + ": overwrite this file?"
			}
			g.answer, err = ask(question)
			if err != nil {
				return err
			}
		}
		for _, path := range g.links {
			if g.whole && !linkAnswers[path] {
				g.answer = false
			}
		}
		if !g.answer && !(g.path == ".devkit/config.json" && o.explicit()) {
			for _, path := range g.links {
				if within(g.path, path) {
					linkAnswers[path] = false
				}
			}
		}
	}
	for _, g := range groups {
		if g.path == ".devkit/config.json" && !g.answer && o.explicit() && skillsOnly == nil && !o.dry {
			return fmt.Errorf("cannot update only skills/provider/sessions/language in invalid .devkit/config.json; approve replacing the config to repair it")
		}
		partial := g.path == ".devkit/config.json" && !g.answer && o.explicit() && !bytes.Equal(original, skillsOnly)
		for _, c := range g.changes {
			c.allowed = g.answer || !g.whole && !c.exists && !c.remove
			if c.path == kitinfo.StampPath {
				c.allowed = g.answer
			}
			if partial {
				c.allowed = true
				if !c.remove {
					c.data = skillsOnly
				}
			}
			for _, path := range g.links {
				if within(c.path, path) && !linkAnswers[path] {
					c.allowed = false
				}
			}
		}
		for _, c := range g.changes {
			if !c.remove || c.allowed {
				continue
			}
			for _, other := range g.changes {
				if within(other.path, c.path) {
					other.allowed = false
				}
			}
		}
		for _, c := range g.changes {
			if c.remove && !c.allowed {
				for _, parent := range g.changes {
					if parent.remove && within(c.path, parent.path) {
						parent.allowed = false
						for _, write := range g.changes {
							if !write.remove && write.path == parent.path {
								write.allowed = false
							}
						}
					}
				}
			}
		}
	}
	for _, g := range groups {
		complete := true
		for _, c := range g.changes {
			if !c.allowed {
				complete = false
			}
		}
		if !complete {
			for _, c := range g.changes {
				if c.path == kitinfo.StampPath {
					c.allowed = false
				}
			}
		}
	}
	if o.dry {
		_, err := fmt.Fprintln(env.Stdout, "Dry run: nothing written or deleted.")
		return err
	}
	var summary strings.Builder
	removed := map[string]bool{}
	for _, g := range groups {
		for _, c := range g.changes {
			if !c.allowed {
				fmt.Fprintf(&summary, "Left alone: %s\n", c.path)
				continue
			}
			dest := filepath.Join(root, filepath.FromSlash(c.path))
			if c.remove {
				if !removed[c.path] {
					if err := os.Remove(dest); err != nil {
						return err
					}
					removed[c.path] = true
					fmt.Fprintf(&summary, "Deleted: %s\n", c.path)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(dest, c.data, 0644); err != nil {
					return err
				}
				fmt.Fprintf(&summary, "Wrote: %s\n", c.path)
			}
		}
	}
	if _, err = fmt.Fprint(env.Stdout, summary.String()); err != nil {
		return err
	}
	doctor.Report(cli.Env{Cwd: root, Stdout: env.Stdout, Stderr: env.Stderr, LookupEnv: env.LookupEnv, Environ: env.Environ})
	if original != nil {
		return nil
	}
	written, err := os.ReadFile(filepath.Join(root, ".devkit/config.json"))
	if err != nil {
		return nil
	}
	settings, err := config.Parse(written)
	if err != nil {
		return nil
	}
	_, err = fmt.Fprintf(env.Stdout, "\nNext: in a session in this repository, run the %s skill. It fits the settings, checks and standards to this repository; until then no checks are configured.\n", settings.Skills["devkit-optimize"])
	return err
}

func within(path, folder string) bool {
	return folder == "." || path == folder || strings.HasPrefix(path, folder+"/")
}

func localeLanguage(env cli.Env) (string, string) {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := env.Getenv(key)
		if value == "" {
			continue
		}
		if end := strings.IndexAny(value, "_.@"); end >= 0 {
			value = value[:end]
		}
		value = strings.ToLower(value)
		if value == "" || value == "c" || value == "posix" {
			value = "en"
		}
		return value, key
	}
	return "en", "default"
}

func (o options) explicit() bool {
	return len(o.skills) > 0 || o.provider != "" || o.sessions != nil || o.language != ""
}

func compare(root string, o options, language string) ([]*group, []byte, []byte, error) {
	original, err := os.ReadFile(filepath.Join(root, ".devkit/config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, nil, err
	}
	settings := config.Defaults("")
	var stored map[string]json.RawMessage
	validObject := false
	warning := ""
	if err == nil {
		decodeErr := json.Unmarshal(original, &stored)
		validObject = decodeErr == nil && stored != nil
		if validObject {
			destinations := map[string]json.RawMessage{}
			for _, key := range []string{"standards", "skills", "provider", "sessions", "language"} {
				if value, ok := stored[key]; ok {
					destinations[key] = value
				}
			}
			data, err := json.Marshal(destinations)
			if err != nil {
				return nil, nil, nil, err
			}
			settings, err = config.Parse(data)
			if err != nil {
				return nil, nil, nil, err
			}
		} else {
			warning = "Warning: .devkit/config.json is not a valid JSON object; using default destinations for comparison."
		}
	}
	if original == nil || o.language != "" {
		settings.Language = language
	}
	old := map[string]string{}
	for key, name := range settings.Skills {
		old[key] = name
	}
	oldSessions := append([]string{}, settings.Sessions...)
	for key, name := range o.skills {
		settings.Skills[key] = name
	}
	if o.provider != "" {
		settings.Provider = o.provider
	}
	if o.sessions != nil {
		settings.Sessions = o.sessions
	} else if _, exists := stored["sessions"]; !exists && o.provider != "" {
		settings.Sessions = []string{o.provider}
	}
	validation, err := json.Marshal(settings)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := config.Parse(validation); err != nil {
		return nil, nil, nil, err
	}
	pairs := []string{}
	paths := []string{".archon/workflows/dev"}
	for _, key := range config.SkillKeys {
		pairs = append(pairs, key, settings.Skills[key])
	}
	for _, session := range settings.Sessions {
		for _, key := range config.SkillKeys {
			paths = append(paths, skillFolder(session)+settings.Skills[key])
		}
	}
	replaceNames := strings.NewReplacer(pairs...)
	paths = append(paths, settings.Standards, ".devkit/config.json", ".archon/config.yaml")
	wanted := map[string]*tree{}
	for _, path := range paths {
		wanted[path] = &tree{children: map[string]*tree{}}
	}
	for _, session := range oldSessions {
		for _, key := range config.SkillKeys {
			path := skillFolder(session) + old[key]
			if !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	put := func(dest string, data []byte) error {
		for _, folder := range paths {
			if dest == folder {
				wanted[folder] = &tree{data: data}
				return nil
			}
			if strings.HasPrefix(dest, folder+"/") && wanted[folder] != nil {
				node := wanted[folder]
				for _, part := range strings.Split(strings.TrimPrefix(dest, folder+"/"), "/") {
					if node.children == nil {
						node.children = map[string]*tree{}
					}
					if node.children[part] == nil {
						node.children[part] = &tree{}
					}
					node = node.children[part]
				}
				node.data = data
				return nil
			}
		}
		return fmt.Errorf("unhandled kit file: %s", dest)
	}
	err = fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		dest := path
		switch {
		case strings.HasPrefix(path, ".devkit/standards/"):
			dest = settings.Standards + strings.TrimPrefix(path, ".devkit/standards")
		case path == ".devkit/config.json":
			data = []byte(strings.ReplaceAll(string(data), ".devkit/standards", settings.Standards))
			for _, key := range []string{"skills", "provider", "sessions", "language"} {
				var value any
				switch key {
				case "skills":
					value = settings.Skills
				case "provider":
					value = settings.Provider
				case "sessions":
					value = settings.Sessions
				case "language":
					value = settings.Language
				}
				encoded, _ := json.Marshal(value)
				data, err = withSetting(data, key, encoded)
				if err != nil {
					return err
				}
			}
		case strings.HasPrefix(path, ".claude/skills/"):
			for _, session := range settings.Sessions {
				dest = strings.Replace(replaceNames.Replace(path), ".claude/skills/", skillFolder(session), 1)
				installed := strings.ReplaceAll(replaceNames.Replace(string(data)), ".claude/skills/", skillFolder(session))
				if err := put(dest, []byte(installed)); err != nil {
					return err
				}
			}
			return nil
		case strings.HasPrefix(path, ".archon/workflows/dev/"):
			data = []byte(replaceNames.Replace(string(data)))
			if settings.Provider == "codex" && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) && !strings.Contains(path, "/fixtures/") {
				data, err = codexWorkflow(data)
				if err != nil {
					return err
				}
			}
		}
		return put(dest, data)
	})
	if err != nil {
		return nil, nil, nil, err
	}
	stamp, err := json.MarshalIndent(kitinfo.Stamp{DevkitVersion: o.version, KitDigest: kitinfo.EmbeddedDigest()}, "", "  ")
	if err != nil {
		return nil, nil, nil, err
	}
	if err := put(kitinfo.StampPath, append(stamp, '\n')); err != nil {
		return nil, nil, nil, err
	}
	skillsOnly := original
	if original != nil && o.explicit() && validObject {
		for _, key := range []string{"skills", "provider", "sessions", "language"} {
			var value any
			switch key {
			case "skills":
				if len(o.skills) == 0 {
					continue
				}
				value = settings.Skills
			case "provider":
				if o.provider == "" {
					continue
				}
				value = settings.Provider
			case "sessions":
				if o.sessions == nil && (o.provider == "" || stored["sessions"] != nil) {
					continue
				}
				value = settings.Sessions
			case "language":
				if o.language == "" {
					continue
				}
				value = settings.Language
			}
			encoded, _ := json.Marshal(value)
			skillsOnly, err = withSetting(skillsOnly, key, encoded)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	} else if original != nil && o.explicit() {
		skillsOnly = nil
	}
	groups := []*group{}
	for _, path := range paths {
		g := &group{path: path, whole: path == settings.Standards}
		if path == ".devkit/config.json" {
			g.warning = warning
		}
		if wanted[path] == nil && (strings.HasPrefix(path, ".claude/skills/") || strings.HasPrefix(path, ".agents/skills/")) {
			ours, err := installedByKit(root, path, old)
			if err != nil {
				return nil, nil, nil, err
			}
			if !ours {
				if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
					g.warning = "Left alone: " + path + " holds files devkit did not install, so it is not offered for deletion."
					groups = append(groups, g)
				}
				continue
			}
		}
		ancestor, err := linkAncestor(root, path)
		if err != nil {
			return nil, nil, nil, err
		}
		if ancestor != "" {
			if wanted[path] == nil {
				continue
			}
			g.links = append(g.links, ancestor)
			g.changes = append(g.changes, &change{path: ancestor, remove: true, exists: true})
			g.ask = true
			g.replace++
			g.add(path, wanted[path])
		} else if err := g.diff(root, path, wanted[path]); err != nil {
			return nil, nil, nil, err
		}
		slices.SortStableFunc(g.changes, func(a, b *change) int {
			if a.path == kitinfo.StampPath && b.path != kitinfo.StampPath {
				return 1
			}
			if a.path != kitinfo.StampPath && b.path == kitinfo.StampPath {
				return -1
			}
			return 0
		})
		if len(g.changes) > 0 {
			groups = append(groups, g)
		}
	}
	return groups, original, skillsOnly, nil
}

func installedByKit(root, folder string, names map[string]string) (bool, error) {
	pairs := []string{}
	key := ""
	for _, k := range config.SkillKeys {
		pairs = append(pairs, k, names[k])
		if filepath.Base(folder) == names[k] {
			key = k
		}
	}
	if key == "" {
		return false, nil
	}
	pairs = append(pairs, ".claude/skills/", filepath.ToSlash(filepath.Dir(folder))+"/")
	rename := strings.NewReplacer(pairs...)
	found := false
	ours := true
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(folder)), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		found = true
		if !entry.Type().IsRegular() {
			ours = false
			return fs.SkipAll
		}
		relative, err := filepath.Rel(filepath.Join(root, filepath.FromSlash(folder)), path)
		if err != nil {
			return err
		}
		shipped, err := kit.Files.ReadFile(".claude/skills/" + key + "/" + filepath.ToSlash(relative))
		if err != nil {
			ours = false
			return fs.SkipAll
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, []byte(rename.Replace(string(shipped)))) {
			ours = false
			return fs.SkipAll
		}
		return nil
	})
	return found && ours, err
}

func withSetting(data []byte, setting string, skills []byte) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	var current any
	if json.Unmarshal(object[setting], &current) == nil {
		encoded, err := json.Marshal(current)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(encoded, skills) {
			return data, nil
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		start := int(decoder.InputOffset())
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if key == setting {
			start += bytes.IndexByte(data[start:], ':') + 1
			for bytes.ContainsRune([]byte(" \t\r\n"), rune(data[start])) {
				start++
			}
			result := append([]byte{}, data[:start]...)
			result = append(result, skills...)
			return append(result, data[decoder.InputOffset():]...), nil
		}
	}
	end := bytes.LastIndexByte(data, '}')
	result := append([]byte{}, data[:end]...)
	if len(object) > 0 {
		result = append(result, ',')
	}
	result = append(result, []byte("\n  \""+setting+"\": ")...)
	result = append(result, skills...)
	result = append(result, '\n')
	return append(result, data[end:]...), nil
}

func linkAncestor(root, path string) (string, error) {
	parts := append([]string{"."}, strings.Split(filepath.ToSlash(filepath.Dir(path)), "/")...)
	current := ""
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(filepath.Join(root, current))
		if os.IsNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.ToSlash(current), nil
		}
		if !info.IsDir() {
			return "", fmt.Errorf("destination parent is not a directory: %s", current)
		}
	}
	return "", nil
}

func (g *group) add(path string, wanted *tree) {
	if wanted == nil {
		return
	}
	if wanted.children == nil {
		g.changes = append(g.changes, &change{path: path, data: wanted.data})
		return
	}
	for _, name := range childNames(wanted) {
		g.add(path+"/"+name, wanted.children[name])
	}
}

func childNames(node *tree) []string {
	names := []string{}
	if node != nil {
		for name := range node.children {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (g *group) diff(root, path string, wanted *tree) error {
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		g.add(path, wanted)
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		g.links = append(g.links, path)
		g.changes = append(g.changes, &change{path: path, remove: true, exists: true})
		g.ask = true
		if wanted == nil {
			g.deleted++
		} else {
			g.replace++
		}
		g.add(path, wanted)
		return nil
	}
	if info.IsDir() {
		if wanted != nil && wanted.children == nil && (g.path == ".devkit/config.json" || g.path == ".archon/config.yaml") {
			return fmt.Errorf("expected a file: %s", path)
		}
		entries, err := os.ReadDir(filepath.Join(root, path))
		if err != nil {
			return err
		}
		names := childNames(wanted)
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		names = slices.Compact(names)
		for _, name := range names {
			var child *tree
			if wanted != nil {
				child = wanted.children[name]
			}
			if err := g.diff(root, path+"/"+name, child); err != nil {
				return err
			}
		}
		if wanted == nil || wanted.children == nil {
			g.changes = append(g.changes, &change{path: path, remove: true, exists: true})
			g.ask = true
			if wanted != nil {
				g.replace++
				g.changes = append(g.changes, &change{path: path, data: wanted.data, exists: true})
			}
		}
	} else if wanted == nil || wanted.children != nil {
		g.changes = append(g.changes, &change{path: path, remove: true, exists: true})
		g.ask = true
		if wanted == nil {
			g.deleted++
		} else {
			g.replace++
			g.add(path, wanted)
		}
	} else {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("destination is not a regular file: %s", path)
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if !bytes.Equal(data, wanted.data) {
			g.changes = append(g.changes, &change{path: path, data: wanted.data, exists: true})
			if path != kitinfo.StampPath {
				g.replace++
				g.ask = true
			}
		}
	}
	if g.whole && len(g.changes) > 0 {
		g.ask = true
	}
	return nil
}

func skillFolder(session string) string {
	if session == "codex" {
		return ".agents/skills/"
	}
	return ".claude/skills/"
}

func codexWorkflow(data []byte) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node.Kind == yaml.MappingNode {
			ai := false
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == "provider" {
					ai = true
					node.Content[i+1].Value = "codex"
				}
			}
			if ai {
				kept := []*yaml.Node{}
				for i := 0; i < len(node.Content); i += 2 {
					if slices.Contains([]string{"allowed_tools", "denied_tools", "settingSources"}, node.Content[i].Value) {
						continue
					}
					kept = append(kept, node.Content[i], node.Content[i+1])
				}
				node.Content = kept
			}
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(&document)
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, err
	}
	return out.Bytes(), encoder.Close()
}
