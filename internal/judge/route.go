package judge

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ildjenty/dev-kit/internal/claudecli"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/codexcli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/jev"
)

func runRoute(args []string, env cli.Env) error {
	return runRouteTimeout(env, codexcli.Timeout)
}

func runRouteTimeout(env cli.Env, timeout time.Duration) error {
	if err := env.Require("ARTIFACTS_DIR"); err != nil {
		return err
	}
	artifacts := env.Path(env.Getenv("ARTIFACTS_DIR"))
	result := routeJevClient(artifacts, jev.New(env))
	if err := save(filepath.Join(artifacts, "judge/route-jev.json"), result); err != nil {
		return err
	}
	delete(result, "probabilities")
	if truth(result["available"]) {
		delete(result, "available")
		result["source"] = "jev"
		return env.JSON(result)
	}
	target, err := read(filepath.Join(artifacts, "judge/target.json"))
	if err != nil {
		return err
	}
	mapping, err := concerns.Load(filepath.Join(artifacts, "criteria", text(target["standards"])))
	if err != nil {
		return err
	}
	changes, err := read(filepath.Join(artifacts, "judge/changes.json"))
	if err != nil {
		return err
	}
	paths := []string{}
	for _, f := range docs(changes["files"]) {
		paths = append(paths, text(f["path"]))
		if old := text(f["oldPath"]); old != "" {
			paths = append(paths, old)
		}
	}
	schema, err := json.Marshal(doc{"type": "object", "additionalProperties": false, "properties": doc{
		"qualities": doc{"type": "array", "items": doc{"type": "object", "additionalProperties": false, "properties": doc{
			"quality": doc{"type": "string", "enum": mapping.Judged("code")}, "file": doc{"type": "string", "enum": paths}, "reason": doc{"type": "string", "minLength": 1},
		}, "required": []string{"quality", "file", "reason"}}},
	}, "required": []string{"qualities"}})
	if err != nil {
		return err
	}
	settings, err := config.Load(env.Cwd)
	if err != nil {
		return err
	}
	runCLI := claudecli.Run
	label := "Claude"
	if settings.Provider == "codex" {
		runCLI = codexcli.Run
		label = "Codex"
	}
	answer, err := runCLI(env, "judge", "judge/commands/route.md", string(schema), timeout, text(target["tree"]))
	if err == nil {
		answer["source"] = settings.Provider
		return env.JSON(answer)
	}
	reason := fmt.Sprintf("Jev unavailable: %s; %s unavailable: %s. Select every routable quality: an extra judge is cheaper than a missed one.", text(result["reason"]), label, err)
	selected := []doc{}
	for _, f := range docs(changes["files"]) {
		for _, q := range mapping.Judged("code") {
			selected = append(selected, doc{"quality": q, "file": f["path"], "reason": reason})
		}
	}
	return env.JSON(doc{"source": "all", "reason": reason, "qualities": selected})
}
