package proposal

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
)

func PublishProposal(ctx record.ContextData, env cli.Env, content map[string]any, path string) (issues.Creation, error) {
	progress, err := issues.ReadCreation(path)
	if err != nil {
		return progress, err
	}
	count, err := gitx.Run(ctx.Cwd, "rev-list", "--count", ctx.BaseBranch+"..HEAD")
	if err != nil {
		return progress, err
	}
	if count == "0" {
		progress.Publication = "nothing_to_publish"
		return progress, issues.SaveCreation(path, progress)
	}
	branch, err := gitx.Run(ctx.Cwd, "branch", "--show-current")
	if err != nil {
		return progress, err
	}
	clean, err := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return progress, err
	}
	if branch != ctx.Branch || branch == ctx.BaseBranch || clean != "" {
		return progress, fmt.Errorf("proposal publication requires its clean branch head")
	}
	title, body, err := publicationText(ctx, env, content, progress)
	if err != nil {
		return progress, err
	}
	results, err := record.List(ctx.Dir, "result")
	if err != nil {
		return progress, err
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return progress, err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return progress, err
	}
	address, _, err := g.Publish(env, gitx.Publication{Branch: ctx.Branch, BaseBranch: ctx.BaseBranch, Title: title, Body: body, State: "all", Recorded: record.RecordedCommitsOf(results, judgments)}, nil)
	if err != nil {
		return progress, err
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return progress, fmt.Errorf("invalid pull request URL: %q", address)
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) < 3 || parts[len(parts)-2] != "pull" {
		return progress, fmt.Errorf("invalid pull request URL: %q", address)
	}
	number, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || number <= 0 {
		return progress, fmt.Errorf("invalid pull request URL: %q", address)
	}
	progress.PRNumber, progress.PRURL, progress.Publication = number, address, "published"
	return progress, issues.SaveCreation(path, progress)
}

func publicationText(ctx record.ContextData, env cli.Env, content map[string]any, progress issues.Creation) (string, string, error) {
	cfg, err := config.Load(ctx.Cwd)
	if err != nil {
		return "", "", err
	}
	repository := ""
	if i := strings.LastIndex(progress.PRURL, "/pull/"); i >= 0 {
		repository = progress.PRURL[:i]
	}
	if repository == "" {
		repository, err = gitx.PublicationCommand(ctx.Cwd, env, "", "gh", "repo", "view", "--json", "url", "--jq", ".url")
		if err != nil {
			return "", "", err
		}
	}
	return proposalText(content, progress, ctx.Branch, repository, cfg)
}

func UpdateProposal(ctx record.ContextData, env cli.Env, content map[string]any, progress issues.Creation) error {
	if progress.PRNumber == 0 {
		return nil
	}
	title, body, err := publicationText(ctx, env, content, progress)
	if err != nil {
		return err
	}
	_, err = gitx.PublicationCommand(ctx.Cwd, env, body, "gh", "pr", "edit", progress.PRURL, "--title", title, "--body-file", "-")
	return err
}
