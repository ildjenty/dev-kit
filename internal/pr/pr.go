package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
)

type doc = record.Document

func object(v any) doc  { d, _ := v.(map[string]any); return d }
func text(v any) string { s, _ := v.(string); return s }
func truth(v any) bool  { b, _ := v.(bool); return b }
func docs(v any) []doc {
	if d, ok := v.([]doc); ok {
		return d
	}
	out := []doc{}
	if a, ok := v.([]any); ok {
		for _, v := range a {
			out = append(out, object(v))
		}
	}
	return out
}
func stringsOf(v any) []string {
	if a, ok := v.([]string); ok {
		return a
	}
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, v := range a {
			out = append(out, text(v))
		}
	}
	return out
}
func clone(d doc) doc {
	out := doc{}
	for k, v := range d {
		out[k] = v
	}
	return out
}
func read(file string) (doc, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var d doc
	err = json.Unmarshal(b, &d)
	return d, err
}
func optional(file string) (doc, error) {
	d, err := read(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return d, err
}
func save(file string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}
func jsonString(v any) (string, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return "", err
	}
	encoded := bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
	var result bytes.Buffer
	for i := 0; i < len(encoded); i++ {
		if encoded[i] == '\\' && i+1 < len(encoded) {
			if i+5 < len(encoded) && (string(encoded[i+1:i+6]) == "u2028" || string(encoded[i+1:i+6]) == "u2029") {
				if encoded[i+5] == '8' {
					result.WriteRune('\u2028')
				} else {
					result.WriteRune('\u2029')
				}
				i += 5
			} else {
				result.Write(encoded[i : i+2])
				i++
			}
		} else {
			result.WriteByte(encoded[i])
		}
	}
	return result.String(), nil
}
func context(env cli.Env) (record.ContextData, error) {
	ctx, err := record.Context(env)
	ctx.Dir = env.Path(ctx.Dir)
	return ctx, err
}
func stopDoc(state string, unresolved []string) doc {
	return doc{"outcome": "stop", "stop": doc{"state": state, "unresolved": unresolved, "premise": ""}}
}
func Commands() []cli.Command {
	out := []cli.Command{{Area: "pr", Name: "answers", Usage: "devkit pr answers", Run: answers}, {Area: "pr", Name: "facts", Usage: "devkit pr facts", Run: facts}, {Area: "pr", Name: "open", Usage: "devkit pr open", Run: openPR}, {Area: "pr", Name: "finish", Usage: "devkit pr finish", Run: finish}, {Area: "pr", Name: "output", Usage: "devkit pr output <not-started> <result> <pr>", Run: output}}
	for _, mode := range []string{"assemble", "judgment", "state"} {
		out = append(out, cli.Command{Area: "pr", Name: mode, Usage: "devkit pr " + mode, Run: func(args []string, env cli.Env) error { return report(mode, env) }})
	}
	return out
}

var permission = regexp.MustCompile(`^(allow|deny)\b`)

func parsePermission(answer string) (bool, error) {
	match := permission.FindStringSubmatch(answer)
	if match == nil {
		return false, &record.RecordError{Message: "a change answer must begin with allow or deny"}
	}
	return match[1] == "allow", nil
}
func changeAnswers(dir, runID string) ([]doc, error) {
	h, err := record.History(dir)
	if err != nil {
		return nil, err
	}
	var run doc
	for _, r := range h.Runs {
		if r["run_id"] == runID {
			run = r
			break
		}
	}
	if run == nil {
		return nil, &record.RecordError{Message: fmt.Sprintf("run %s did not pass the start conditions", runID)}
	}
	out := []doc{}
	for _, id := range stringsOf(run["answers"]) {
		q, err := record.ReadKeyed(dir, "question", id)
		if err != nil {
			return nil, err
		}
		a, err := record.ReadKeyed(dir, "answer", id)
		if err != nil {
			return nil, err
		}
		if q["workflow"] != "pr" || object(q["target"])["kind"] != "change" {
			continue
		}
		var change doc
		if err = json.Unmarshal([]byte(text(object(q["target"])["ref"])), &change); err != nil {
			return nil, err
		}
		out = append(out, doc{"question_id": q["id"], "change": change, "answer": a["answer"]})
	}
	return out, nil
}
func recordAnswers(ctx record.ContextData, answered []doc, now string) (int, []doc, error) {
	decisions := []doc{}
	denied := []doc{}
	for _, a := range answered {
		allows, err := parsePermission(text(a["answer"]))
		if err != nil {
			return 0, nil, err
		}
		d := clone(a)
		d["allows"] = allows
		decisions = append(decisions, d)
		if !allows {
			denied = append(denied, d)
		}
	}
	for _, a := range decisions {
		if !truth(a["allows"]) {
			continue
		}
		body := clone(object(a["change"]))
		body["question_id"] = a["question_id"]
		body["value"] = body["item"]
		if _, err := record.Write(ctx, "allowed", body, now); err != nil {
			return 0, nil, err
		}
	}
	return len(decisions) - len(denied), denied, nil
}
func answers(args []string, env cli.Env) error {
	ctx, err := context(env)
	if err != nil {
		return err
	}
	answered, err := changeAnswers(ctx.Dir, ctx.RunID)
	if err != nil {
		return err
	}
	allowed, denied, err := recordAnswers(ctx, answered, "")
	if err != nil {
		return err
	}
	ids := []string{}
	unresolved := []string{}
	for _, d := range denied {
		ids = append(ids, text(d["question_id"]))
		unresolved = append(unresolved, text(d["question_id"])+": "+text(object(d["change"])["file"]))
	}
	if len(denied) > 0 {
		out := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr"))
		if err = os.MkdirAll(out, 0755); err != nil {
			return err
		}
		if err = save(filepath.Join(out, "outcome.json"), stopDoc("The owner denied a change; remove that change before opening the pull request", unresolved)); err != nil {
			return err
		}
	}
	return env.JSON(doc{"allowed_count": allowed, "denied_count": len(denied), "denied": ids})
}
func output(args []string, env cli.Env) error {
	if len(args) != 3 {
		return cli.Usage("devkit pr output <not-started> <result> <pr>")
	}
	out := doc{}
	for i, key := range []string{"not_started", "result", "pr"} {
		var value any
		if args[i] != "" {
			if err := json.Unmarshal([]byte(args[i]), &value); err != nil {
				return err
			}
		}
		out[key] = value
	}
	return env.JSON(out)
}
