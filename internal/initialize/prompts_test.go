package initialize

import (
	"github.com/ildjenty/dev-kit/kit"
	"io/fs"
	"strings"
	"testing"
)

func TestPromptsStartWithPurposeAndNameHandoff(t *testing.T) {
	err := fs.WalkDir(kit.Files, ".archon/workflows/dev", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") || !strings.Contains(path, "/commands/") && !strings.HasSuffix(path, "/.shared/judge/judging.md") {
			return nil
		}
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if !strings.HasPrefix(text, "## Purpose\n\n") {
			t.Errorf("%s lacks an initial purpose", path)
		}
		purpose := strings.SplitN(text, "\n\n", 3)[1]
		if !strings.Contains(purpose, "verify") || !strings.Contains(text, "language") {
			t.Errorf("%s lacks independent evidence or language instructions", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRestoredStepSpecificInstructions(t *testing.T) {
	cases := map[string][]string{
		"plan/commands/write-structure.md": {"Only structure", "fail loudly", "existing bodies unchanged", "no new logic, tests or comments", "Change nothing the requirements do not need", "Keep the IDs", "returned"},
		"plan/commands/write-detail.md":    {"Preserve the structure rows", "Add, remove or move no file", "retaining IDs", "returned"},
		"implement/commands/implement.md":  {"this unit only", "Never change this unit's tests", "uncertain", "Start from the skeleton", "cause shown by the check logs", "Leave committing to the workflow", "returned"},
		"implement/commands/tests.md":      {"completion-condition text, never from implementation code", "not `later_units`", "do not weaken, skip or delete", "fail against the", "skeleton", "only test files"},
		"requirements/commands/write.md":   {"explicitly named by the owner is a constraint", "session is a candidate", "Never fill ambiguity with a guess", "only what stops the damage", "returned"},
		"fix/commands/fix.md":              {"\"target\": \"finding|decision\"", "Difficulty of repair is not an objection", "failure cannot happen", "criterion does not apply", "break a completion condition", "command results"},
		"implement/commands/step-back.md":  {"Judge from the logs", "not from what the implementation session said"},
		"fix/commands/step-back.md":        {"Judge from the logs", "not from what the fixing session said"},
		".shared/judge/judging.md":         {"absolute `tree` path in `target.json`", "Grep and Glob", "every document in materials `criteria`", "Without requirements", "review/structure.md"},
	}
	for path, parts := range cases {
		text, err := kit.Files.ReadFile(".archon/workflows/dev/" + path)
		if err != nil {
			t.Fatal(err)
		}
		for _, part := range parts {
			if !strings.Contains(string(text), part) {
				t.Errorf("%s lacks %q", path, part)
			}
		}
	}
}
