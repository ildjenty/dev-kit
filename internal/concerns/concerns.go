package concerns

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
)

const Qualities = "structure data contract security ui cost test delivery scope readability document"

const Consumers = "writer:implement writer:requirements writer:plan writer:refine writer:tests writer:fix writer:pr-body writer:sync writer:retro judge:requirements judge:pr-body judge:refine judge:retro judge:step-back judge:dispute"

func Names() []string { return strings.Fields(Qualities) }

type Quality struct {
	Title            string   `json:"title"`
	Decides          string   `json:"decides"`
	Routing          string   `json:"routing,omitempty"`
	RoutingThreshold float64  `json:"routing_threshold,omitempty"`
	Principles       []string `json:"principles"`
	Plan             []string `json:"plan"`
	Code             []string `json:"code"`
}
type Map struct {
	Qualities    map[string]Quality  `json:"qualities"`
	EveryQuality []string            `json:"every_quality"`
	Others       map[string][]string `json:"others"`
}
type Material struct {
	Criteria     []string `json:"criteria"`
	Principles   []string `json:"principles"`
	EveryQuality []string `json:"every_quality"`
	Common       string   `json:"common"`
	Authority    string   `json:"authority"`
}

func Load(dir string) (Map, error) {
	data, err := os.ReadFile(filepath.Join(dir, "concerns.json"))
	if err != nil {
		return Map{}, err
	}
	if !json.Valid(data) {
		return Map{}, fmt.Errorf("invalid concerns.json")
	}
	var m Map
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&m); err != nil {
		return Map{}, err
	}
	if err = m.Check(dir); err != nil {
		return Map{}, err
	}
	return m, nil
}
func (m Map) Judged(phase string) []string {
	out := []string{}
	for _, q := range Names() {
		if phase == "plan" && len(m.Qualities[q].Plan) > 0 || phase == "code" && len(m.Qualities[q].Code) > 0 {
			out = append(out, q)
		}
	}
	return out
}
func (m Map) Materials(dir, subject, phase string) Material {
	paths := func(refs []string) []string {
		out := []string{}
		for _, ref := range refs {
			out = append(out, filepath.Join(dir, ref))
		}
		return out
	}
	material := Material{Criteria: []string{}, Principles: []string{}, EveryQuality: []string{}, Common: filepath.Join(dir, "review/README.md"), Authority: filepath.Join(dir, "authority.md")}
	if q, ok := m.Qualities[subject]; ok {
		if phase == "plan" {
			material.Criteria = paths(q.Plan)
			material.EveryQuality = paths(m.EveryQuality)
		} else {
			material.Criteria = paths(q.Code)
		}
		material.Principles = paths(q.Principles)
	} else {
		for _, ref := range m.Others[subject] {
			if strings.Contains(ref, "#") {
				material.Principles = append(material.Principles, filepath.Join(dir, ref))
			} else {
				material.Criteria = append(material.Criteria, filepath.Join(dir, ref))
			}
		}
	}
	if subject == "judge:refine" {
		material.Principles = append(material.Principles, paths(m.Qualities["document"].Principles)...)
		binding := m.Materials(dir, "judge:retro", phase)
		material.Criteria = append(material.Criteria, binding.Criteria...)
		material.Principles = append(material.Principles, binding.Principles...)
	}
	if strings.HasPrefix(subject, "writer:") || subject == "judge:dispute" {
		name := strings.TrimPrefix(subject, "writer:")
		if name == "requirements" || name == "refine" || name == "pr-body" || name == "retro" {
			entry := m.Materials(dir, "judge:"+name, phase)
			material.Criteria = append(material.Criteria, entry.Criteria...)
			material.Principles = append(material.Principles, entry.Principles...)
		}
		if name == "plan" || name == "implement" || name == "fix" || name == "tests" || subject == "judge:dispute" {
			for _, q := range m.Judged(phase) {
				entry := m.Materials(dir, q, phase)
				material.Criteria = append(material.Criteria, entry.Criteria...)
				material.Principles = append(material.Principles, entry.Principles...)
			}
			material.EveryQuality = paths(m.EveryQuality)
		}
	}
	if subject == "writer:requirements" || subject == "judge:requirements" {
		material.EveryQuality = paths(m.EveryQuality)
	}
	return material
}
func (m Map) Check(dir string) error {
	problems := []string{}
	add := func(s string) { problems = append(problems, s) }
	for _, q := range Names() {
		entry, ok := m.Qualities[q]
		if !ok {
			add("missing quality: " + q)
			continue
		}
		if strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(entry.Decides) == "" {
			add("quality needs title and decides: " + q)
		}
		if Routed(q) && len(entry.Code) > 0 && strings.TrimSpace(entry.Routing) == "" {
			add("quality needs a routing description: " + q)
		}
		if Routed(q) && len(entry.Code) > 0 && (entry.RoutingThreshold <= 0 || entry.RoutingThreshold > 1) {
			add("quality needs a routing_threshold above 0 and at most 1: " + q)
		}
		for _, phase := range [][]string{entry.Plan, entry.Code} {
			if len(phase) > 0 && !slices.Equal(phase, []string{"review/" + q + ".md"}) {
				add("quality criteria must be review/" + q + ".md")
			}
		}
	}
	for q := range m.Qualities {
		if !slices.Contains(Names(), q) {
			add("unknown quality: " + q)
		}
	}
	numbered := map[string]bool{}
	reviews := []string{}
	pattern := regexp.MustCompile(`(?m)^## ([0-9]+)\. `)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(relative, "review/") {
			if relative != "review/README.md" {
				reviews = append(reviews, relative)
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			numbered[relative+"#"+match[1]] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	owners := map[string][]string{}
	named := map[string]bool{}
	reviewOwners := map[string][]string{}
	check := func(owner string, refs []string, principles bool) {
		for _, ref := range refs {
			document, number, hasNumber := strings.Cut(ref, "#")
			if document == "" || filepath.IsAbs(document) || filepath.Clean(document) != document || document == ".." || strings.HasPrefix(document, "../") {
				add("invalid document: " + ref)
				continue
			}
			named[document] = true
			if strings.HasPrefix(document, "review/") && !slices.Contains(reviewOwners[document], owner) {
				reviewOwners[document] = append(reviewOwners[document], owner)
			}
			if info, err := os.Stat(filepath.Join(dir, document)); err != nil || info.IsDir() {
				add("missing document: " + document)
			}
			if principles && !hasNumber {
				add("principle reference needs a number: " + ref)
			}
			if hasNumber {
				if number == "" || !numbered[ref] {
					add("unknown principle: " + ref)
				}
				owners[ref] = append(owners[ref], owner)
			}
		}
	}
	for q, entry := range m.Qualities {
		check(q, entry.Principles, true)
		check(q, entry.Plan, false)
		check(q, entry.Code, false)
	}
	check("every_quality", m.EveryQuality, true)
	for owner, refs := range m.Others {
		if !slices.Contains(strings.Fields(Consumers), owner) {
			add("unknown owner: " + owner)
		}
		check(owner, refs, false)
	}
	for ref := range numbered {
		if len(owners[ref]) != 1 {
			add(fmt.Sprintf("%s has %d owners: %s", ref, len(owners[ref]), strings.Join(owners[ref], ", ")))
		}
	}
	reference := regexp.MustCompile(`[A-Za-z0-9_./-]+\.md#[0-9]+`)
	for _, doc := range reviews {
		data, e := os.ReadFile(filepath.Join(dir, doc))
		if e != nil {
			return e
		}
		for _, ref := range reference.FindAllString(string(data), -1) {
			for _, owner := range reviewOwners[doc] {
				if !slices.Contains(owners[ref], owner) && !(slices.Contains(Names(), owner) && slices.Contains(owners[ref], "every_quality")) {
					add(doc + " references " + ref + " owned elsewhere: " + strings.Join(owners[ref], ", "))
				}
			}
		}
		if !named[doc] {
			add("unmapped review document: " + doc)
		}
	}
	for _, doc := range []string{"review/README.md", "authority.md"} {
		if _, err := os.Stat(filepath.Join(dir, doc)); err != nil {
			add("missing document: " + doc)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("concerns check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}
func Commands() []cli.Command {
	return []cli.Command{{Area: "concerns", Name: "check", Usage: "devkit concerns check [standards-dir]", Run: func(args []string, env cli.Env) error {
		if len(args) > 1 {
			return cli.Usage("devkit concerns check [standards-dir]")
		}
		dir := ".devkit/standards"
		if len(args) == 1 {
			dir = args[0]
		}
		_, err := Load(env.Path(dir))
		if err != nil {
			return err
		}
		return env.JSON(map[string]bool{"valid": true})
	}}}
}

func Routed(quality string) bool {
	return quality != "scope" && quality != "test" && quality != "readability" && quality != "delivery"
}
