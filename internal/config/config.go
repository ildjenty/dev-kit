package config

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

type Check struct {
	Name           string   `json:"name"`
	Command        string   `json:"command"`
	Paths          []string `json:"paths,omitempty"`
	TimeoutSeconds float64  `json:"timeout_seconds"`
}
type Quality struct {
	Paths []string `json:"paths"`
}
type DecisionRule struct {
	Paths   []string `json:"paths"`
	Quality string   `json:"quality"`
	Ask     bool     `json:"ask"`
}
type Limits struct {
	Restarts int `json:"restarts"`
	Fixes    int `json:"fixes"`
	Passes   int `json:"passes"`
}
type Refine struct {
	RelevanceThreshold float64 `json:"relevance_threshold"`
}
type Config struct {
	Provider      string             `json:"provider"`
	Sessions      []string           `json:"sessions"`
	Language      string             `json:"language"`
	Skills        map[string]string  `json:"skills"`
	Standards     string             `json:"standards"`
	Docs          string             `json:"docs"`
	Decisions     string             `json:"decisions"`
	Topics        string             `json:"topics"`
	Units         string             `json:"units"`
	Checks        []Check            `json:"checks"`
	SkeletonCheck []string           `json:"skeleton_check"`
	Qualities     map[string]Quality `json:"qualities"`
	DecisionPaths []DecisionRule     `json:"decision_paths"`
	Refine        Refine             `json:"refine"`
	Limits        Limits             `json:"limits"`
}

func (c Check) MarshalJSON() ([]byte, error) {
	type plain Check
	if c.Paths == nil {
		return json.Marshal(plain(c))
	}
	return json.Marshal(struct {
		plain
		Paths []string `json:"paths"`
	}{plain(c), c.Paths})
}

func Defaults(standards string) Config {
	var c Config
	_ = json.Unmarshal([]byte(defaultJSON), &c)
	for _, q := range concerns.Names() {
		if _, ok := c.Qualities[q]; !ok {
			c.Qualities[q] = Quality{Paths: []string{}}
		}
	}
	if standards != "" && standards != c.Standards {
		c.Standards = standards
		paths := c.Qualities["structure"]
		for i, path := range paths.Paths {
			if path == ".devkit/standards/**" {
				paths.Paths[i] = standards + "/**"
			}
		}
		for i := range c.DecisionPaths {
			for j, path := range c.DecisionPaths[i].Paths {
				if path == ".devkit/standards/**" {
					c.DecisionPaths[i].Paths[j] = standards + "/**"
				}
			}
		}
	}
	return c
}

func Load(cwd string) (Config, error) {
	root, err := gitx.Run(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(filepath.Join(root, ".devkit/config.json"))
	if os.IsNotExist(err) {
		return Defaults(""), nil
	}
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}
func Parse(data []byte) (Config, error) {
	var supplied map[string]any
	if err := json.Unmarshal(data, &supplied); err != nil {
		var other any
		if json.Unmarshal(data, &other) == nil {
			return Config{}, invalid("expected an object")
		}
		return Config{}, err
	}
	if supplied == nil {
		return Config{}, invalid("expected an object")
	}
	if _, exists := supplied["norms"]; exists {
		return Config{}, invalid("norms was renamed to standards")
	}
	standards, _ := supplied["standards"].(string)
	base, _ := json.Marshal(Defaults(standards))
	var merged map[string]any
	_ = json.Unmarshal(base, &merged)
	for key, value := range supplied {
		if key == "limits" || key == "qualities" || key == "skills" || key == "refine" {
			object, ok := value.(map[string]any)
			if !ok {
				return Config{}, invalid(key + " must be an object")
			}
			nested := merged[key].(map[string]any)
			for k, v := range object {
				nested[k] = v
			}
		} else {
			merged[key] = value
		}
	}
	if _, exists := supplied["sessions"]; !exists {
		merged["sessions"] = []any{merged["provider"]}
	}
	if err := shape(merged); err != nil {
		return Config{}, err
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return Config{}, err
	}
	var c Config
	err = json.Unmarshal(data, &c)
	return c, err
}
func ChecksAcrossBase(cwd, baseBranch string) ([]Check, error) {
	current, err := Load(cwd)
	if err != nil {
		return nil, err
	}
	g, err := gitx.New(cwd, baseBranch)
	if err != nil {
		return nil, err
	}
	base, err := g.MergeBase("")
	if err != nil {
		return nil, err
	}
	previous := Defaults("")
	if g.OK("cat-file", "-e", base+":.devkit/config.json") {
		data, err := g.Run("show", base+":.devkit/config.json")
		if err != nil {
			return nil, err
		}
		previous, err = Parse([]byte(data))
		if err != nil {
			return nil, err
		}
	}
	result := append([]Check{}, previous.Checks...)
	for _, check := range current.Checks {
		if !slices.ContainsFunc(result, func(old Check) bool { return reflect.DeepEqual(old, check) }) {
			result = append(result, check)
		}
	}
	return result, nil
}
func Covers(touch, path string) bool {
	folder := strings.TrimRight(strings.TrimPrefix(touch, "./"), "/")
	return folder == "." || folder == "" || path == folder || strings.HasPrefix(path, folder+"/")
}
func invalid(at string) error { return fmt.Errorf("invalid .devkit/config.json: %s", at) }
func text(value any) bool     { s, ok := value.(string); return ok && strings.TrimSpace(s) != "" }
func texts(value any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if !text(item) {
			return false
		}
	}
	return true
}
func keys(value any, allowed []string) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for key := range object {
		if !slices.Contains(allowed, key) {
			return false
		}
	}
	return true
}

var SkillKeys = []string{"devkit-runs", "devkit-discuss", "devkit-optimize"}

func shape(c map[string]any) error {
	if !keys(c, []string{"provider", "sessions", "language", "skills", "standards", "docs", "decisions", "units", "topics", "checks", "skeleton_check", "qualities", "decision_paths", "limits", "refine"}) {
		return invalid("unknown setting")
	}
	if c["provider"] != "claude" && c["provider"] != "codex" {
		return invalid("provider must be claude or codex")
	}
	sessions, ok := c["sessions"].([]any)
	if !ok {
		return invalid("sessions must be a list of claude or codex")
	}
	seenSessions := map[string]bool{}
	for _, value := range sessions {
		name, _ := value.(string)
		if name != "claude" && name != "codex" || seenSessions[name] {
			return invalid("sessions must contain distinct claude or codex entries")
		}
		seenSessions[name] = true
	}
	if !text(c["language"]) {
		return invalid("language must be a non-empty string")
	}
	if !keys(c["skills"], SkillKeys) {
		return invalid("skills must map " + strings.Join(SkillKeys, ", ") + " to skill names")
	}
	skills := c["skills"].(map[string]any)
	used := map[string]bool{}
	for _, key := range SkillKeys {
		name, _ := skills[key].(string)
		if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(name) {
			return invalid("skills." + key + " must contain only lowercase letters, digits and hyphens")
		}
		if used[name] {
			return invalid("skill names must be distinct")
		}
		used[name] = true
	}
	for _, key := range []string{"standards", "docs", "decisions", "units", "topics"} {
		s, _ := c[key].(string)
		if !text(c[key]) || strings.HasPrefix(s, "/") || strings.Contains(s, "\\") || slices.Contains(strings.Split(s, "/"), "..") || slices.Contains(strings.Split(s, "/"), ".") {
			return invalid(key + " must be a repository-relative folder")
		}
	}
	checks, ok := c["checks"].([]any)
	if !ok {
		return invalid("checks must be an array")
	}
	names := map[string]bool{}
	for _, value := range checks {
		if !keys(value, []string{"name", "command", "paths", "timeout_seconds"}) {
			return invalid("check")
		}
		check := value.(map[string]any)
		name, _ := check["name"].(string)
		if !text(check["name"]) || names[name] {
			return invalid("check names must be non-empty and unique")
		}
		names[name] = true
		if !text(check["command"]) {
			return invalid("check " + name + ": command must be a non-empty string")
		}
		if paths, exists := check["paths"]; exists && !texts(paths) {
			return invalid("check " + name + ": paths must be an array of globs")
		}
		n, ok := check["timeout_seconds"].(float64)
		if !ok || math.IsInf(n, 0) || math.IsNaN(n) || n <= 0 {
			return invalid("check " + name + ": timeout_seconds must be positive")
		}
	}
	if !texts(c["skeleton_check"]) {
		return invalid("skeleton_check must name configured checks")
	}
	for _, name := range c["skeleton_check"].([]any) {
		if !names[name.(string)] {
			return invalid("skeleton_check must name configured checks")
		}
	}
	if !keys(c["qualities"], concerns.Names()) {
		return invalid("qualities")
	}
	for quality, value := range c["qualities"].(map[string]any) {
		if !keys(value, []string{"paths"}) {
			return invalid("qualities." + quality)
		}
		if !texts(value.(map[string]any)["paths"]) {
			return invalid("qualities." + quality + ".paths must be an array of globs")
		}
	}
	rules, ok := c["decision_paths"].([]any)
	if !ok {
		return invalid("decision_paths must be an array")
	}
	for _, value := range rules {
		if !keys(value, []string{"paths", "quality", "ask"}) {
			return invalid("decision_paths entry")
		}
		rule := value.(map[string]any)
		quality, _ := rule["quality"].(string)
		_, boolean := rule["ask"].(bool)
		if !texts(rule["paths"]) || !slices.Contains(concerns.Names(), quality) || !boolean {
			return invalid("decision_paths entry must have paths, quality and ask")
		}
	}
	if !keys(c["refine"], []string{"relevance_threshold"}) {
		return invalid("refine")
	}
	threshold, ok := c["refine"].(map[string]any)["relevance_threshold"].(float64)
	if !ok || math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
		return invalid("refine.relevance_threshold must be a number between 0 and 1")
	}
	if !keys(c["limits"], []string{"restarts", "fixes", "passes"}) {
		return invalid("limits")
	}
	for _, key := range []string{"restarts", "fixes", "passes"} {
		n, ok := c["limits"].(map[string]any)[key].(float64)
		if !ok || n < 0 || math.Trunc(n) != n {
			return invalid("limits." + key + " must be a non-negative integer")
		}
	}
	return nil
}

func Matches(globs []string, path string) bool {
	path = strings.TrimPrefix(path, "./")
	for _, glob := range globs {
		var source strings.Builder
		source.WriteString("^")
		chars := []rune(glob)
		for i := 0; i < len(chars); i++ {
			switch chars[i] {
			case '*':
				if i+1 < len(chars) && chars[i+1] == '*' {
					if i+2 < len(chars) && chars[i+2] == '/' {
						source.WriteString("(?:.*/)?")
						i += 2
					} else {
						source.WriteString(".*")
						i++
					}
				} else {
					source.WriteString("[^/]*")
				}
			case '?':
				source.WriteString("[^/]")
			default:
				source.WriteString(regexp.QuoteMeta(string(chars[i])))
			}
		}
		source.WriteString("$")
		if regexp.MustCompile(source.String()).MatchString(path) {
			return true
		}
	}
	return false
}
func pathsOf(file gitx.File) []string {
	if file.OldPath != "" {
		return []string{file.OldPath, file.Path}
	}
	return []string{file.Path}
}
func ChecksFor(c Config, changes gitx.Changes) []Check {
	selected := []Check{}
	for _, check := range c.Checks {
		matched := check.Paths == nil
		for _, file := range changes.Files {
			for _, path := range pathsOf(file) {
				matched = matched || Matches(check.Paths, path)
			}
		}
		if matched {
			selected = append(selected, check)
		}
	}
	return selected
}
func QualitiesFor(c Config, changes gitx.Changes) []string {
	result := []string{}
	for _, quality := range concerns.Names() {
		matched := false
		for _, file := range changes.Files {
			if quality == "structure" && file.Status != "" && strings.ContainsRune("ADRC", rune(file.Status[0])) {
				matched = true
			}
			for _, path := range pathsOf(file) {
				matched = matched || Matches(c.Qualities[quality].Paths, path)
			}
		}
		if matched {
			result = append(result, quality)
		}
	}
	return result
}

type DecisionChange struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Status  string `json:"status"`
	Kind    string `json:"kind"`
	Quality string `json:"quality"`
	Ask     bool   `json:"ask"`
}

func DecisionChanges(c Config, changes gitx.Changes) []DecisionChange {
	result := []DecisionChange{}
	for _, file := range changes.Files {
		if slices.Contains(changes.Excluded, file.Path) && (file.OldPath == "" || slices.Contains(changes.Excluded, file.OldPath)) {
			continue
		}
		for _, rule := range c.DecisionPaths {
			matched := false
			for _, path := range pathsOf(file) {
				matched = matched || Matches(rule.Paths, path)
			}
			if matched {
				status := file.Status
				if status == "" {
					status = "M"
				}
				result = append(result, DecisionChange{file.Path, file.OldPath, status, "file", rule.Quality, rule.Ask})
			}
		}
	}
	recordDirectory := func(path string) bool {
		found := false
		for _, file := range changes.Files {
			for _, changedPath := range pathsOf(file) {
				if !Covers(path, changedPath) {
					continue
				}
				found = true
				if !Covers(c.Decisions, changedPath) && !Covers(c.Units, changedPath) && !Covers(c.Topics, changedPath) && !slices.Contains(changes.Excluded, changedPath) {
					return false
				}
			}
		}

		return found
	}
	for _, path := range changes.AddedDirectories {
		if recordDirectory(path) {
			continue
		}
		result = append(result, DecisionChange{Path: path, Status: "A", Kind: "directory", Quality: "structure"})
	}
	for _, path := range changes.RemovedDirectories {
		if recordDirectory(path) {
			continue
		}
		result = append(result, DecisionChange{Path: path, Status: "D", Kind: "directory", Quality: "structure"})
	}
	return result
}
func Commands() []cli.Command {
	return []cli.Command{{Area: "config", Usage: "devkit config [cwd]", Run: func(args []string, env cli.Env) error {
		cwd := env.Cwd
		if len(args) > 0 {
			cwd = env.Path(args[0])
		}
		c, err := Load(cwd)
		if err != nil {
			return err
		}
		return env.JSON(c)
	}}}
}
