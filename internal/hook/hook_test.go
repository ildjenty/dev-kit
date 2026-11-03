package hook

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/oklog/ulid/v2"
)

func newHandler(t *testing.T) Handler {
	t.Helper()
	sources := make(map[string][]byte)
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "kit", "schemas", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = data
	}
	validator, err := ledger.NewValidator(sources)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler{
		Root: t.TempDir(), WorkID: ulid.Make().String(), Validator: validator,
		ReadConfig: func(string) (config.Config, error) {
			return config.Config{Deny: config.Deny{
				Rules: []string{"Bash(git push *)", "Agent(fork)", "mcp_all_write"},
				Judge: []string{"apply_patch", "mcp_write"},
			}}, nil
		},
		ReadAgent: func(_ string, _ event.Host, name string) (AgentDefinition, error) {
			return AgentDefinition{Name: name, Model: "inherit", Effort: "high"}, nil
		},
		InstructionRunner: func(instruction ledger.Instruction) (string, error) {
			if instruction.Kind == form.InstructionPlan {
				return "devkit-planner", nil
			}
			return "devkit-judge", nil
		},
	}
	appendPayload(t, h, &ledger.Work{
		ID: h.WorkID, TaskRef: "task.md", Branch: "devkit/" + h.WorkID,
		BaseCommit: "base", KitVersion: ledger.ReadableKitVersion,
	})
	return h
}

func appendPayload(t *testing.T, h Handler, payload ledger.Payload) string {
	t.Helper()
	store := fsstore.New(h.workRoot())
	id, err := store.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(id, string((ledger.Record{Payload: payload}).Kind()), payload); err != nil {
		t.Fatal(err)
	}
	return id
}

func process(t *testing.T, h Handler, e event.Event) event.Result {
	t.Helper()
	result, err := h.Process(e)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSessionAndPromptObservations(t *testing.T) {
	h := newHandler(t)
	bound := h.WorkID
	h.WorkID = ""
	e := event.Event{Host: event.ClaudeCode, Name: event.SessionStart, SessionID: "session"}
	if result := process(t, h, e); result.AdditionalContext != "" {
		t.Fatalf("unbound start: %+v", result)
	}
	observation, err := ReadSession(h.runRoot(), e.SessionID)
	if err != nil || observation == nil || observation.WorkID != "" {
		t.Fatalf("unbound observation: %+v, %v", observation, err)
	}
	h.WorkID = bound
	if err := PublishSession(h.runRoot(), SessionObservation{Host: e.Host, SessionID: e.SessionID, WorkID: bound, ObservedAt: time.Now().UTC()}, "start"); err != nil {
		t.Fatal(err)
	}
	if result := process(t, h, e); !strings.Contains(result.AdditionalContext, bound) {
		t.Fatalf("missing checkpoint: %+v", result)
	}
	observation, err = ReadSession(h.runRoot(), e.SessionID)
	if err != nil || observation.WorkID != bound {
		t.Fatalf("binding was lost: %+v, %v", observation, err)
	}
	e.Name, e.Prompt = event.UserPromptSubmit, "  Original answer\nwith a correction.\n"
	process(t, h, e)
	files, err := filepath.Glob(filepath.Join(h.workRoot(), "prompts", "*.txt"))
	if err != nil || len(files) != 1 {
		t.Fatalf("prompt files: %v, %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || string(data) != e.Prompt {
		t.Fatalf("original prompt: %q, %v", data, err)
	}
	e.Prompt = "<task-notification>Completed</task-notification>"
	process(t, h, e)
	files, _ = filepath.Glob(filepath.Join(h.workRoot(), "prompts", "*.txt"))
	if len(files) != 1 {
		t.Fatalf("completion notification was observed: %v", files)
	}
	e.Host = event.Codex
	process(t, h, e)
	files, _ = filepath.Glob(filepath.Join(h.workRoot(), "prompts", "*.txt"))
	if len(files) != 2 {
		t.Fatalf("Codex input was excluded: %v", files)
	}
	h.WorkID = ""
	process(t, h, e)
	if _, err := os.Stat(filepath.Join(h.runRoot(), "work", "prompts")); !os.IsNotExist(err) {
		t.Fatalf("unbound prompt was observed: %v", err)
	}
}

func TestSubagentSessionAndStopAreIgnored(t *testing.T) {
	h := Handler{Root: t.TempDir(), WorkID: "work"}
	for _, name := range []event.Name{event.SessionStart, event.Stop} {
		result := process(t, h, event.Event{Name: name, AgentID: "child"})
		if result != (event.Result{}) {
			t.Fatalf("subagent event: %+v", result)
		}
	}
	if _, err := os.Stat(h.runRoot()); !os.IsNotExist(err) {
		t.Fatalf("subagent event wrote observations: %v", err)
	}
}

func TestDenyRules(t *testing.T) {
	h := newHandler(t)
	for _, test := range []struct {
		command string
		deny    bool
	}{
		{"git push", true}, {"git push origin main", true}, {"git pushx", false},
		{"git status && git push origin", true}, {"git status || git push", true},
		{"git status; git push", true}, {"git status | git push", true},
		{`echo "safe; git push"`, false}, {`echo 'safe | git push'`, false},
		{`echo safe\;git push`, false}, {"git status", false},
	} {
		e := event.Event{Host: event.Codex, Name: event.PreToolUse, SessionID: "session", ToolName: "exec_command", Command: test.command}
		result := process(t, h, e)
		if (result.Decision == event.Deny) != test.deny {
			t.Errorf("%q: %+v", test.command, result)
		}
	}
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		for _, caller := range []struct {
			id, role string
			deny     bool
		}{
			{"", "", false},
			{"", "devkit-judge", false},
			{"child", "custom", false},
			{"child", "devkit-judge", true},
			{"child", "devkit-planner", true},
		} {
			for _, tool := range []string{"apply_patch", "mcp_write", "mcp_all_write", "Read"} {
				e := event.Event{Host: host, Name: event.PreToolUse, SessionID: "session", AgentID: caller.id, AgentType: caller.role, ToolName: tool}
				result := process(t, h, e)
				wantDeny := tool == "mcp_all_write" || caller.deny && tool != "Read"
				if (result.Decision == event.Deny) != wantDeny {
					t.Errorf("%s caller %+v tool %s: %+v", host, caller, tool, result)
				}
			}
		}
	}
	result := process(t, h, event.Event{Host: event.ClaudeCode, Name: event.PreToolUse, SessionID: "session", ToolName: "Agent", Agent: &event.AgentInvocation{Type: "fork"}})
	if result.Decision != event.Deny {
		t.Fatalf("fork launch: %+v", result)
	}
	h.WorkID = ""
	if result := process(t, h, event.Event{Name: event.PreToolUse, Command: "git push"}); result.Decision != "" {
		t.Fatalf("deny affected an unbound session: %+v", result)
	}
	if result := process(t, h, event.Event{Name: event.PreToolUse, AgentID: "child", AgentType: "devkit-judge", ToolName: "apply_patch"}); result.Decision != "" {
		t.Fatalf("judge deny affected an unbound session: %+v", result)
	}
}

func TestKitDenyDefaults(t *testing.T) {
	h := newHandler(t)
	data, err := os.ReadFile(filepath.Join("..", "..", "kit", "hosts", "deny.toml"))
	if err != nil {
		t.Fatal(err)
	}
	settings, err := config.Parse(append([]byte("kit_version = \"v1\"\n"), data...))
	if err != nil {
		t.Fatal(err)
	}
	h.ReadConfig = func(string) (config.Config, error) { return settings, nil }
	e := event.Event{Host: event.Codex, Name: event.PreToolUse, SessionID: "session", ToolName: "apply_patch"}
	if result := process(t, h, e); result.Decision == event.Deny {
		t.Fatalf("default denied a parent write: %+v", result)
	}
	for _, role := range []string{"devkit-judge", "devkit-planner"} {
		e.AgentID, e.AgentType = "child", role
		if result := process(t, h, e); result.Decision != event.Deny {
			t.Fatalf("default allowed a %s write: %+v", role, result)
		}
	}
}

func TestDevkitLaunchRestrictions(t *testing.T) {
	h := newHandler(t)
	for _, test := range []struct {
		host event.Host
		args event.AgentInvocation
		deny bool
	}{
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction issued"}, false},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-planner", Prompt: "devkit-instruction issued", Model: "inherit", Effort: "high", Isolation: "worktree"}, false},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction issued", Model: "other"}, true},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction issued", Effort: "low"}, true},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction issued\nextra"}, true},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction "}, true},
		{event.ClaudeCode, event.AgentInvocation{Type: "devkit-judge", Prompt: "Please evaluate this"}, true},
		{event.ClaudeCode, event.AgentInvocation{Type: "custom", Prompt: "Any prompt", Model: "other"}, false},
		{event.Codex, event.AgentInvocation{Type: "devkit-judge", ForkTurns: "none"}, false},
		{event.Codex, event.AgentInvocation{Type: "devkit-planner", ForkTurns: "all"}, true},
		{event.Codex, event.AgentInvocation{Type: "devkit-judge"}, true},
		{event.Codex, event.AgentInvocation{Type: "custom", ForkTurns: "all"}, false},
	} {
		tool := "Agent"
		if test.host == event.Codex {
			tool = "collaborationspawn_agent"
		}
		result := process(t, h, event.Event{Host: test.host, Name: event.PreToolUse, SessionID: "session", ToolName: tool, Agent: &test.args})
		if (result.Decision == event.Deny) != test.deny {
			t.Errorf("%s %+v: %+v", test.host, test.args, result)
		}
	}
}

func TestPauseBypassesObservationAndConfigurationFailures(t *testing.T) {
	h := Handler{Root: t.TempDir(), WorkID: "work"}
	if err := os.WriteFile(filepath.Join(h.Root, ".devkit"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := process(t, h, event.Event{Name: event.PreToolUse, SessionID: "session", ToolName: "Bash", Command: "devkit pause"})
	if result.Decision != event.Allow || result.SystemMessage == "" {
		t.Fatalf("pause was obstructed: %+v", result)
	}
	h = newHandler(t)
	h.ReadConfig = func(string) (config.Config, error) { return config.Config{}, errors.New("configuration unavailable") }
	result = process(t, h, event.Event{Name: event.PreToolUse, SessionID: "session", Command: "devkit pause"})
	if result.Decision != event.Allow {
		t.Fatalf("pause depended on configuration: %+v", result)
	}
}

func TestNormativeReference(t *testing.T) {
	h := newHandler(t)
	var reference, agentReference string
	h.ReadConfig = func(ref string) (config.Config, error) { reference = ref; return config.Config{}, nil }
	h.ReadAgent = func(ref string, _ event.Host, name string) (AgentDefinition, error) {
		agentReference = ref
		return AgentDefinition{Name: name}, nil
	}
	e := event.Event{Host: event.ClaudeCode, Name: event.PreToolUse, SessionID: "session", ToolName: "Agent", Agent: &event.AgentInvocation{Type: "devkit-judge", Prompt: "devkit-instruction issued"}}
	process(t, h, e)
	if reference != "devkit/"+h.WorkID || agentReference != reference {
		t.Fatalf("outside-round references: config %q, agent %q", reference, agentReference)
	}
	store := fsstore.New(h.workRoot())
	runID, err := store.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(runID, string(ledger.KindStageRun), &ledger.StageRun{ID: runID, StageID: "implement", PlanVersion: 1, State: ledger.StageWorking}); err != nil {
		t.Fatal(err)
	}
	roundID, err := store.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(roundID, string(ledger.KindRound), &ledger.Round{ID: roundID, StageRunID: runID, SessionID: "session", StartTarget: form.TargetVersion{Kind: form.TargetTree, Tree: "frozen-tree"}, Norms: []form.NormReference{}}); err != nil {
		t.Fatal(err)
	}
	process(t, h, e)
	if reference != "frozen-tree" || agentReference != reference {
		t.Fatalf("round references: config %q, agent %q", reference, agentReference)
	}
}

func TestSubagentResultCollection(t *testing.T) {
	for _, kind := range []form.InstructionKind{form.InstructionJudgment, form.InstructionPlanCheck, form.InstructionDetectionTest, form.InstructionPlan} {
		t.Run(string(kind), func(t *testing.T) {
			h := newHandler(t)
			appendPayload(t, h, &ledger.Instruction{ID: "issued", Kind: kind, TargetKey: "target", SessionID: "session"})
			runner := "devkit-judge"
			if kind == form.InstructionPlan {
				runner = "devkit-planner"
			}
			e := event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: "session", AgentID: "child", AgentType: runner, LastAssistantMessage: `{"instruction_id":"other"}`, Effort: "high", Model: "unobserved"}
			process(t, h, e)
			e.LastAssistantMessage = `{"instruction_id":"issued","result":"body"}`
			e.AgentType = "custom"
			process(t, h, e)
			files, _ := filepath.Glob(filepath.Join(h.workRoot(), "results", "*.json"))
			if len(files) != 0 {
				t.Fatalf("mismatched result was collected: %v", files)
			}
			e.AgentType = "devkit-other"
			process(t, h, e)
			files, _ = filepath.Glob(filepath.Join(h.workRoot(), "results", "*.json"))
			if len(files) != 0 {
				t.Fatalf("mismatched runner was collected: %v", files)
			}
			// Collection also remains available while work is paused.
			appendPayload(t, h, &ledger.Pause{Position: form.Reference{RecordID: lastRecord(t, h)}})
			e.AgentType = runner
			process(t, h, e)
			data, err := os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-child.json"))
			if err != nil {
				t.Fatal(err)
			}
			var got SubagentResult
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.InstructionID != "issued" || got.Runner != runner || got.Effort != "high" || got.Model != "" || got.ObservedAt.IsZero() || string(got.Content) != e.LastAssistantMessage {
				t.Fatalf("result provenance: %+v", got)
			}
			for _, message := range []string{e.LastAssistantMessage, `{"instruction_id":"issued","result":"changed"}`} {
				e.LastAssistantMessage = message
				process(t, h, e)
				repeated, err := os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-child.json"))
				if err != nil || string(repeated) != string(data) {
					t.Fatalf("first result was replaced: %s, %v", repeated, err)
				}
			}
			e.Host, e.AgentID, e.Model = event.Codex, "codex-child", "observed-model"
			process(t, h, e)
			data, err = os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-codex-child.json"))
			if err != nil {
				t.Fatal(err)
			}
			var codex SubagentResult
			if err := json.Unmarshal(data, &codex); err != nil || codex.Model != "observed-model" || codex.Effort != "" {
				t.Fatalf("Codex provenance: %+v, %v", codex, err)
			}
		})
	}
}

func TestConcurrentSubagentResultCollection(t *testing.T) {
	h := newHandler(t)
	appendPayload(t, h, &ledger.Instruction{ID: "issued", Kind: form.InstructionJudgment, TargetKey: "target", SessionID: "session"})
	state, err := ledger.Load(fsstore.New(h.workRoot()), nil, h.Validator)
	if err != nil {
		t.Fatal(err)
	}
	e := event.Event{Host: event.Codex, Name: event.SubagentStop, AgentID: "child", AgentType: "devkit-judge", LastAssistantMessage: `{"instruction_id":"issued"}`}
	type outcome struct {
		data []byte
		err  error
	}
	const collectors = 8
	start := make(chan struct{})
	outcomes := make(chan outcome, collectors)
	for i := range collectors {
		go func() {
			<-start
			var result outcome
			result.err = h.subagentStop(e, state, time.Unix(int64(i), 0))
			if result.err == nil {
				result.data, result.err = os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-child.json"))
			}
			outcomes <- result
		}()
	}
	close(start)
	var first []byte
	for range collectors {
		result := <-outcomes
		if result.err != nil {
			t.Fatal(result.err)
		}
		if first == nil {
			first = result.data
		} else if string(result.data) != string(first) {
			t.Fatalf("concurrent collection replaced the first result: %s", result.data)
		}
	}
}

func lastRecord(t *testing.T, h Handler) string {
	t.Helper()
	state, err := ledger.Load(fsstore.New(h.workRoot()), nil, h.Validator)
	if err != nil {
		t.Fatal(err)
	}
	return state.LastRecordID()
}

func TestStopLimitPersistsAcrossSessionsAndOnlyRecordsResetIt(t *testing.T) {
	h := newHandler(t)
	for i, session := range []string{"first", "second", "third", "fourth"} {
		result := process(t, h, event.Event{Host: event.Codex, Name: event.Stop, SessionID: session})
		if i < 2 && result.Decision != event.Block || i >= 2 && (result.Decision != "" || !strings.Contains(result.SystemMessage, "Should this work continue?")) {
			t.Fatalf("stop %d: %+v", i+1, result)
		}
		process(t, h, event.Event{Host: event.Codex, Name: event.SessionStart, SessionID: session})
		process(t, h, event.Event{Host: event.Codex, Name: event.PreToolUse, SessionID: session, ToolName: "Read"})
	}
	stops, err := ReadStops(h.workRoot())
	if err != nil || len(stops) != 4 || stops[2].Question == nil || stops[2].Continuations != 3 || stops[3].Continuations != 3 {
		t.Fatalf("stop observations: %+v, %v", stops, err)
	}
	question := *stops[2].Question
	appendPayload(t, h, &ledger.Question{Question: question})
	appendPayload(t, h, &ledger.Answer{QuestionID: question.ID, Target: question.ApprovalTarget, Interpretation: ledger.AnswerApproved, Original: "Continue", SessionID: "third", ObservedAt: time.Now().UTC()})
	if result := process(t, h, event.Event{Name: event.Stop, SessionID: "fifth"}); result.Decision != event.Block {
		t.Fatalf("new records did not reset the counter: %+v", result)
	}
	stops, err = ReadStops(h.workRoot())
	if err != nil || stops[len(stops)-1].Continuations != 1 || stops[len(stops)-1].LimitReached {
		t.Fatalf("reset counter: %+v, %v", stops, err)
	}
}

func TestStopWaitAndPause(t *testing.T) {
	for _, kind := range []form.InstructionKind{form.InstructionQuestion, form.InstructionExternal, form.InstructionJudgmentWait, form.InstructionJudgment, form.InstructionPlanCheck, form.InstructionDetectionTest} {
		t.Run(string(kind), func(t *testing.T) {
			h := newHandler(t)
			appendPayload(t, h, &ledger.Instruction{ID: "issued", Kind: kind, TargetKey: "target", SessionID: "session"})
			if result := process(t, h, event.Event{Name: event.Stop, SessionID: "session"}); result.Decision != "" {
				t.Fatalf("wait instruction was blocked: %+v", result)
			}
		})
	}
	for _, status := range []ledger.WorkStatus{ledger.WorkCompleted, ledger.WorkAborted} {
		h := newHandler(t)
		appendPayload(t, h, &ledger.StateChange{WorkStatus: status})
		if result := process(t, h, event.Event{Name: event.Stop}); result.Decision != "" {
			t.Fatalf("terminal work was blocked: %+v", result)
		}
	}
	h := newHandler(t)
	appendPayload(t, h, &ledger.Pause{Position: form.Reference{RecordID: lastRecord(t, h)}})
	if err := os.MkdirAll(filepath.Join(h.workRoot(), "stops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.workRoot(), "stops", "corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := process(t, h, event.Event{Name: event.Stop}); result.Decision != "" {
		t.Fatalf("paused work was blocked: %+v", result)
	}
	result := process(t, h, event.Event{Name: event.SessionStart, SessionID: "session"})
	if !strings.HasPrefix(result.AdditionalContext, "Warning:") {
		t.Fatalf("SessionStart did not inject its failure: %+v", result)
	}
}
