package retro

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/judgeeval"
	"github.com/ildjenty/dev-kit/internal/record"
)

func reviewHash(entry doc) string {
	b, _ := json.Marshal(entry)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func reviewedErrors(ctx record.ContextData, artifacts string, entry doc) ([]string, error) {
	errors, e := reviewedDraftErrors(ctx, artifacts, entry)
	if e != nil {
		return nil, e
	}
	errors = append(stringsOf(entry["errors"]), errors...)
	approval, e := os.ReadFile(file(artifacts, "approval.txt"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	if string(approval) != str(entry["approval_text"]) {
		errors = append(errors, "approval question changed after the judgment")
	}
	return errors, nil
}

func verify(ctx record.ContextData, artifacts string, env cli.Env) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	entry, e := readJSON(file(artifacts, "rounds/latest.json"))
	if e != nil {
		return nil, e
	}
	errors, e := reviewedErrors(ctx, artifacts, entry)
	if e != nil {
		return nil, e
	}
	cases := items(obj(entry["draft"])["cases"])
	if entry["cleared"] != true || len(errors) > 0 || len(cases) == 0 {
		return doc{"needed": false}, nil
	}
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	temp, e := os.MkdirTemp("", "devkit-retro-standards-*")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(temp)
	standards := str(obj(input["folders"])["standards"])
	if e = os.CopyFS(temp, os.DirFS(filepath.Join(ctx.Cwd, standards))); e != nil {
		return nil, e
	}
	out := file(artifacts, "evaluation")
	if e = os.MkdirAll(out, 0755); e != nil {
		return nil, e
	}
	log, e := os.Create(filepath.Join(out, "run.log"))
	if e != nil {
		return nil, e
	}
	env.Cwd, env.Stdout, env.Stderr = ctx.Cwd, log, log
	args := []string{"eval", "judge", "--dir", filepath.Join(ctx.Cwd, caseFolder(input)), "--standards", temp, "--out", out}
	for _, c := range cases {
		args = append(args, "--case", str(c["name"]))
	}
	runError := cli.New("retro", judgeeval.Commands()).Run(args, env)
	closed := log.Close()
	if closed != nil {
		return nil, closed
	}
	verification := doc{"review": reviewHash(entry), "result": nil, "error": "", "log": filepath.Join(out, "run.log")}
	if runError != nil {
		verification["error"] = runError.Error()
	} else {
		result, e := readJSON(filepath.Join(out, "result.json"))
		if e != nil {
			return nil, e
		}
		verification["result"] = result
	}
	if e = writeJSON(file(artifacts, "verification.json"), verification); e != nil {
		return nil, e
	}
	return doc{"needed": true}, nil
}
