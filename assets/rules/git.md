# Git operation rules

The session agent follows these rules for branches, worktrees, commits, and integration. Workers follow the commit message rule. Devkit does not create branches; it commits each completed stage on the current branch.

## Work branches

- Before `devkit start`, cut a work branch from the starting branch (usually `main`). Name it with one of the kinds `feat/`, `fix/`, `chore/`, `refactor/`, or `docs/` followed by a short hyphenated description, for example `fix/retry-on-timeout`.
- Run one work per checkout. Devkit identifies work by the checkout, not by the branch name. To run another loop in parallel, create a separate git worktree with its own work branch (`git worktree add <path> -b <branch>`) and open a new session there.
- Stay on the work branch until the work completes. If devkit warns that the branch differs from the one at work start, switch back before writing or completing.

## Commit messages

- Prefix each commit message with the same kind as the work branch, followed by a colon and a short imperative summary, for example `fix: retry order submission after a timeout`.
- When claiming stage completion, the worker writes `commit_message` in the progress block following this rule. The completion commit holds every change of the stage run since the stage started, across all its rounds, so the message describes that whole change, not only the last round. Devkit uses it for the stage's completion commit and falls back to the `[git] commit_message` template in `devkit.toml` when it is absent.

## Integration

- After the work completes, integrate the work branch back into the starting branch. Integrate only completed work; leave paused or aborted work on its branch.
- Push and create pull requests only through `devkit external`, which records the operation and asks for human confirmation as configured. Do not run `git push` or create pull requests directly.
- Remove a worktree created for a parallel loop after its work branch is integrated.
