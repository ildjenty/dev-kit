package retro

import (
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

func start(ctx record.ContextData, env cli.Env) (record.StartOutput, error) {
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return record.StartOutput{}, err
	}
	if oldProposal(p) {
		return record.StartOutput{Stopped: true, Stage: "start", Reason: "old-format retro proposal cannot be used; start retro again on a new retro branch", Details: ctx.Dir, Remaining: []string{}}, nil
	}
	return proposal.Start(ctx, env, "retro")
}
func approvalErrors(ctx record.ContextData, version string) ([]string, error) {
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	if oldProposal(p) {
		return []string{"old-format retro proposal cannot be approved; start retro again on a new retro branch"}, nil
	}
	return proposal.ApprovalErrors(ctx, version, "retro")
}
func authorizedApproval(ctx record.ContextData, version string) error {
	if problems, err := approvalErrors(ctx, version); err != nil {
		return err
	} else if len(problems) > 0 {
		return &record.RecordError{Message: "proposal cannot be approved", Errors: problems}
	}
	return proposal.AuthorizedApproval(ctx, version, "retro")
}
