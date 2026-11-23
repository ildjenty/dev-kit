# Git operation rules

Agents conducting work and agents responsible for individual stages follow these rules. Obtain the owner's approval before first applying rule changes, as changes to the workflow itself.

## Work branches and worktrees

- Use one work branch per request, created from the base branch.
- Use a separate worktree for each workflow execution. Do not run multiple executions in a shared checkout.
- Stay on the execution's branch and worktree while it runs. If the branch differs from the starting branch, return to the execution branch before writing or reporting completion.
- Check the current branch before committing or pushing; another session may be using the shared working tree.

## Commit messages

- Commit coherent units of work, rather than every small fix.
- Use `feat:`, `fix:`, `refactor:`, `chore:`, or `docs:`, followed by a concise description of the change. For example: `fix: prevent duplicate submission after a timeout`. Choose the prefix from the change, rather than the branch name.
- When reporting stage completion, prepare a message covering all changes from the start of the stage through completion, rather than only the last fix.
- Use only `Co-Authored-By:` commit trailers. Omit provider-specific session trailers and session URLs from commits and PR bodies.

## Integration and external operations

- Only completed work is eligible for merging. Leave paused or aborted work on its execution branch.
- Never merge from a workflow. The owner decides merging; completion does not authorize automatic integration into the starting branch.
- Follow `authority.md` for pushes and PR creation. Perform them within delegated scope and report the results. PR comments, issue updates, posts, and other external operations remain limited to explicit instructions.
- After the execution branch has been merged, remove the worktree created for that execution.
