package doctor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/kit"
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
		if !Report(env) {
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
	valid := true
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
		fmt.Fprint(env.Stdout, line)
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
	for _, provider := range usedProviders(env.Cwd, settings.Provider) {
		providerOK, providerDetail := providerInstalled(env, providerLimit, provider)
		finding("required", "Archon provider "+provider, providerOK, providerDetail, "Provider is not installed; run archon provider add "+provider+", then archon doctor.")
	}
	_, devkit := exec.LookPath("devkit")
	finding("required", "devkit on PATH", devkit == nil, "workflows can invoke devkit", "Workflow bash nodes cannot run; install devkit with scripts/install.sh and add its bin directory to PATH.")
	for _, session := range settings.Sessions {
		_, ok := command(env, limit, session, "--version")
		finding("needed for a part", session+" sessions", ok, "skills can be used in "+session+" sessions", "Installed skills need the "+session+" session tool; install and sign in to "+session+".")
	}
	_, gh := command(env, limit, "gh", "--version")
	finding("needed for a part", "gh (pr)", gh, "GitHub CLI available", "The pr workflow cannot publish; install gh from https://cli.github.com.")
	_, auth := command(env, limit, "gh", "auth", "status")
	finding("needed for a part", "gh authentication (pr)", auth, "authenticated", "The pr workflow cannot publish; run gh auth login.")
	origin, remote := command(env, limit, "git", "remote", "get-url", "origin")
	finding("needed for a part", "origin (pr)", remote && origin != "", "origin configured", "The pr workflow needs origin; run git remote add origin <repository-url>.")
	key := env.Getenv("TYPESAFE_API_KEY") != ""
	if !key {
		_, fallback := command(env, limit, settings.Provider, "--version")
		finding("needed for a part", settings.Provider+" Jev fallback", fallback, "read-only CLI available for judge route and refine reflect", "Without Jev, reflection fails and routing selects all qualities; install and sign in to "+settings.Provider+".")
	}
	finding("optional", "TYPESAFE_API_KEY", key, "set; TypeSafe receives changed-file diffs for routing and new decisions plus their citing documents for reflection", "Not set; the configured provider CLI performs routing and reflection. Set TYPESAFE_API_KEY to use TypeSafe (diffs, new decisions and citing documents are sent).")
	finding("optional", "checks", len(settings.Checks) > 0, fmt.Sprintf("%d configured; command availability is checked when they run", len(settings.Checks)), "No verification runs; run the "+settings.Skills["devkit-optimize"]+" skill to configure repository checks.")
	return valid
}

func usedProviders(root, configured string) []string {
	providers := []string{configured}
	files, _ := filepath.Glob(filepath.Join(root, ".archon/workflows/dev/*/*.yaml"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			name, ok := strings.CutPrefix(strings.TrimSpace(line), "provider:")
			name = strings.TrimSpace(name)
			if ok && name != "" && !slices.Contains(providers, name) {
				providers = append(providers, name)
			}
		}
	}
	return providers
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
