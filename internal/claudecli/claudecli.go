package claudecli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func Run(env cli.Env, area, promptPath, schema string, timeout time.Duration, readDirs ...string) (map[string]any, error) {
	prompt, err := os.ReadFile(env.Path(filepath.Join(env.Getenv("PACK_DIR"), promptPath)))
	if err != nil {
		return nil, fmt.Errorf("could not read Claude prompt")
	}
	text := strings.NewReplacer("$ARTIFACTS_DIR", env.Path(env.Getenv("ARTIFACTS_DIR")), "$PACK_DIR", env.Path(env.Getenv("PACK_DIR"))).Replace(string(prompt))
	var source any
	if err = json.Unmarshal([]byte(schema), &source); err != nil {
		return nil, fmt.Errorf("invalid Claude output schema")
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://devkit.invalid/claude-output.json"
	if err = compiler.AddResource(url, source); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "devkit-claude-*")
	if err != nil {
		return nil, fmt.Errorf("could not create Claude temporary directory")
	}
	defer os.RemoveAll(dir)
	answerPath := filepath.Join(dir, "answer.json")

	logDir := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), area))
	if err = os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create Claude log directory")
	}
	log, err := os.CreateTemp(logDir, "claude-*.log")
	if err != nil {
		return nil, fmt.Errorf("could not create Claude log")
	}
	defer log.Close()
	redact := func(s string) string {
		if key := env.Getenv("TYPESAFE_API_KEY"); key != "" {
			return strings.ReplaceAll(s, key, "[redacted]")
		}
		return s
	}
	fmt.Fprintln(env.Stderr, redact("Claude log: "+log.Name()))
	executable := env.Getenv("DEVKIT_CLAUDE")
	if executable == "" {
		executable = "claude"
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := []string{"-p", "--json-schema", schema, "--allowedTools", "Read,Grep,Glob", "--tools", "Read,Grep,Glob", "--permission-mode", "dontAsk", "--output-format", "json"}
	for _, dir := range readDirs {
		if dir != "" {
			args = append(args, "--add-dir", dir)
		}
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = env.Cwd
	cmd.Env = env.Environ
	cmd.Stdin = strings.NewReader(text)
	answerFile, err := os.Create(answerPath)
	if err != nil {
		return nil, fmt.Errorf("could not create Claude answer file")
	}
	defer answerFile.Close()
	cmd.Stdout, cmd.Stderr = io.MultiWriter(answerFile, log), log
	cmd.WaitDelay = 100 * time.Millisecond
	err = cmd.Run()
	reason := ""
	if ctx.Err() != nil {
		reason = "Claude timed out"
	} else if err != nil {
		reason = "Claude unavailable: " + redact(err.Error())
	}
	var answer map[string]any
	if reason == "" {
		data, readErr := os.ReadFile(answerPath)
		var result struct {
			Type             string         `json:"type"`
			Subtype          string         `json:"subtype"`
			IsError          bool           `json:"is_error"`
			StructuredOutput map[string]any `json:"structured_output"`
		}
		decodeErr := json.Unmarshal(data, &result)
		answer = result.StructuredOutput
		if readErr != nil || decodeErr != nil || result.Type != "result" || result.Subtype != "success" || result.IsError || compiled.Validate(answer) != nil {
			reason = "Claude answer does not match output schema"
		}
	}
	if reason != "" {
		if size, statErr := log.Stat(); statErr == nil {
			if _, seekErr := log.Seek(max(0, size.Size()-16000), io.SeekStart); seekErr == nil {
				data, _ := io.ReadAll(log)
				tail := []rune(redact(string(data)))
				fmt.Fprintln(env.Stderr, string(tail[max(0, len(tail)-4000):]))
			}
		}
		return nil, fmt.Errorf("%s", reason)
	}
	data, err := json.Marshal(answer)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(redact(string(data))), &answer); err != nil {
		return nil, err
	}
	return answer, nil
}
