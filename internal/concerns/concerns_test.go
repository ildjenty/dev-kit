package concerns_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

func standards(t *testing.T) (string, concerns.Map) {
	t.Helper()
	dir := t.TempDir()
	testutil.Standards(t, dir, ".")
	return dir, testutil.Must(concerns.Load(dir))
}
func TestShippedOwnershipAndQualitySections(t *testing.T) {
	dir, m := standards(t)
	testutil.Equal(t, len(m.Qualities), len(concerns.Names()))
	for _, q := range concerns.Names() {
		text := string(testutil.Must(os.ReadFile(filepath.Join(dir, "review", q+".md"))))
		for _, heading := range []string{"Decisions", "Code", "Blocking", "Not reported"} {
			testutil.Equal(t, strings.Count(text, "## "+heading+"\n"), 1)
		}
	}
}

func TestDocumentOwnsDeliveredWritingPrinciplesAndRefineStillReceivesThem(t *testing.T) {
	dir, m := standards(t)
	q := m.Qualities["document"]
	testutil.Equal(t, q.Principles, []string{"writing.md#2", "writing.md#3", "writing.md#4", "writing.md#5", "writing.md#6", "writing.md#8", "writing.md#9"})
	testutil.Equal(t, q.RoutingThreshold, 0.6)
	testutil.Equal(t, q.Plan, []string{})
	testutil.Equal(t, q.Code, []string{"review/document.md"})
	testutil.Equal(t, slices.Contains(m.Judged("code"), "document"), true)
	testutil.Equal(t, slices.Contains(m.Judged("plan"), "document"), false)
	for _, ref := range q.Principles {
		testutil.Equal(t, slices.Contains(m.Materials(dir, "judge:refine", "code").Principles, filepath.Join(dir, ref)), true)
		testutil.Equal(t, slices.Contains(m.Others["judge:refine"], ref), false)
	}
	testutil.Equal(t, slices.Contains(m.Others["judge:refine"], "writing.md#7"), true)
	testutil.Equal(t, slices.Contains(m.Others["judge:pr-body"], "writing.md#1"), true)
}
func TestOwnershipRejectsMissingDuplicateAndUnknownPrinciples(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*concerns.Map)
		want   string
	}{
		{"unowned", func(m *concerns.Map) { m.EveryQuality = nil }, "designing.md#1 has 0 owners"},
		{"duplicate", func(m *concerns.Map) { m.Others["writer:plan"] = []string{"designing.md#1"} }, "designing.md#1 has 2 owners"},
		{"unknown principle", func(m *concerns.Map) { m.EveryQuality = append(m.EveryQuality, "designing.md#999") }, "unknown principle: designing.md#999"},
		{"unknown quality", func(m *concerns.Map) { m.Qualities["diff"] = concerns.Quality{} }, "unknown quality: diff"},
		{"missing quality", func(m *concerns.Map) { delete(m.Qualities, "scope") }, "missing quality: scope"},
		{"criteria", func(m *concerns.Map) {
			q := m.Qualities["delivery"]
			q.Plan = []string{"review/structure.md"}
			m.Qualities["delivery"] = q
		}, "quality criteria must be review/delivery.md"},
		{"unknown document", func(m *concerns.Map) { m.Others["writer:plan"] = []string{"missing.md"} }, "missing document: missing.md"},
		{"path escape", func(m *concerns.Map) { m.Others["writer:plan"] = []string{"../outside.md"} }, "invalid document"},
	} {
		t.Run(tc.name, func(t *testing.T) { dir, m := standards(t); tc.change(&m); testutil.Fails(t, m.Check(dir), tc.want) })
	}
	dir, m := standards(t)
	testutil.Put(t, dir, "extra.md", "## 1. New principle\n")
	testutil.Put(t, dir, "review/unused.md", "# Unused\n")
	err := m.Check(dir)
	testutil.Fails(t, err, "extra.md#1 has 0 owners")
	testutil.Fails(t, err, "unmapped review document: review/unused.md")
	testutil.OK(t, os.Remove(filepath.Join(dir, "review/security.md")))
	testutil.Fails(t, m.Check(dir), "missing document: review/security.md")
}
func TestLoadAndCLIValidateTheSelectedFolder(t *testing.T) {
	dir, _ := standards(t)
	var out, stderr bytes.Buffer
	registry := cli.New("test", concerns.Commands())
	env := cli.Env{Cwd: dir, Stdout: &out, Stderr: &stderr}
	testutil.Equal(t, registry.Execute([]string{"concerns", "check", "."}, env), 0)
	testutil.Contains(t, out.String(), `"valid":true`)
	testutil.Put(t, dir, "concerns.json", `{"unexpected":true}`)
	testutil.Equal(t, registry.Execute([]string{"concerns", "check", "."}, env), 1)
	testutil.Contains(t, stderr.String(), "unknown field")
	testutil.Equal(t, registry.Execute([]string{"concerns", "check", "a", "b"}, env), 2)
}
func TestCLIDefaultsToStandardsFolder(t *testing.T) {
	dir := t.TempDir()
	testutil.Standards(t, dir, ".devkit/standards")
	var out, stderr bytes.Buffer
	testutil.Equal(t, cli.New("test", concerns.Commands()).Execute([]string{"concerns", "check"}, cli.Env{Cwd: dir, Stdout: &out, Stderr: &stderr}), 0)
	testutil.Contains(t, out.String(), `"valid":true`)
}
func TestMaterialsComeFromMapForEachPhaseAndWriter(t *testing.T) {
	dir, m := standards(t)
	delivery := m.Materials(dir, "delivery", "plan")
	testutil.Equal(t, delivery.Criteria, []string{filepath.Join(dir, "review/delivery.md")})
	for _, phase := range []string{"plan", "code"} {
		for _, q := range m.Judged(phase) {
			material := m.Materials(dir, q, phase)
			testutil.Equal(t, material.Criteria, []string{filepath.Join(dir, "review", q+".md")})
			if phase == "plan" {
				testutil.Equal(t, material.EveryQuality, []string{filepath.Join(dir, "designing.md#1")})
			} else {
				testutil.Equal(t, material.EveryQuality, []string{})
			}
			testutil.Equal(t, material.Common, filepath.Join(dir, "review/README.md"))
		}
	}
	for _, item := range []struct{ subject, phase string }{{"writer:requirements", "plan"}, {"judge:requirements", "plan"}, {"writer:plan", "plan"}, {"writer:implement", "code"}, {"writer:tests", "code"}, {"writer:fix", "code"}} {
		testutil.Equal(t, m.Materials(dir, item.subject, item.phase).EveryQuality, []string{filepath.Join(dir, "designing.md#1")})
	}
	testutil.Equal(t, m.Materials(dir, "delivery", "code").Criteria, []string{})
	testutil.Equal(t, m.Materials(dir, "writer:retro", "plan"), m.Materials(dir, "judge:retro", "plan"))
	writer := m.Materials(dir, "writer:plan", "plan")
	testutil.Equal(t, len(writer.Criteria), len(m.Judged("plan")))
	testutil.Equal(t, m.Materials(dir, "judge:refine", "plan").Criteria[0], filepath.Join(dir, "review/wording.md"))
}
func TestWorkflowJudgeNodesEqualMappedPhases(t *testing.T) {
	_, m := standards(t)
	for _, item := range []struct{ workflow, phase string }{{"plan", "plan"}, {"judge", "code"}} {
		data := testutil.Must(kit.Files.ReadFile(".archon/workflows/dev/" + item.workflow + "/" + item.workflow + ".yaml"))
		var doc map[string]any
		testutil.OK(t, yaml.Unmarshal(data, &doc))
		found := []string{}
		var visit func(map[string]any)
		visit = func(d map[string]any) {
			if id, ok := d["id"].(string); ok && strings.HasPrefix(id, "judge-") {
				q := strings.TrimPrefix(id, "judge-")
				found = append(found, q)
				testutil.Equal(t, d["provider"], "claude")
				testutil.Equal(t, d["context"], "fresh")
				testutil.Equal(t, d["mutates_checkout"], false)
				if d["allowed_tools"] == nil && d["denied_tools"] == nil {
					t.Errorf("%s lacks read-only tools", id)
				}
			}
			for _, v := range d {
				switch value := v.(type) {
				case map[string]any:
					visit(value)
				case []any:
					for _, entry := range value {
						if row, ok := entry.(map[string]any); ok {
							visit(row)
						}
					}
				}
			}
		}
		visit(doc)
		want := m.Judged(item.phase)
		slices.Sort(found)
		slices.Sort(want)
		testutil.Equal(t, found, want)
	}
}
func TestConfigAndSchemaQualitySetsMatchConstant(t *testing.T) {
	expected := concerns.Names()
	slices.Sort(expected)
	defaults := config.Defaults("")
	keys := []string{}
	for q := range defaults.Qualities {
		keys = append(keys, q)
	}
	slices.Sort(keys)
	testutil.Equal(t, keys, expected)
	repo := testutil.Repo(t)
	for _, q := range concerns.Names() {
		data := testutil.Must(json.Marshal(map[string]any{"qualities": map[string]any{q: map[string]any{"paths": []any{}}}}))
		testutil.Put(t, repo, ".devkit/config.json", string(data))
		_, err := config.Load(repo)
		testutil.OK(t, err)
	}
	for _, file := range []string{"plan.json", "judgment.json"} {
		schema := testutil.Must(record.Schema(strings.TrimSuffix(file, ".json")))
		checked := 0
		var visit func(any)
		visit = func(value any) {
			switch value := value.(type) {
			case map[string]any:
				if enum, ok := value["enum"].([]any); ok && slices.Contains(enum, any("readability")) {
					qualities := []string{}
					for _, name := range enum {
						s := name.(string)
						if !slices.Contains([]string{"requirements", "pr-body", "report", "dispute", "refine", "retro", "optimize"}, s) {
							qualities = append(qualities, s)
						}
					}
					slices.Sort(qualities)
					testutil.Equal(t, qualities, expected)
					checked++
				}
				for _, child := range value {
					visit(child)
				}
			case []any:
				for _, child := range value {
					visit(child)
				}
			}
		}
		visit(schema)
		if checked == 0 {
			t.Fatalf("no quality enum checked in %s", file)
		}
	}
}

func TestF11UnknownConsumersAndCrossOwnedReferencesFail(t *testing.T) {
	t.Run("unknown consumer", func(t *testing.T) {
		dir, m := standards(t)
		m.Others["judge:does-not-exist"] = m.Others["writer:implement"]
		delete(m.Others, "writer:implement")
		testutil.Fails(t, m.Check(dir), "unknown owner: judge:does-not-exist")
	})
	t.Run("another owner's principle", func(t *testing.T) {
		dir, m := standards(t)
		testutil.Put(t, dir, "review/scope.md", "Check `context.md#5`.\n")
		testutil.Fails(t, m.Check(dir), "context.md#5")
	})
}

func TestF14TestJudgeHonorsPlannedVerificationMethod(t *testing.T) {
	data := testutil.Must(kit.Files.ReadFile(".devkit/standards/review/test.md"))
	text := string(data)
	for _, part := range []string{"screen_record", "owner_check", "named test", "completion condition", "configured check"} {
		testutil.Contains(t, text, part)
	}
}
