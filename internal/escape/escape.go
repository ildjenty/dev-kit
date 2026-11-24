package escape

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

type Entry struct {
	Defect     string `json:"defect"`
	Entered    string `json:"entered"`
	Fixed      string `json:"fixed,omitempty"`
	RecordedAt string `json:"recorded_at"`
}

var commit = regexp.MustCompile(`^[a-fA-F0-9]{4,64}$`)
var number = regexp.MustCompile(`^[1-9][0-9]*$`)
var pull = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/[1-9][0-9]*$`)
var ID = regexp.MustCompile(`^[a-f0-9]{6,}$`)

func IsPR(ref string) bool { return number.MatchString(ref) || pull.MatchString(ref) }

func Validate(data []byte) (Entry, error) {
	var entry Entry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entry); err != nil {
		return entry, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return entry, fmt.Errorf("escape must contain one JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return entry, err
	}
	for key := range fields {
		if key != "defect" && key != "entered" && key != "fixed" && key != "recorded_at" {
			return entry, fmt.Errorf("unknown escape field %q", key)
		}
	}
	if !utf8.Valid(data) || strings.TrimSpace(entry.Defect) == "" {
		return entry, fmt.Errorf("defect must be non-empty UTF-8 text")
	}
	for _, field := range []struct{ name, value string }{{"entered", entry.Entered}, {"fixed", entry.Fixed}} {
		_, present := fields[field.name]
		if field.name == "fixed" && !present {
			continue
		}
		if !commit.MatchString(field.value) && !IsPR(field.value) {
			return entry, fmt.Errorf("%s must be a commit hash, GitHub PR URL or positive PR number", field.name)
		}
	}
	if _, err := time.Parse(time.RFC3339, entry.RecordedAt); err != nil {
		return entry, fmt.Errorf("recorded_at must be an RFC3339 timestamp")
	}
	return entry, nil
}

func resolve(root, ref string) (string, error) {
	if pull.MatchString(ref) {
		return ref, nil
	}
	if ref != "" && strings.TrimSpace(ref) == ref {
		if hash, err := gitx.Run(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}"); err == nil {
			return hash, nil
		}
	}
	if commit.MatchString(ref) || IsPR(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("invalid reference %q: expected a local commit, commit hash, GitHub PR URL or positive PR number", ref)
}

func Commands() []cli.Command {
	const usage = "devkit backlog escape --defect-file <path> --entered <commit|PR URL|PR number> [--fixed <commit|PR>]"
	return []cli.Command{{Area: "backlog", Name: "escape", Usage: usage, Run: func(args []string, env cli.Env) error {
		flags := flag.NewFlagSet("backlog escape", flag.ContinueOnError)
		flags.SetOutput(env.Stderr)
		var defectFile, entered, fixed string
		flags.StringVar(&defectFile, "defect-file", "", "file containing the owner's defect description")
		flags.StringVar(&entered, "entered", "", "entering commit or PR")
		flags.StringVar(&fixed, "fixed", "", "fixing commit or PR (optional)")
		if err := flags.Parse(args); err != nil || flags.NArg() != 0 || defectFile == "" || entered == "" {
			return cli.Usage(usage)
		}
		fixedSet := false
		flags.Visit(func(f *flag.Flag) { fixedSet = fixedSet || f.Name == "fixed" })
		if fixedSet && fixed == "" {
			return fmt.Errorf("fixed must not be empty")
		}
		root, err := gitx.Run(env.Cwd, "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		data, err := os.ReadFile(env.Path(defectFile))
		if err != nil {
			return err
		}
		if !utf8.Valid(data) || strings.TrimSpace(string(data)) == "" {
			return fmt.Errorf("defect file must contain non-empty UTF-8 text")
		}
		entered, err = resolve(root, entered)
		if err != nil {
			return err
		}
		if fixedSet {
			fixed, err = resolve(root, fixed)
			if err != nil {
				return err
			}
		}
		entry := Entry{Defect: string(data), Entered: entered, Fixed: fixed, RecordedAt: time.Now().UTC().Format(time.RFC3339)}
		body, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return err
		}
		if _, err = Validate(body); err != nil {
			return err
		}
		dir := filepath.Join(root, ".devkit/backlog/escapes")
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		for {
			id := make([]byte, 3)
			if _, err = rand.Read(id); err != nil {
				return err
			}
			path := filepath.Join(dir, hex.EncodeToString(id)+".json")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			_, err = file.Write(append(body, '\n'))
			closed := file.Close()
			if err != nil {
				return err
			}
			if closed != nil {
				return closed
			}
			_, err = fmt.Fprintf(env.Stdout, "%s\nCommit this record to reach the base branch, with the fix or through the repository's PR process.\n", path)
			return err
		}
	}}}
}
