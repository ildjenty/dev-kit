---
name: devkit
description: Advance the devkit planning, work, and judgment loop. Use when starting a task with /devkit, answering questions for ongoing work, or asked to pause or resume.
---

# Using devkit

Carry out the instructions returned by `devkit`. It determines stage order, judgment, approval, and completion. Follow repository-owned norms through the logical names in the brief: `principles/…`, `stages/…`, `checks/…`, `plans/…`, `templates/…`, and `rules.md`. Read the supplied self-contained output schemas identified by `schemas/…`; they include all referenced definitions and are owned by the binary. Work briefs also include the supported note kinds and their schemas, and `schemas/structure.json`. Read `requirements.md` as the repository's requirements source and use relevant records from `structure.json`. Use the actual working-tree paths and existence information shown for these products in the brief; create a missing product at its indicated path.

The brief supplies the current stage definition, the aspect being judged and its referenced principles, rules, requirements, and relevant structure records. For other norms, use the listed paths under the frozen `snap/` snapshot for that round. Plan checks and detection tests use committed norms from the work branch HEAD fixed at instruction issue time. Do not replace frozen inputs with norms being edited in the working tree. Work records retain `kit_version` (`v2`) to identify compatibility with the binary's record format. Every work brief and judgment request references a work-history file fixed at instruction issue time; use it as required by the stage or aspect. Round judgments also include the expanded first-round start comparison base for the stage run, changed-file list, and patch to the returned target.

## Invoking commands

Invoke `devkit` as a standalone command with cwd set to the target checkout. Do not add `cd`, redirection, command chaining, or substitution. This lets it run outside the sandbox through the path allowed by the host. Write input JSON to a file first, then pass it as a command argument.

When `next` runs verification commands or the completion commit, set the host timeout to match the operation's deadline. Do not treat failures or timeouts as success or completion; convey the returned reason.

## Starting and locating work

1. Start a new task with `devkit start --plan <name> <task>`. If the human has not specified a plan, omit `--plan` to use `default_plan` in `devkit.toml`, whose default is `default`. Select templates only from `plans/`. Devkit binds the latest human input observed by UserPromptSubmit for the session to the work; if none was observed, the record states that the original request is unavailable. Do not substitute the agent-written task for the original request.
2. Launch `devkit-planner` with the planner instruction ID returned by `start`. Call `devkit next` after its result arrives.
3. For ongoing work, locate the current state from the SessionStart checkpoint and `devkit status`, then call `devkit next`. If warned of a branch mismatch, check the work branch and do not proceed to writes or completion. If paused, wait for the human to request resume.

If start is refused because no SessionStart observation exists, use `devkit doctor` to check what is missing. After Codex installation, the human must trust the project in interactive mode and trust each hook through `/hooks`. Do not substitute temporary configuration overrides.

`devkit init` commits installed files after confirmation unless `--no-commit` is used. Before creating a branch or records, start requires an existing HEAD and verifies that existing configured norms, `devkit.toml`, and agent definitions are committed and unmodified; if refused, commit the installation changes first as instructed.

## Following next instructions

- **Planning**: Launch the specified `devkit-planner`. When instructed to replan, launch the specified planner through the same procedure. For plan checks, detection tests, and judgments, launch the assigned judge. After results arrive, call `devkit next` to accept results captured by the hook.
- **Work**: Read the brief and its references and carry out the agreed scope. Record unread inputs and their impact as unverified matters. Other skills and plugins may be used within the request's scope.
- **Awaiting judgment**: Wait for results from judges or planners already launched. Do not launch duplicates for the same instruction ID; call `devkit next` after results arrive.
- **Questions and external operation confirmation**: Present them in chat using the next section's procedure, end the turn, and wait for a human answer. Treat human-assigned stages the same way.
- **Completion or abort**: Report the recorded state and actual remaining work, then finish. A worker's completion claim does not finalize completion.

If output returns a path instead of the body, read that file. Do not repeat `next` with an unchanged instruction to substitute for progression or approval.

## Launching judges and planners

Use only the following single line as the launch message, inserting the returned instruction ID. Do not add self-evaluation, author reasoning, or additional requests.

```text
devkit-instruction <instruction_id>
```

This is not a command to execute. The subagent resolves the request by reading `.devkit/run/work/<work_id>/brief/<instruction_id>.md`, using the work ID contained in the instruction ID.

- In Claude Code, use the assigned `devkit-judge` or `devkit-planner` as Agent's `subagent_type`. Do not inherit the parent's history or override the model or effort in the role definition.
- In Codex, use the assigned role as `spawn_agent`'s `agent_type` and always specify `fork_turns: "none"`. Use only the single line above as `message`.

Subagents return results in their final messages. The parent must not transcribe, modify, or submit their results. Do not write `.devkit/run/**/results/` or `records/` through ordinary file editing. When a judgment or verification command is the runner, devkit executes it; do not launch a substitute judge.

## Recording and submitting

Record intermediate decisions, evidence, and lessons with `devkit note <kind> <file>`, using the supported kinds and bundled schemas in the work brief. Record the reproduction procedure's location and version before starting a fix, and a hypothesis result before starting the next hypothesis. For classifications allowing provisional decisions, record the decision and applicability in a note so a human can check them at the next gate or PR review.

Write round results to a file following the termination block JSON Schema specified by the request, then submit with `devkit submit <file>`. The default input location is `.devkit/run/work/<work_id>/inbox/`.

The termination block includes the current work instruction's `instruction_id`, work performed, confirmed and unconfirmed matters, obstacles, termination kind, temporary state with its verification method, source, and invalidation conditions, and resume conditions. Devkit generates the changed-file list from the diff. Choose `completion-claimed`, `confirmation-required`, or `work-stopped` as the termination kind; do not assign yourself `PASS`. `confirmation-required` waits for a question without verification or judgment. `work-stopped` does not replace `pause`.

After submission, follow `devkit next`. Devkit commits on stage completion; agents do not commit themselves.

Keep runtime records under the fixed, untracked `.devkit/run/` location. Preserve decisions needed by later tasks in `requirements.md`, `structure.json`, or `rules.md` as described in `stages/README.md#decisions-for-later-work`.

## Plan approval

When `next` returns a plan approval question, present it as generated, including the original request, completion conditions, stages, deviations, required checks, and `open_questions`. Do not replace it with your own summary. The human may answer freely; record the answer with `devkit answer`. A nonempty `open_questions` array requires human approval under every plan approval policy. Answers to open questions and revision requests go back to the planner as rejection and replanning instructions.

## Changes to the loop

Edits to norms, host settings, skills, and the active pre-commit configuration are changes to the loop. Confirmation follows only `[approval] meta_loop_edit` in `devkit.toml`, independently of `[router] human_confirmation`. When devkit asks, present the generated question unchanged, including the changed norm, host-setting and skill files and the diff reference, and wait for the human. If the human rejects the changes, follow the next work round to undo the listed rejected changes. Devkit retains the work target and tracks confirmed norms separately; do not restore an older work tree or discard unrelated work. Unchanged rejected edits are returned to the worker for removal rather than repeatedly asking for confirmation.

## Verification commands

If a `command:<name>` check fails because the command is not registered, create the verification method and register it under `[commands.<name>]` in `devkit.toml`. Do not remove the check to avoid creating verification. If a verification command changes the tree, make its generated output ignored.

## Questions and answers

Show questions returned by `next` or Stop in chat as generated, preserving the form's wording. Gate questions include the plan's gate reason, the termination block's work performed and confirmed and unconfirmed matters, and the diff reference. Loop-edit questions include the changed files and diff reference. Do not replace either with an agent-written summary. Include the question ID, approval target, paused stage and reason, options, recommendation and reason, stage to proceed to after the answer, and references to grounds. Explain that the human may answer freely, give an unlisted or conditional answer, or respond with a question.

When the human answers, interpret it against the original input observed by the hook and record it with `devkit answer <question_id>`. Do not reinterpret quotations, explanations, or another actor's report as approval. If the meaning is ambiguous, clarify rather than proceeding as approved. Follow the CLI's required answer input format; after acceptance, continue with `devkit next`. When the human asks to return to an earlier stage in a stage question, record the answer with `devkit answer --interpretation back <question_id>`; devkit follows the plan's back transition and replanning path. Record corrections as new answers referencing the answer being replaced.

Record human-requested plan changes with `devkit plan <file>`, withdrawals with `devkit plan --withdraw`, and work cancellation with `devkit abort` at the human's request.

## Pause and resume

When a human asks to stop, regardless of wording, call `devkit pause` without waiting for a termination block submission or for subagents or external operations to finish. The human may also invoke it directly in a terminal or through the host's `!`. Pausing retains work and records.

While paused, do not start new work, judgments, planning, verification, external operations, or completion processing. Do not interpret result arrival, repeated `next`, session changes, or compaction as resume. Do not cancel running external operations or rerun operations whose results are unknown.

Call `devkit resume` only when the human explicitly requests resume, then continue with `devkit next`. Also present questions caused by the Stop continuation limit and wait for an answer. Do not try to reset continuation counts through tool calls.

## External operations

Route external writes through registered `devkit external --target <target> <kind> <input_file>` operations. If confirmation is pending, present the question, record the answer with `answer`, then make the same call. Follow the returned state for running, unknown-result, and successful operations; do not resend through another path.

## Retrospective

Carry out `stages/retrospective.md` when the plan includes it. Read the work-history file referenced in the brief for all judgments and findings, including nonblocking findings, the sequence of `FAIL` and `NOT ASSESSED` results, round counts, human rejections and corrections, replanning instructions, human-discovered misses recorded as lessons with `devkit note`, and the norms inventory. Investigate conversation records and causes as needed.

For each problem, identify where it should have been caught and where to prevent recurrence. Follow `[lessons] destinations`; the default priority is machine checks (hooks, lint, types, and verification commands), plan templates, aspects, principles, then skills. Apply changes to repository-owned norms, update detection examples with aspect changes, and record adoption, rejection, or deferral with reasons. Consider unenforced rules and recurring violations for machine checks, weighing the failure prevented against execution, maintenance, and false-positive burden.

Check which principles are used by which aspects and templates, and whether any relevant mechanism is unused or costs more than it contributes. Focus on the task's problems, misses, gaps, and burdens; inventory all norms only when a need is evident. Reflect repository requirements from approved completion conditions in `requirements.md` if purpose did not already do so, including when purpose is absent. Follow the loop-edit confirmation policy for norm changes; without loop edits, no loop-edit confirmation is needed.

