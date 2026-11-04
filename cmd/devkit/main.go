package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/external"
	"github.com/ildjenty/dev-kit/internal/install"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

const usage = `Usage: devkit <command> [arguments]
  start [--plan <name>] <task>
  next
  submit <file>
  plan <file> | plan --withdraw
  answer [--interpretation approved|rejected|deferred|text] [--text <text>]
         [--observation <basename>] [--supersedes <answer_record_id>] <question_id>
  external --target <target> [--instruction <id>] <kind> <input_file>
  external --recover <operation_id>
  note <decision|evidence|lesson> <file>
  pause [reason]
  resume
  abort
  status [--records | --record <id>]
  view
  init [--yes]
  uninstall [--yes]
  doctor
  version
  hook <event> [--host claude-code|codex]
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devkit:", err)
		os.Exit(1)
	}
}

func sessionID() string {
	if id := os.Getenv("CLAUDE_CODE_SESSION_ID"); id != "" {
		return id
	}
	return os.Getenv("CODEX_THREAD_ID")
}

func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

func parse(f *flag.FlagSet, args []string, min, max int) error {
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return fmt.Errorf("%s", usage)
		}
		return err
	}
	if f.NArg() < min || f.NArg() > max {
		return fmt.Errorf("invalid arguments for %s; run devkit help", f.Name())
	}
	return nil
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(output, usage)
		return err
	}
	name, args := args[0], args[1:]
	f := flags(name)
	if name == "hook" {
		return runHook(ctx, args, input, output)
	}
	if name == "version" {
		if err := parse(f, args, 0, 0); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "devkit", kit.Version)
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if name == "init" || name == "uninstall" || name == "doctor" {
		yes := false
		if name == "init" || name == "uninstall" {
			f.BoolVar(&yes, "yes", false, "Apply changes without confirmation")
		}
		if err := parse(f, args, 0, 0); err != nil {
			return err
		}
		if name == "init" {
			w, err := workspace.Open(ctx, cwd)
			if err != nil {
				return err
			}
			result, err := install.Init(ctx, cwd, func(changes []install.Change) error {
				return confirmInit(changes, filepath.Join(w.Root(), ".devkit/run"), input, output, yes)
			})
			if err != nil {
				return err
			}
			return writeOutput(output, filepath.Join(result.Root, ".devkit/run"), "init.json", result)
		}
		if name == "uninstall" {
			result, err := install.Uninstall(ctx, cwd, func(changes []install.Change) error {
				return confirmUninstall(changes, input, output, yes)
			})
			if err != nil {
				return err
			}
			return writeUninstalled(output, result)
		}
		report, err := install.Doctor(ctx, cwd, sessionID())
		if err != nil {
			return err
		}
		if err := writeOutput(output, filepath.Join(report.Root, ".devkit/run"), "doctor.json", report); err != nil {
			return err
		}
		if !report.Healthy() {
			return errors.New("doctor found issues; review the report")
		}
		return nil
	}
	e, err := newEngine(ctx, cwd, sessionID())
	if err != nil {
		return err
	}
	var result engine.Result
	switch name {
	case "start":
		template := f.String("plan", "default", "Repository plan template")
		if err := parse(f, args, 1, len(args)); err != nil {
			return err
		}
		result, err = e.Start(ctx, engine.StartRequest{Template: *template, Task: strings.Join(f.Args(), " ")})
	case "next", "resume", "abort":
		if err := parse(f, args, 0, 0); err != nil {
			return err
		}
		switch name {
		case "next":
			result, err = e.Next(ctx)
		case "resume":
			result, err = e.Resume(ctx)
		case "abort":
			result, err = e.Abort(ctx)
		}
	case "pause":
		if err := parse(f, args, 0, len(args)); err != nil {
			return err
		}
		result, err = e.Pause(ctx, strings.Join(f.Args(), " "))
	case "submit", "plan", "note":
		withdraw := false
		if name == "plan" {
			f.BoolVar(&withdraw, "withdraw", false, "Withdraw the latest proposal")
		}
		if err := f.Parse(args); err != nil {
			return err
		}
		if withdraw {
			if f.NArg() != 0 {
				return errors.New("plan --withdraw does not accept a file")
			}
			result, err = e.WithdrawPlan(ctx)
			break
		}
		count := 1
		if name == "note" {
			count = 2
		}
		if f.NArg() != count {
			return fmt.Errorf("%s requires %d arguments; run devkit help", name, count)
		}
		data, readErr := os.ReadFile(f.Arg(count - 1))
		if readErr != nil {
			return readErr
		}
		switch name {
		case "submit":
			result, err = e.Submit(ctx, data)
		case "plan":
			result, err = e.Plan(ctx, data)
		case "note":
			result, err = e.Note(ctx, ledger.Kind(f.Arg(0)), data)
		}
	case "answer":
		interpretation := f.String("interpretation", "text", "Interpretation of the observed human input")
		text := f.String("text", "", "Interpreted answer text")
		observation := f.String("observation", "", "Observed prompt basename")
		supersedes := f.String("supersedes", "", "Previous answer record ID")
		if err := parse(f, args, 1, 1); err != nil {
			return err
		}
		result, err = e.Answer(ctx, engine.AnswerRequest{QuestionID: f.Arg(0), Interpretation: ledger.Interpretation(*interpretation), Text: *text, Observation: *observation, Supersedes: *supersedes})
	case "external":
		target := f.String("target", "", "Concrete external target")
		instruction := f.String("instruction", "", "External instruction ID")
		recoverID := f.String("recover", "", "Query an interrupted operation")
		if err := f.Parse(args); err != nil {
			return err
		}
		if *recoverID != "" {
			if f.NArg() != 0 || *target != "" || *instruction != "" {
				return errors.New("external --recover accepts only an operation ID")
			}
			result, err = e.RecoverExternal(ctx, *recoverID)
			break
		}
		if f.NArg() != 2 || *target == "" {
			return errors.New("external requires --target <target> <kind> <input_file>")
		}
		data, readErr := os.ReadFile(f.Arg(1))
		if readErr != nil {
			return readErr
		}
		result, err = e.External(ctx, external.Request{Kind: f.Arg(0), Target: *target, Input: json.RawMessage(data), InstructionID: *instruction})
	case "status", "view":
		return inspect(ctx, e, name, f, args, output)
	default:
		return fmt.Errorf("unknown command %q; run devkit help", name)
	}
	if err != nil {
		return err
	}
	return writeOutput(output, filepath.Join(e.Root(), ".devkit/run/work", result.WorkID), "response.json", result)
}

func confirmInit(changes []install.Change, root string, input io.Reader, output io.Writer, yes bool) error {
	var diff strings.Builder
	count := 0
	for _, change := range changes {
		if !strings.HasPrefix(change.Path, ".claude/") && !strings.HasPrefix(change.Path, ".codex/") {
			continue
		}
		count++
		fmt.Fprintf(&diff, "--- %s (before)\n+++ %s (after)\n", change.Path, change.Path)
		if len(change.Before) != 0 {
			fmt.Fprintln(&diff, "-"+strings.ReplaceAll(strings.TrimSuffix(string(change.Before), "\n"), "\n", "\n-"))
		}
		if len(change.After) != 0 {
			fmt.Fprintln(&diff, "+"+strings.ReplaceAll(strings.TrimSuffix(string(change.After), "\n"), "\n", "\n+"))
		}
	}
	if diff.Len() > 3000 {
		if err := fsstore.New(root).Publish("init.diff", []byte(diff.String())); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(output, "Initialization changes %d host files. Full diff: %s\n", count, filepath.Join(root, "init.diff")); err != nil {
			return err
		}
	} else if _, err := io.WriteString(output, diff.String()); err != nil {
		return err
	}
	if yes {
		return nil
	}
	if _, err := fmt.Fprint(output, "Apply initialization changes? [y/N] "); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	if scanner.Scan() {
		answer := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("initialization canceled")
}

// confirmUninstall lists the removals and shows the edits to settings files.
// Paths under the kit copy and runtime state are summarized per directory.
func confirmUninstall(changes []install.Change, input io.Reader, output io.Writer, yes bool) error {
	var text strings.Builder
	summarized := map[string]int{}
	for _, change := range changes {
		if dir, ok := summarizedDirectory(change.Path); ok {
			summarized[dir]++
			continue
		}
		if change.Remove {
			fmt.Fprintf(&text, "remove %s\n", change.Path)
			continue
		}
		fmt.Fprintf(&text, "--- %s (before)\n+++ %s (after)\n", change.Path, change.Path)
		fmt.Fprintln(&text, "-"+strings.ReplaceAll(strings.TrimSuffix(string(change.Before), "\n"), "\n", "\n-"))
		if len(change.After) != 0 {
			fmt.Fprintln(&text, "+"+strings.ReplaceAll(strings.TrimSuffix(string(change.After), "\n"), "\n", "\n+"))
		}
	}
	for _, dir := range []string{".devkit/kit/", ".devkit/run/"} {
		if n := summarized[dir]; n != 0 {
			fmt.Fprintf(&text, "remove %s (%d files)\n", dir, n)
		}
	}
	if _, err := io.WriteString(output, text.String()); err != nil {
		return err
	}
	if yes {
		return nil
	}
	if _, err := fmt.Fprint(output, "Apply uninstall changes? [y/N] "); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	if scanner.Scan() {
		answer := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("uninstall canceled")
}

func summarizedDirectory(path string) (string, bool) {
	for _, dir := range []string{".devkit/kit/", ".devkit/run/"} {
		if strings.HasPrefix(path, dir) {
			return dir, true
		}
	}
	return "", false
}

// writeUninstalled prints a plain summary because the runtime directory no longer exists.
func writeUninstalled(output io.Writer, result install.Result) error {
	if _, err := fmt.Fprintf(output, "Uninstalled devkit from %s (%d changes).\n", result.Root, len(result.Changes)); err != nil {
		return err
	}
	if len(result.Preserved) != 0 {
		if _, err := fmt.Fprintln(output, "Left in place because they differ from the kit or hold your settings:"); err != nil {
			return err
		}
		for _, path := range result.Preserved {
			if _, err := fmt.Fprintln(output, "  "+path); err != nil {
				return err
			}
		}
	}
	for _, guidance := range result.Guidance {
		if _, err := fmt.Fprintln(output, guidance); err != nil {
			return err
		}
	}
	return nil
}

func writeOutput(output io.Writer, root string, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 3000 {
		if err := fsstore.New(root).Publish(name, append(data, '\n')); err != nil {
			return err
		}
		switch value := value.(type) {
		case engine.Result:
			_, err = fmt.Fprintf(output, "Work %s: %s; paused=%t.\n", value.WorkID, value.Status, value.Paused)
		case engine.Status:
			_, err = fmt.Fprintf(output, "Work %s: %s; paused=%t.\n", value.WorkID, value.Position.Status, value.Position.Paused)
		case install.Result:
			if _, err := fmt.Fprintf(output, "Initialized %s (%d changes).\n", value.Root, len(value.Changes)); err != nil {
				return err
			}
			for _, guidance := range value.Guidance {
				if _, err := fmt.Fprintln(output, guidance); err != nil {
					return err
				}
			}
		case install.Report:
			_, err = fmt.Fprintf(output, "Doctor: healthy=%t.\n", value.Healthy())
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Full output:", filepath.Join(root, name))
		return err
	}
	_, err = fmt.Fprintln(output, string(data))
	return err
}
