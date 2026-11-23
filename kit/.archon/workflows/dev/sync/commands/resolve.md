## Purpose

Resolve the conflicts while moving this branch onto the configured base branch. Other authors made both sides; verify their changes yourself. The workflow continues the rebase and runs checks, then code judges review the whole change again. Dropping either side here may survive until a test happens to exercise it.

Read `$ARTIFACTS_DIR/sync/conflict.json`: `language`, `standards`, `writer_materials`, conflicting `files`, `replaying` (the branch commit being applied) and `onto` (the base commit). Read the fixed documents named by `writer_materials`: `criteria`, `common`, `authority` and numbered principle references (path before `#`). Apply these standards. Read both fixed versions with `git show` when needed. Conflict markers are in the checkout.

Resolve each conflict so that both changes hold. Preserve the branch behavior within the base's current names, locations and contracts. Edit other files only when required by the conflict resolution. Do not run mutating git commands, stage, commit, continue or abort the rebase; the script does that.

For a generated file such as a lockfile, regenerate it with the repository's own tool only when the repository tells you which command to use and the source conflict is resolved. Use Bash only for read-only git inspection and that established regeneration command. If the tool or command cannot be determined, report the unresolved generated file; do not hand-edit it or choose a package manager.

When both changes cannot hold or a generated conflict cannot be regenerated, do not pick a side. Return `resolved: false` and `contradiction` describing both changes and the reason, with paths and lines. Otherwise remove every marker and return `resolved: true`, `contradiction: ""`. Write human-readable output in the language named by `language` in your input. Change nothing outside the resolution.
