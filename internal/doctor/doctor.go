package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

const timeout = 3 * time.Second

func Commands() []cli.Command {
	return []cli.Command{{Area: "doctor", Usage: "devkit doctor [dir]", Run: func(args []string, env cli.Env) error {
		if len(args) > 1 {
			return cli.Usage("devkit doctor [dir]")
		}
		if len(args) == 1 {
			env.Cwd = env.Path(args[0])
		}
		valid, err := reportOutput(env, timeout, 20*time.Second)
		if err != nil {
			return err
		}
		if !valid {
			return &cli.ExitError{Code: 1}
		}
		return nil
	}}}
}

func Report(env cli.Env) bool { return report(env, timeout, 20*time.Second) }

func command(env cli.Env, limit time.Duration, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = env.Cwd, env.Environ
	cmd.WaitDelay = 100 * time.Millisecond
	data, err := cmd.Output()
	return strings.TrimSpace(string(data)), err == nil
}

func report(env cli.Env, limit, providerLimit time.Duration) bool {
	valid, err := reportOutput(env, limit, providerLimit)
	return valid && err == nil
}

func reportOutput(env cli.Env, limit, providerLimit time.Duration) (bool, error) {
	valid := true
	var outputErr error
	finding := func(level, item string, ok bool, present, missing string) {
		status, detail := "OK", present
		if !ok {
			status, detail = "MISSING", missing
			if level == "required" {
				valid = false
			}
		}
		line := fmt.Sprintf("[%s] %s: %s — %s\n", level, item, status, detail)
		if key := env.Getenv("TYPESAFE_API_KEY"); key != "" {
			line = strings.ReplaceAll(line, key, "[redacted]")
		}
		if outputErr == nil {
			_, outputErr = fmt.Fprint(env.Stdout, line)
		}
	}
	_, git := command(env, limit, "git", "--version")
	finding("required", "git", git, "available", "All workflows need Git; install Git from https://git-scm.com/downloads.")
	root, repo := command(env, limit, "git", "rev-parse", "--show-toplevel")
	_, commit := command(env, limit, "git", "rev-parse", "--verify", "HEAD^{commit}")
	finding("required", "repository", repo && commit, "Git repository with a commit", "Worktrees need a Git repository with a commit; run git init and create the first commit.")
	if repo {
		env.Cwd = root
	}
	settings := config.Defaults("")
	data, readErr := os.ReadFile(filepath.Join(env.Cwd, ".devkit/config.json"))
	parsed, configErr := config.Parse(data)
	if configErr == nil {
		settings = parsed
	}
	finding("required", "devkit config", readErr == nil && configErr == nil, "configuration passes", "Workflow settings are unavailable or invalid; run devkit init and repair .devkit/config.json with devkit config.")
	_, concernsErr := concerns.Load(filepath.Join(env.Cwd, settings.Standards))
	finding("required", "devkit concerns check", concernsErr == nil, "standards ownership passes", "Judgments cannot prepare; run devkit concerns check and repair the reported standards, or install them with devkit init.")
	missing := []string{}
	checkFile := func(path string) {
		if info, err := os.Stat(filepath.Join(env.Cwd, path)); err != nil || !info.Mode().IsRegular() {
			missing = append(missing, path)
		}
	}
	walkErr := fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch {
		case strings.HasPrefix(path, ".devkit/standards/"):
			checkFile(settings.Standards + strings.TrimPrefix(path, ".devkit/standards"))
		case strings.HasPrefix(path, ".claude/skills/"):
			key, relative, _ := strings.Cut(strings.TrimPrefix(path, ".claude/skills/"), "/")
			for _, session := range settings.Sessions {
				folder := ".claude/skills/"
				if session == "codex" {
					folder = ".agents/skills/"
				}
				checkFile(folder + settings.Skills[key] + "/" + relative)
			}
		default:
			checkFile(path)
		}
		return nil
	})
	detail := "Workflows or skills are incomplete; run devkit init."
	if len(missing) > 0 {
		detail = fmt.Sprintf("%d kit files missing (first: %s); workflows or skills are incomplete. Run devkit init.", len(missing), missing[0])
	}
	finding("required", "kit files", walkErr == nil && len(missing) == 0, "workflow, skill, standards and configuration files installed", detail)
	calls, callsErr := kitinfo.Invocations(os.DirFS(env.Cwd), kitinfo.Workflows, false)
	for _, session := range settings.Sessions {
		for _, name := range settings.Skills {
			folder := ".claude/skills/"
			if session == "codex" {
				folder = ".agents/skills/"
			}
			invoked, err := kitinfo.Invocations(os.DirFS(env.Cwd), folder+name, true)
			calls = append(calls, invoked...)
			if err != nil && callsErr == nil {
				callsErr = err
			}
		}
	}
	slices.Sort(calls)
	absent := env.Registry.MissingCommands(slices.Compact(calls))
	compatibility := "missing commands: " + strings.Join(absent, ", ")
	if callsErr != nil {
		compatibility += "; cannot read installed kit: " + callsErr.Error()
	}
	finding("required", "kit and binary", callsErr == nil && len(absent) == 0, "all installed workflow and skill commands are registered", compatibility)
	stamp, stampErr := kitinfo.ReadStamp(filepath.Join(env.Cwd, kitinfo.Workflows))
	kitVersion := "no install stamp; run devkit init to record the installed kit version"
	if stampErr == nil {
		kitVersion = fmt.Sprintf("the installed kit was written by devkit %s; this binary carries a different kit (%s); `devkit init` shows the differences and asks before changing anything", stamp.DevkitVersion, env.Registry.Version())
	} else if !os.IsNotExist(stampErr) {
		kitVersion = stampErr.Error()
	}
	finding("needed for a part", "kit version", stampErr == nil && stamp.KitDigest == kitinfo.EmbeddedDigest(), "installed kit digest matches this binary", kitVersion)
	version, archon := command(env, limit, "archon", "--version")
	version = strings.Join(strings.Fields(version), " ")
	finding("required", "archon", archon, version, "Workflows cannot run; install Archon from https://archon.diy and run archon provider add "+settings.Provider+".")
	providers, providersErr := usedProviders(env.Cwd, settings.Provider)
	if providersErr != nil {
		finding("required", "workflow providers", false, "", "Cannot read workflow providers: "+providersErr.Error())
	}
	for _, provider := range providers {
		providerOK, providerDetail := providerInstalled(env, providerLimit, provider)
		finding("required", "Archon provider "+provider, providerOK, providerDetail, "Provider is not installed; run archon provider add "+provider+", then archon doctor.")
	}
	_, devkit := exec.LookPath("devkit")
	finding("required", "devkit on PATH", devkit == nil, "workflows can invoke devkit", "Workflow bash nodes cannot run; install devkit with scripts/install.sh and add its bin directory to PATH.")
	for _, session := range settings.Sessions {
		_, ok := command(env, limit, session, "--version")
		finding("needed for a part", session+" sessions", ok, "skills can be used in "+session+" sessions", "Installed skills need the "+session+" session tool; install and sign in to "+session+".")
	}
	ghExecutable := env.Getenv("DEVKIT_GH")
	if ghExecutable == "" {
		ghExecutable = "gh"
	}
	_, gh := command(env, limit, ghExecutable, "--version")
	finding("needed for a part", "gh (issues, pr)", gh, "GitHub CLI available", "Issue operations and pr publishing need GitHub CLI; install gh from https://cli.github.com.")
	_, auth := command(env, limit, ghExecutable, "auth", "status")
	finding("needed for a part", "gh authentication (issues, pr)", auth, "authenticated", "Issue operations and pr publishing need authentication; run gh auth login.")
	labelJSON, labelsOK := command(env, limit, ghExecutable, "label", "list", "--limit", strconv.Itoa(int(^uint(0)>>1)), "--json", "name")
	type label struct {
		Name string `json:"name"`
	}
	var labels []label
	labelsOK = labelsOK && json.Unmarshal([]byte(labelJSON), &labels) == nil
	for _, name := range []string{settings.Issues.Labels.Unit, settings.Issues.Labels.Topic} {
		present := labelsOK && slices.ContainsFunc(labels, func(item label) bool { return item.Name == name })
		finding("needed for a part", "issue label "+name, present, "available for Issue creation", "Issue operations need this label; approval creates it before creating Issues. If labels cannot be read, check gh authentication and repository access.")
	}
	origin, remote := command(env, limit, "git", "remote", "get-url", "origin")
	finding("needed for a part", "origin (pr)", remote && origin != "", "origin configured", "The pr workflow needs origin; run git remote add origin <repository-url>.")
	key := env.Getenv("TYPESAFE_API_KEY") != ""
	if !key {
		_, fallback := command(env, limit, settings.Provider, "--version")
		finding("needed for a part", settings.Provider+" Jev fallback", fallback, "read-only CLI available for judge route", "Without Jev or its CLI fallback, routing selects all qualities; install and sign in to "+settings.Provider+".")
	}
	finding("optional", "TYPESAFE_API_KEY", key, "set; TypeSafe receives changed-file diffs for routing", "Not set; the configured provider CLI performs routing. Set TYPESAFE_API_KEY to use TypeSafe (changed-file diffs are sent).")
	finding("optional", "checks", len(settings.Checks) > 0, fmt.Sprintf("%d configured; command availability is checked when they run", len(settings.Checks)), "No verification runs; run the "+settings.Skills["devkit-optimize"]+" skill to configure repository checks.")
	return valid, outputErr
}

func usedProviders(root, configured string) ([]string, error) {
	providers := []string{configured}
	type workflow struct {
		Provider  string     `yaml:"provider"`
		Nodes     []workflow `yaml:"nodes"`
		LoopGroup *workflow  `yaml:"loop_group"`
	}
	var visit func(workflow)
	visit = func(node workflow) {
		if node.Provider != "" && !slices.Contains(providers, node.Provider) {
			providers = append(providers, node.Provider)
		}
		for _, child := range node.Nodes {
			visit(child)
		}
		if node.LoopGroup != nil {
			visit(*node.LoopGroup)
		}
	}
	err := filepath.WalkDir(filepath.Join(root, kitinfo.Workflows), func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "fixtures" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(file) != ".yaml" && filepath.Ext(file) != ".yml" {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var document workflow
		if err := yaml.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		for _, node := range document.Nodes {
			visit(node)
		}
		return nil
	})
	return providers, err
}

func providerInstalled(env cli.Env, limit time.Duration, provider string) (bool, string) {
	output, ok := command(env, limit, "archon", "doctor")
	listed, installed := false, false
	warnings := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if _, providers, found := strings.Cut(line, "Installed providers:"); found {
			listed = true
			for _, entry := range strings.Split(providers, ",") {
				id, _, _ := strings.Cut(strings.TrimSpace(entry), " (")
				if id == provider {
					installed = true
				}
			}
		}
		for _, mark := range []string{"✗", "x", "!"} {
			if rest, found := strings.CutPrefix(line, mark); found && strings.HasPrefix(strings.TrimSpace(rest), provider+":") {
				warnings += "; warning: " + line
			}
		}
	}
	if ok && listed {
		return installed, "listed by archon doctor" + warnings
	}
	home := env.Getenv("ARCHON_HOME")
	if home == "" {
		home = filepath.Join(env.Getenv("HOME"), ".archon")
	}
	plugin := filepath.Join(home, "plugins", "archon-provider-"+provider)
	info, err := os.Stat(plugin)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0, "plugin binary at " + plugin + warnings
}
