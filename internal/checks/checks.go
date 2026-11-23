package checks

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

type Result struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Command string  `json:"command"`
	Log     string  `json:"log"`
	Reason  *string `json:"reason,omitempty"`
	Base    string  `json:"base,omitempty"`
	BaseLog string  `json:"base_log,omitempty"`
}
type Gate struct {
	Preexisting []string `json:"preexisting"`
	Regressed   []string `json:"regressed"`
	Round       int      `json:"round"`
	Passed      bool     `json:"passed"`
	Stuck       bool     `json:"stuck"`
	Failing     []string `json:"failing"`
	StuckChecks []string `json:"stuck_checks"`
	Skipped     []string `json:"skipped"`
	Results     string   `json:"results,omitempty"`
}

func Judge(round int, now, before []Result, required []string) Gate {
	if required == nil {
		required = []string{}
		for _, r := range now {
			required = append(required, r.Name)
		}
	}
	environment := []string{}
	for _, r := range now {
		if r.Status == "error" {
			environment = append(environment, r.Name+" (environment failure)")
		}
	}
	for _, name := range required {
		found := false
		for _, r := range now {
			found = found || r.Name == name
		}
		if !found {
			environment = append(environment, name+" (environment failure: no record)")
		}
	}
	failing := []string{}
	stuck := []string{}
	skipped := []string{}
	preexisting, regressed := []string{}, []string{}
	for _, r := range now {
		if r.Status == "fail" {
			failing = append(failing, r.Name)
			if r.Base == "failed" {
				preexisting = append(preexisting, r.Name)
			}
			if r.Base == "passed" {
				regressed = append(regressed, r.Name)
			}
		}
		if r.Status == "skip" {
			reason := ""
			if r.Reason != nil {
				reason = *r.Reason
			}
			skipped = append(skipped, r.Name+" ("+reason+")")
		}
	}
	failing = append(failing, environment...)
	for _, current := range now {
		if current.Status != "fail" || current.Base == "failed" {
			continue
		}
		name := current.Name
		for _, r := range before {
			if r.Name == name && r.Status == "fail" {
				stuck = append(stuck, name)
				break
			}
		}
	}
	stuck = append(stuck, environment...)
	return Gate{Preexisting: preexisting, Regressed: regressed, Round: round, Passed: len(failing) == 0, Stuck: len(stuck) > 0, Failing: failing, StuckChecks: stuck, Skipped: skipped}
}
func encode(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.!~*'()", rune(c)) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

type CheckOptions struct {
	Cwd   string
	Dir   string
	Round int
	Env   []string
}

func RunCheck(check config.Check, o CheckOptions) (Result, error) {
	return runCheck(check, o, true)
}

func runCheck(check config.Check, o CheckOptions, retry bool) (Result, error) {
	log := filepath.Join(o.Dir, fmt.Sprintf("%d-%s.log", o.Round, encode(check.Name)))
	if err := os.WriteFile(log, []byte("$ "+check.Command+"\n"), 0644); err != nil {
		return Result{}, err
	}
	attempt := func() (int, bool, error) {
		fd, err := os.OpenFile(log, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return -1, false, err
		}
		cmd := exec.Command("sh", "-c", check.Command)
		cmd.Dir = o.Cwd
		cmd.Env = o.Env
		cmd.Stdout = fd
		cmd.Stderr = fd
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		startErr := cmd.Start()
		if startErr != nil {
			closeErr := fd.Close()
			if closeErr != nil {
				return -1, true, closeErr
			}
			if err := appendFile(log, startErr.Error()+"\n"); err != nil {
				return -1, true, err
			}
			return -1, true, nil
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		timer := time.NewTimer(time.Duration(check.TimeoutSeconds * float64(time.Second)))
		timedOut := false
		var waitErr error
		select {
		case waitErr = <-done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			timedOut = true
			killErr := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if killErr != nil && killErr != syscall.ESRCH {
				_ = fd.Close()
				return -1, true, killErr
			}
			waitErr = <-done
		}
		if err := fd.Close(); err != nil {
			return -1, false, err
		}
		if timedOut {
			if err := appendFile(log, fmt.Sprintf("Timed out after %s seconds\n", strconv.FormatFloat(check.TimeoutSeconds, 'f', -1, 64))); err != nil {
				return -1, true, err
			}
			return -1, true, nil
		}
		if waitErr == nil {
			return 0, false, nil
		}
		if cmd.ProcessState.ExitCode() < 0 {
			return -1, false, nil
		}
		return cmd.ProcessState.ExitCode(), false, nil
	}
	code, environment, err := attempt()
	if err != nil {
		return Result{}, err
	}
	if code == 75 && retry {
		if err := appendFile(log, "\n--- retried after an environment failure ---\n"); err != nil {
			return Result{}, err
		}
		code, environment, err = attempt()
		if err != nil {
			return Result{}, err
		}
	}
	status := "fail"
	switch {
	case code == 0:
		status = "pass"
	case code == 77:
		status = "skip"
	case code == 75 || environment:
		status = "error"
	}
	result := Result{Name: check.Name, Status: status, Command: check.Command, Log: log}
	if status == "skip" {
		data, err := os.ReadFile(log)
		if err != nil {
			return Result{}, err
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		reason := lines[len(lines)-1]
		result.Reason = &reason
	}
	data, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	if err := appendFile(filepath.Join(o.Dir, fmt.Sprintf("%d.jsonl", o.Round)), string(data)+"\n"); err != nil {
		return Result{}, err
	}
	return result, nil
}

type Options struct {
	NoBase       bool
	BaseOnRepeat bool
	Cwd          string
	ArtifactsDir string
	BaseBranch   string
	Only         []string
	Env          []string
}

func RunChecks(o Options) (Gate, error) {
	if o.ArtifactsDir == "" {
		return Gate{}, fmt.Errorf("ARTIFACTS_DIR must be set")
	}
	g, err := gitx.New(o.Cwd, o.BaseBranch)
	if err != nil {
		return Gate{}, err
	}
	root, err := g.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return Gate{}, err
	}
	allChecks, err := config.ChecksAcrossBase(root, o.BaseBranch)
	c := config.Config{Checks: allChecks}
	if err != nil {
		return Gate{}, err
	}
	for _, name := range o.Only {
		found := false
		for _, check := range c.Checks {
			found = found || check.Name == name
		}
		if !found {
			return Gate{}, fmt.Errorf("--only must name configured checks")
		}
	}
	g, err = gitx.New(root, o.BaseBranch)
	if err != nil {
		return Gate{}, err
	}
	changes, err := g.WorkingChanges("")
	if err != nil {
		return Gate{}, err
	}
	selected := []config.Check{}
	for _, check := range config.ChecksFor(c, changes) {
		if o.Only == nil || slices.Contains(o.Only, check.Name) {
			selected = append(selected, check)
		}
	}
	artifacts := o.ArtifactsDir
	if !filepath.IsAbs(artifacts) {
		artifacts = filepath.Join(o.Cwd, artifacts)
	}
	dir := filepath.Join(artifacts, "checks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Gate{}, err
	}
	previous := 0
	roundFile := filepath.Join(dir, "round")
	data, err := os.ReadFile(roundFile)
	if err == nil {
		number := strings.TrimSpace(string(data))
		n := float64(0)
		if number != "" {
			n, err = strconv.ParseFloat(number, 64)
		}
		if err != nil || n < 0 || n != float64(int(n)) {
			return Gate{}, fmt.Errorf("invalid checks round")
		}
		previous = int(n)
	} else if !os.IsNotExist(err) {
		return Gate{}, err
	}
	round := previous + 1
	before := []Result{}
	data, err = os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.jsonl", previous)))
	if err != nil && !os.IsNotExist(err) {
		return Gate{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var r Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return Gate{}, err
		}
		before = append(before, r)
	}
	resultsPath := filepath.Join(dir, fmt.Sprintf("%d.jsonl", round))
	for _, file := range []struct{ path, text string }{{roundFile, strconv.Itoa(round)}, {resultsPath, ""}, {filepath.Join(dir, fmt.Sprintf("%d-diff.patch", round)), changes.Patch}, {filepath.Join(artifacts, "diff.patch"), changes.Patch}} {
		if err := os.WriteFile(file.path, []byte(file.text), 0644); err != nil {
			return Gate{}, err
		}
	}
	results := []Result{}
	names := []string{}
	for index, check := range selected {
		checkDir := dir
		if slices.ContainsFunc(selected[:index], func(other config.Check) bool { return other.Name == check.Name }) {
			data, err := json.Marshal(check)
			if err != nil {
				return Gate{}, err
			}
			checkDir = filepath.Join(dir, fmt.Sprintf("%x", sha256.Sum256(data)))
			if err := os.MkdirAll(checkDir, 0755); err != nil {
				return Gate{}, err
			}
		}
		r, err := RunCheck(check, CheckOptions{root, checkDir, round, o.Env})
		if err != nil {
			return Gate{}, err
		}
		if checkDir != dir {
			data, err := json.Marshal(r)
			if err != nil {
				return Gate{}, err
			}
			if err := appendFile(resultsPath, string(data)+"\n"); err != nil {
				return Gate{}, err
			}
		}
		results = append(results, r)
		names = append(names, check.Name)
	}
	if !o.NoBase {
		compare := func(r Result) bool { return true }
		if o.BaseOnRepeat {
			compare = func(r Result) bool {
				return slices.ContainsFunc(before, func(b Result) bool { return b.Name == r.Name && b.Status == "fail" })
			}
		}
		if err := compareBase(g, selected, results, dir, round, o.Env, compare); err != nil {
			return Gate{}, err
		}
	} else {
		for i := range results {
			if results[i].Status == "fail" {
				results[i].Base = "unavailable"
			}
		}
	}
	if err := os.WriteFile(resultsPath, nil, 0644); err != nil {
		return Gate{}, err
	}
	for _, result := range results {
		data, err := json.Marshal(result)
		if err != nil {
			return Gate{}, err
		}
		if err := appendFile(resultsPath, string(data)+"\n"); err != nil {
			return Gate{}, err
		}
	}
	gate := Judge(round, results, before, names)
	gate.Results = resultsPath
	return gate, nil
}
func Commands() []cli.Command {
	return []cli.Command{{Area: "checks", Usage: "devkit checks [--only a,b] [--no-base | --base-on-repeat]", Run: func(args []string, env cli.Env) error {
		var only []string
		noBase, onRepeat := false, false
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--no-base":
				noBase = true
			case "--base-on-repeat":
				onRepeat = true
			case "--only":
				if only != nil || i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
					return cli.Usage("devkit checks [--only name1,name2] [--no-base | --base-on-repeat]")
				}
				i++
				only = strings.Split(args[i], ",")
			default:
				return cli.Usage("devkit checks [--only name1,name2] [--no-base | --base-on-repeat]")
			}
		}
		artifacts := env.Getenv("ARTIFACTS_DIR")
		if artifacts == "" {
			dir, err := os.MkdirTemp("", "devkit-checks-")
			if err != nil {
				return err
			}
			artifacts = dir
		}
		if noBase && onRepeat {
			return cli.Usage("devkit checks [--only name1,name2] [--no-base | --base-on-repeat]")
		}
		gate, err := RunChecks(Options{NoBase: noBase, BaseOnRepeat: onRepeat, Cwd: env.Cwd, ArtifactsDir: artifacts, BaseBranch: env.Getenv("BASE_BRANCH"), Only: only, Env: env.Environ})
		if err != nil {
			return err
		}
		return env.JSON(gate)
	}}}
}
