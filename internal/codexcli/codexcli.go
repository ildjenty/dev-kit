package codexcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const Timeout = 10 * time.Minute

func Run(env cli.Env, area, promptPath, schema string, timeout time.Duration, readDirs ...string) (map[string]any, error) {
	if err := env.Require("ARTIFACTS_DIR", "PACK_DIR"); err != nil {
		return nil, err
	}
	prompt, err := os.ReadFile(env.Path(filepath.Join(env.Getenv("PACK_DIR"), promptPath)))
	if err != nil {
		return nil, fmt.Errorf("could not read Codex prompt")
	}
	text := strings.NewReplacer("$ARTIFACTS_DIR", env.Path(env.Getenv("ARTIFACTS_DIR")), "$PACK_DIR", env.Path(env.Getenv("PACK_DIR"))).Replace(string(prompt))
	if quality := env.Getenv("QUALITY"); quality != "" {
		text = strings.ReplaceAll(text, "$INPUTS.quality", quality)
	}
	var source any
	if err = json.Unmarshal([]byte(schema), &source); err != nil {
		return nil, fmt.Errorf("invalid Codex output schema")
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://devkit.invalid/codex-output.json"
	if err = compiler.AddResource(url, source); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "devkit-codex-*")
	if err != nil {
		return nil, fmt.Errorf("could not create Codex temporary directory")
	}
	defer os.RemoveAll(dir)
	schemaPath, answerPath := filepath.Join(dir, "schema.json"), filepath.Join(dir, "answer.json")
	if err = os.WriteFile(schemaPath, []byte(schema), 0600); err != nil {
		return nil, fmt.Errorf("could not write Codex output schema")
	}
	logDir := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), area))
	if err = os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create Codex log directory")
	}
	log, err := os.CreateTemp(logDir, "codex-*.log")
	if err != nil {
		return nil, fmt.Errorf("could not create Codex log")
	}
	defer log.Close()
	redact := func(s string) string {
		if key := env.Getenv("TYPESAFE_API_KEY"); key != "" {
			return strings.ReplaceAll(s, key, "[redacted]")
		}
		return s
	}
	fmt.Fprintln(env.Stderr, redact("Codex log: "+log.Name()))
	executable := env.Getenv("DEVKIT_CODEX")
	if executable == "" {
		executable = "codex"
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cwd := env.Cwd
	if len(readDirs) > 0 && readDirs[0] != "" {
		cwd = readDirs[0]
	}
	cmd := exec.CommandContext(ctx, executable, "exec", "--sandbox", "read-only", "--cd", cwd, "--output-schema", schemaPath, "-o", answerPath, "--skip-git-repo-check", "-")
	cmd.Dir = cwd
	childEnv := env.Environ
	if childEnv == nil {
		childEnv = cmd.Environ()
	}
	cmd.Env = make([]string, 0, len(childEnv))
	for _, entry := range childEnv {
		if key, _, _ := strings.Cut(entry, "="); key != "TYPESAFE_API_KEY" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Stdin = strings.NewReader(text)
	logWriter := &redactedWriter{output: log, key: env.Getenv("TYPESAFE_API_KEY")}
	cmd.Stdout, cmd.Stderr = logWriter, logWriter
	cmd.WaitDelay = 100 * time.Millisecond
	err = cmd.Run()
	if flushErr := logWriter.flush(); err == nil {
		err = flushErr
	}
	reason := ""
	if ctx.Err() != nil {
		reason = "Codex timed out"
	} else if err != nil {
		reason = "Codex unavailable: " + redact(err.Error())
	}
	var answer map[string]any
	if reason == "" {
		data, readErr := os.ReadFile(answerPath)
		if readErr != nil || json.Unmarshal(data, &answer) != nil || compiled.Validate(answer) != nil {
			reason = "Codex answer does not match output schema"
		}
	}
	if reason == "" {
		redacted, redactErr := redactJSON(answer, redact)
		if redactErr != nil || compiled.Validate(redacted) != nil {
			reason = "Codex answer does not match output schema"
		} else {
			answer = redacted.(map[string]any)
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
	return answer, nil
}

type redactedWriter struct {
	mu      sync.Mutex
	output  io.Writer
	key     string
	pending string
}

func (w *redactedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.key == "" {
		return w.output.Write(p)
	}
	data := w.pending + string(p)
	var safe strings.Builder
	for {
		index := strings.Index(data, w.key)
		if index < 0 {
			break
		}
		safe.WriteString(data[:index])
		safe.WriteString("[redacted]")
		data = data[index+len(w.key):]
	}
	keep := min(len(data), len(w.key)-1)
	for keep > 0 && !strings.HasPrefix(w.key, data[len(data)-keep:]) {
		keep--
	}
	safe.WriteString(data[:len(data)-keep])
	w.pending = data[len(data)-keep:]
	if _, err := io.WriteString(w.output, safe.String()); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *redactedWriter) flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := io.WriteString(w.output, w.pending)
	w.pending = ""
	return err
}

func redactJSON(value any, redact func(string) string) (any, error) {
	switch value := value.(type) {
	case string:
		return redact(value), nil
	case []any:
		for i, item := range value {
			redacted, err := redactJSON(item, redact)
			if err != nil {
				return nil, err
			}
			value[i] = redacted
		}
	case map[string]any:
		if value == nil {
			return value, nil
		}
		result := make(map[string]any, len(value))
		for key, item := range value {
			key = redact(key)
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("redacted property names collide")
			}
			redacted, err := redactJSON(item, redact)
			if err != nil {
				return nil, err
			}
			result[key] = redacted
		}
		return result, nil
	}
	return value, nil
}
