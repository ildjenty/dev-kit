package engine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/install"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func runtimeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		files[path] = ""
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[path] = "symlink:" + target
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func refuseStart(t *testing.T, e *Engine, ctx context.Context, work *ledger.Work) {
	t.Helper()
	before := runtimeFiles(t, e.runRoot())
	refs := git(t, e.Root(), "show-ref")
	branch := git(t, e.Root(), "branch", "--show-current")
	index := git(t, e.Root(), "ls-files", "--stage")
	if _, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Another task"}); err == nil {
		t.Fatal("start accepted a checkout with unfinished work")
	} else {
		for _, text := range []string{work.ID, work.Branch, "separate worktree", "git worktree add <path> -b <branch>", "new session"} {
			if !strings.Contains(err.Error(), text) {
				t.Fatalf("start refusal omitted %q: %v", text, err)
			}
		}
	}
	if !reflect.DeepEqual(before, runtimeFiles(t, e.runRoot())) || refs != git(t, e.Root(), "show-ref") || branch != git(t, e.Root(), "branch", "--show-current") || index != git(t, e.Root(), "ls-files", "--stage") {
		t.Fatal("refused start changed runtime records, refs, branch, or index")
	}
}

func TestStartRefusesActiveAndPausedCheckout(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "active"
		if paused {
			name = "paused"
		}
		t.Run(name, func(t *testing.T) {
			e, ctx := fixture(t)
			if _, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"}); err != nil {
				t.Fatal(err)
			}
			if paused {
				if _, err := e.Pause(ctx, "Wait for the human"); err != nil {
					t.Fatal(err)
				}
			}
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			work, _ := state.Work()
			refuseStart(t, e, ctx, work)
			git(t, e.Root(), "switch", "-c", "another-branch")
			other, err := newTestEngine(ctx, e.Root(), "another-session")
			if err != nil {
				t.Fatal(err)
			}
			// Refusal must not depend on session binding, current branch, or forward assets.
			write(t, e.Root(), "devkit.toml", "Invalid configuration\n")
			refuseStart(t, other, ctx, work)
		})
	}
}

func TestStartAllowsEndedWorks(t *testing.T) {
	for _, ended := range []ledger.WorkStatus{ledger.WorkCompleted, ledger.WorkAborted} {
		t.Run(string(ended), func(t *testing.T) {
			e, ctx := fixture(t)
			work := working(t, e, ctx)
			if ended == ledger.WorkCompleted {
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
					t.Fatal(err)
				}
				if result, err := e.Next(ctx); err != nil || result.Status != ended {
					t.Fatalf("work did not complete: %+v, %v", result, err)
				}
			} else if _, err := e.Abort(ctx); err != nil {
				t.Fatal(err)
			}
			oldID := e.workID
			if result, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Another task"}); err != nil || result.WorkID == oldID {
				t.Fatalf("ended work blocked a new start: %+v, %v", result, err)
			}
		})
	}
}

func TestConcurrentStartsUseOneCheckout(t *testing.T) {
	e, ctx := fixture(t)
	other, err := newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	newWork := e.newWork
	e.newWork = func() (string, string) {
		close(entered)
		<-release
		return newWork()
	}
	done := make(chan error, 1)
	go func() {
		_, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
		done <- err
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("first start failed before creating work: %v", err)
	}
	_, secondErr := other.Start(ctx, StartRequest{Pattern: "test", Task: "Another task"})
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if secondErr == nil {
		t.Fatal("two concurrent starts shared one checkout")
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	work, _ := state.Work()
	refuseStart(t, other, ctx, work)
}

func TestParallelWorktreeLoops(t *testing.T) {
	main, ctx := fixture(t)
	write(t, main.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n"+sessionWorker))
	configureStage(t, main, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("plan", true)}
	})
	base := git(t, main.Root(), "rev-parse", "HEAD")
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	git(t, main.Root(), "worktree", "add", "-b", "linked-session", linkedRoot)
	t.Cleanup(func() {
		if err := removeExpansion(filepath.Join(linkedRoot, ".devkit/run/work")); err != nil {
			t.Error(err)
		}
	})
	info, err := os.Stat(filepath.Join(linkedRoot, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("linked worktree must have a git file: %v, %v", info, err)
	}
	h := hook.Handler{Root: linkedRoot}
	if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.SessionStart, SessionID: "session-two"}); err != nil {
		t.Fatal(err)
	}
	if report, err := install.Doctor(ctx, linkedRoot, "session-two"); err != nil || !report.Healthy() {
		t.Fatalf("doctor rejected a fresh linked worktree: %+v, %v", report.Checks, err)
	}
	linked, err := newTestEngine(ctx, linkedRoot, "session-two")
	if err != nil {
		t.Fatal(err)
	}
	engines := []*Engine{main, linked}
	results := make([]Result, len(engines))
	works := make([]*ledger.Work, len(engines))
	commits := make([]string, len(engines))
	for i, e := range engines {
		observeAnswer(t, e, "Original request for "+e.sessionID)
		results[i], err = e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
		if err != nil {
			t.Fatal(err)
		}
		state, err := e.load()
		if err != nil {
			t.Fatal(err)
		}
		works[i], _ = state.Work()
		if works[i].OriginalRequest == nil || *works[i].OriginalRequest != "Original request for "+e.sessionID || works[i].BaseCommit != base {
			t.Fatalf("work lost its checkout or session identity: %+v", works[i])
		}
		refuseStart(t, e, ctx, works[i])
		planResult(t, e, results[i])
	}
	if works[0].ID == works[1].ID || works[0].Branch == works[1].Branch || main.runRoot() == linked.runRoot() {
		t.Fatal("worktrees shared work, branch, or runtime roots")
	}
	for _, kind := range []form.InstructionKind{form.InstructionDetectionTest, form.InstructionPlanCheck, form.InstructionQuestion, form.InstructionWork} {
		for i, e := range engines {
			results[i], err = e.Next(ctx)
			if err != nil || results[i].Instruction == nil || results[i].Instruction.Kind != kind {
				t.Fatalf("%s expected %s: %+v, %v", e.sessionID, kind, results[i], err)
			}
			switch kind {
			case form.InstructionDetectionTest:
				acceptDetectionResult(t, e, results[i].Instruction)
			case form.InstructionPlanCheck:
				data, err := e.readInstruction(results[i].Instruction.ID)
				if err != nil {
					t.Fatal(err)
				}
				collect(t, e, results[i].Instruction, pass(*data.Request))
			case form.InstructionQuestion:
				question := results[i].Instruction.Question
				if question == nil || question.StopCategory != "plan_approval" {
					t.Fatalf("expected plan approval: %+v", results[i])
				}
				observeAnswer(t, e, "Approve this plan.")
				if _, err := e.Answer(ctx, AnswerRequest{QuestionID: question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
					t.Fatal(err)
				}
			case form.InstructionWork:
				write(t, e.Root(), "README.md", "# Documentation for "+e.sessionID+"\n")
				indexPath := git(t, e.Root(), "rev-parse", "--path-format=absolute", "--git-path", "index")
				before, err := os.ReadFile(indexPath)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: results[i].Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(indexPath)
				if err != nil || string(before) != string(after) {
					t.Fatalf("snapshot changed the worktree index: %v", err)
				}
			}
		}
	}
	for _, e := range engines {
		result, err := e.Next(ctx)
		if err != nil || result.Instruction == nil || result.Instruction.Kind != form.InstructionJudgment {
			t.Fatalf("expected stage judgment: %+v, %v", result, err)
		}
		data, err := e.readInstruction(result.Instruction.ID)
		if err != nil {
			t.Fatal(err)
		}
		collect(t, e, result.Instruction, pass(*data.Request))
	}
	for i, e := range engines {
		result, err := e.Next(ctx)
		if err != nil || result.Status != ledger.WorkCompleted {
			t.Fatalf("worktree loop did not complete: %+v, %v", result, err)
		}
		state, err := e.load()
		if err != nil {
			t.Fatal(err)
		}
		run, ok := state.LatestStageRun()
		if !ok || run.State != ledger.StageCompleted || run.Commit == "" {
			t.Fatalf("stage completion has no commit: %+v", run)
		}
		commits[i] = run.Commit
		if git(t, e.Root(), "rev-parse", works[i].Branch) != run.Commit || git(t, e.Root(), "show", run.Commit+":README.md") != "# Documentation for "+e.sessionID {
			t.Fatal("completion commit escaped its worktree branch")
		}
		if i == 0 && git(t, e.Root(), "for-each-ref", "--format=%(refname)", "refs/devkit/"+linked.workID+"/") == "" {
			t.Fatal("completing one work removed the other's snapshot refs")
		}
		entries, err := os.ReadDir(filepath.Join(e.runRoot(), "work"))
		if err != nil || len(entries) != 1 || entries[0].Name() != e.workID {
			t.Fatalf("records are not isolated: %v, %v", entries, err)
		}
		if report, err := install.Doctor(ctx, e.Root(), e.sessionID); err != nil || !report.Healthy() {
			t.Fatalf("doctor failed in a completed worktree: %+v, %v", report.Checks, err)
		}
		observation, err := hook.ReadSession(e.runRoot(), e.sessionID)
		if err != nil || observation == nil || observation.WorkID != e.workID {
			t.Fatalf("hook binding escaped the worktree: %+v, %v", observation, err)
		}
	}
	if commits[0] == commits[1] || git(t, main.Root(), "merge-base", commits[0], commits[1]) != base {
		t.Fatal("completion commits did not diverge from their shared base")
	}
	for _, commit := range commits {
		if git(t, linked.Root(), "cat-file", "-t", commit) != "commit" {
			t.Fatal("a completion commit is missing from the shared repository")
		}
	}
}
