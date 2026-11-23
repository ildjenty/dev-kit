package refine

import (
	"fmt"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/codexcli"
	"github.com/ildjenty/dev-kit/internal/jev"
)

const reflectionSchema = `{"type":"object","additionalProperties":false,"properties":{"not_reflected":{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"decision":{"type":"string","minLength":1},"documents":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"reason":{"type":"string","minLength":1}},"required":["decision","documents","reason"]}}},"required":["not_reflected"]}`

func runReflect(args []string, env cli.Env) error {
	return runReflectTimeout(env, codexcli.Timeout)
}

func runReflectTimeout(env cli.Env, timeout time.Duration) error {
	artifacts := env.Path(env.Getenv("ARTIFACTS_DIR"))
	result := reflectJev(env.Cwd, artifacts, jev.New(env))
	if err := writeJSON(file(artifacts, "reflect-jev.json"), result); err != nil && result["available"] == true {
		result = reflectUnavailable("Could not write Jev reflection artifact")
	}
	delete(result, "probabilities")
	if result["available"] == true {
		delete(result, "available")
		result["source"] = "jev"
		return env.JSON(result)
	}
	answer, err := codexcli.Run(env, "refine", "refine/commands/reflect.md", reflectionSchema, timeout)
	if err != nil {
		return fmt.Errorf("Jev unavailable: %s; Codex unavailable: %s", str(result["reason"]), err)
	}
	answer["source"] = "codex"
	return env.JSON(answer)
}
