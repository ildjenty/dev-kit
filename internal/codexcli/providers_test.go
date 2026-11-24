package codexcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVKIT_PROVIDER_HELPER") != "1" {
		os.Exit(m.Run())
	}
	if _, exists := os.LookupEnv("TYPESAFE_API_KEY"); exists {
		os.Exit(21)
	}
	if os.Getenv("OPENAI_API_KEY") != "fake-openai-auth" || os.Getenv("ANTHROPIC_API_KEY") != "fake-anthropic-auth" {
		os.Exit(22)
	}
	switch os.Getenv("DEVKIT_CODEX_STDIN_HOLDER") {
	case "hold":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "spawn":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "DEVKIT_CODEX_STDIN_HOLDER=hold")
		child.Stdin = os.Stdin
		child.Stdout, child.Stderr = io.Discard, io.Discard
		if err := child.Start(); err != nil {
			os.Exit(26)
		}
		if err := os.WriteFile("holder.pid", []byte(fmt.Sprint(child.Process.Pid)), 0600); err != nil {
			child.Process.Kill()
			os.Exit(27)
		}
		os.Exit(0)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		os.Exit(23)
	}
	if os.Getenv("DEVKIT_PROVIDER_FAIL") == "1" {
		fmt.Fprint(os.Stderr, "HEAD"+strings.Repeat("界", 5000)+os.Getenv("DEVKIT_PROVIDER_LOG")+"TAIL")
		os.Exit(9)
	}
	diagnostic := os.Getenv("DEVKIT_PROVIDER_LOG")
	for i := range len(diagnostic) {
		fmt.Fprint(os.Stderr, diagnostic[i:i+1])
	}
	answer := os.Getenv("DEVKIT_PROVIDER_ANSWER")
	fmt.Fprint(os.Stdout, diagnostic)
	for i, arg := range os.Args {
		if arg == "-o" && i+1 < len(os.Args) {
			if err := os.WriteFile(os.Args[i+1], []byte(answer), 0600); err != nil {
				os.Exit(24)
			}
			os.Exit(0)
		}
	}
	os.Exit(25)
}

func providerEnv(t *testing.T, key string, answer map[string]any) (cli.Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	testutil.Put(t, root, "pack/check.md", "prompt")
	data, err := json.Marshal(answer)
	testutil.OK(t, err)
	fake := testutil.Must(os.Executable())
	values := map[string]string{
		"PACK_DIR": "pack", "ARTIFACTS_DIR": "artifacts",
		"DEVKIT_CODEX": fake, "TYPESAFE_API_KEY": key,
	}
	out, stderr := new(bytes.Buffer), new(bytes.Buffer)
	env := cli.Env{
		Cwd: root, Stdout: out, Stderr: stderr,
		LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok },
		Environ: []string{
			"DEVKIT_PROVIDER_HELPER=1",
			"DEVKIT_PROVIDER_ANSWER=" + string(data),
			"DEVKIT_PROVIDER_LOG=" + key,
			"TYPESAFE_API_KEY=" + key,
			"TYPESAFE_API_KEY=another-fake-key",
			"OPENAI_API_KEY=fake-openai-auth",
			"ANTHROPIC_API_KEY=fake-anthropic-auth",
		},
	}
	return env, out, stderr
}

func TestRunFiltersOnlyTypeSafeEnvironment(t *testing.T) {
	for _, inherited := range []bool{false, true} {
		t.Run(fmt.Sprint(inherited), func(t *testing.T) {
			env, out, stderr := providerEnv(t, "fake-typesafe-key", map[string]any{"answer": "ok"})
			original := append([]string(nil), env.Environ...)
			if inherited {
				for _, entry := range env.Environ {
					key, value, _ := strings.Cut(entry, "=")
					t.Setenv(key, value)
				}
				env.Environ = nil
				original = nil
			}
			answer, err := Run(env, "judge", "check.md", testSchema, 5*time.Second)
			testutil.OK(t, err)
			testutil.Equal(t, answer["answer"], "ok")
			testutil.Equal(t, env.Environ, original)
			if inherited && os.Getenv("TYPESAFE_API_KEY") != "another-fake-key" {
				t.Fatal("parent environment changed")
			}
			testutil.Equal(t, out.Len(), 0)
			if strings.Contains(stderr.String(), "fake-typesafe-key") {
				t.Fatal("secret leaked to stderr")
			}
			logs := testutil.Must(filepath.Glob(env.Path("artifacts/judge/codex-*.log")))
			testutil.Equal(t, len(logs), 1)
			log := string(testutil.Must(os.ReadFile(logs[0])))
			if strings.Contains(log, "fake-typesafe-key") {
				t.Fatal("secret persisted in log")
			}
			testutil.Contains(t, log, "[redacted]")
		})
	}
}

func TestRunValidatesRedactedAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, key, schema string
		input, want       map[string]any
	}{
		{"const", "fake-typesafe-key", `{"type":"object","properties":{"answer":{"const":"fake-typesafe-key"}},"required":["answer"],"additionalProperties":false}`, map[string]any{"answer": "fake-typesafe-key"}, nil},
		{"enum", "fake-typesafe-key", `{"type":"object","properties":{"answer":{"enum":["fake-typesafe-key","other"]}},"required":["answer"],"additionalProperties":false}`, map[string]any{"answer": "fake-typesafe-key"}, nil},
		{"length", "fake-typesafe-key", `{"type":"object","properties":{"answer":{"type":"string","minLength":20}},"required":["answer"],"additionalProperties":false}`, map[string]any{"answer": "fake-typesafe-key accepted"}, nil},
		{"property name", "fake-typesafe-key", `{"type":"object","properties":{"fake-typesafe-key":{"type":"string"}},"required":["fake-typesafe-key"],"additionalProperties":false}`, map[string]any{"fake-typesafe-key": "ok"}, nil},
		{"property collision", "fake-typesafe-key", `{"type":"object"}`, map[string]any{"fake-typesafe-key": "first", "[redacted]": "second"}, nil},
		{"ordinary", "fake-typesafe-key", testSchema, map[string]any{"answer": "ok"}, map[string]any{"answer": "ok"}},
		{"free text", "fake-typesafe-key", testSchema, map[string]any{"answer": "fake-typesafe-key accepted"}, map[string]any{"answer": "[redacted] accepted"}},
		{"escaped key", "fake-\"\\\n界", testSchema, map[string]any{"answer": "before fake-\"\\\n界 after"}, map[string]any{"answer": "before [redacted] after"}},
		{"nested", "fake-typesafe-key", `{"type":"object"}`, map[string]any{"items": []any{map[string]any{"fake-typesafe-key": "fake-typesafe-key"}, true, nil, float64(3)}}, map[string]any{"items": []any{map[string]any{"[redacted]": "[redacted]"}, true, nil, float64(3)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, out, stderr := providerEnv(t, tc.key, tc.input)
			original := append([]string(nil), env.Environ...)
			prompt := testutil.Must(os.ReadFile(env.Path("pack/check.md")))
			answer, err := Run(env, "judge", "check.md", tc.schema, 5*time.Second)
			if tc.want == nil {
				testutil.Fails(t, err, "does not match output schema")
				testutil.Equal(t, answer == nil, true)
			} else {
				testutil.OK(t, err)
				testutil.Equal(t, answer, tc.want)
			}
			testutil.Equal(t, env.Environ, original)
			testutil.Equal(t, testutil.Must(os.ReadFile(env.Path("pack/check.md"))), prompt)
			testutil.Equal(t, out.Len(), 0)
			if strings.Contains(stderr.String(), tc.key) || (err != nil && strings.Contains(err.Error(), tc.key)) {
				t.Fatal("secret leaked on return")
			}
			logs := testutil.Must(filepath.Glob(env.Path("artifacts/judge/codex-*.log")))
			testutil.Equal(t, len(logs), 1)
			log := string(testutil.Must(os.ReadFile(logs[0])))
			if strings.Contains(log, tc.key) {
				t.Fatal("secret persisted in log")
			}
		})
	}
}

func TestRunKeepsBoundedFailureLog(t *testing.T) {
	env, out, stderr := providerEnv(t, "fake-typesafe-key", map[string]any{"answer": "ok"})
	env.Environ = append(env.Environ, "DEVKIT_PROVIDER_FAIL=1")
	answer, err := Run(env, "judge", "check.md", testSchema, 5*time.Second)
	testutil.Fails(t, err, "exit status 9")
	testutil.Equal(t, answer == nil, true)
	testutil.Equal(t, out.Len(), 0)
	if strings.Contains(stderr.String(), "fake-typesafe-key") || strings.Contains(stderr.String(), "HEAD") {
		t.Fatal("secret or discarded prefix leaked")
	}
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	testutil.Equal(t, len([]rune(lines[len(lines)-1])), 4000)
	testutil.Equal(t, strings.HasSuffix(stderr.String(), "[redacted]TAIL\n"), true)
	logs := testutil.Must(filepath.Glob(env.Path("artifacts/judge/codex-*.log")))
	testutil.Equal(t, len(logs), 1)
	log := string(testutil.Must(os.ReadFile(logs[0])))
	if strings.Contains(log, "fake-typesafe-key") {
		t.Fatal("secret persisted in failure log")
	}
	testutil.Equal(t, log, "HEAD"+strings.Repeat("界", 5000)+"[redacted]TAIL")
}

func TestRunBoundsInheritedStdinWait(t *testing.T) {
	env, out, _ := providerEnv(t, "fake-typesafe-key", map[string]any{"answer": "ok"})
	prompt := strings.Repeat("x", 1<<20)
	testutil.Put(t, env.Cwd, "pack/check.md", prompt)
	env.Environ = append(env.Environ, "DEVKIT_CODEX_STDIN_HOLDER=spawn")
	var child *os.Process
	t.Cleanup(func() {
		if child != nil {
			child.Kill()
			child.Release()
		}
	})
	started := time.Now()
	answer, err := Run(env, "judge", "check.md", testSchema, 500*time.Millisecond)
	elapsed := time.Since(started)
	if err == nil || answer != nil {
		t.Fatal("unfinished stdin transfer was accepted")
	}
	if elapsed > 2*time.Second {
		t.Fatal("inherited stdin wait exceeded return deadline")
	}
	var pid int
	_, scanErr := fmt.Sscan(string(testutil.Must(os.ReadFile(env.Path("holder.pid")))), &pid)
	testutil.OK(t, scanErr)
	child, err = os.FindProcess(pid)
	testutil.OK(t, err)
	testutil.OK(t, child.Signal(syscall.Signal(0)))
	testutil.Equal(t, out.Len(), 0)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(env.Path("pack/check.md")))), prompt)
}

func TestRedactedWriterSplitWrites(t *testing.T) {
	for _, key := range []string{"fake-typesafe-key", "aba", "架空のキー", "fake-\"\\\n界", ""} {
		input := "prefix " + key + key + " middle " + key + " suffix"
		if key != "" {
			input += key[:len(key)-1]
		}
		for split := 0; split <= len(input); split++ {
			var output bytes.Buffer
			writer := &redactedWriter{output: &output, key: key}
			for _, chunk := range []string{input[:split], input[split:]} {
				n, err := writer.Write([]byte(chunk))
				testutil.OK(t, err)
				testutil.Equal(t, n, len(chunk))
				if key != "" && strings.Contains(output.String(), key) {
					t.Fatal("secret written before flush")
				}
			}
			testutil.OK(t, writer.flush())
			want := input
			if key != "" {
				want = strings.ReplaceAll(input, key, "[redacted]")
			}
			testutil.Equal(t, output.String(), want)
		}
	}
}

func TestRedactedWriterConcurrentWrites(t *testing.T) {
	var output bytes.Buffer
	writer := &redactedWriter{output: &output, key: "fake-typesafe-key"}
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			for range 100 {
				if _, err := writer.Write([]byte("fake-typesafe-key diagnostics\n")); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	workers.Wait()
	testutil.OK(t, writer.flush())
	testutil.Equal(t, output.String(), strings.Repeat("[redacted] diagnostics\n", 200))
}
