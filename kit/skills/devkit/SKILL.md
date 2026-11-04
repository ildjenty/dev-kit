---
name: devkit
description: Advance the devkit planning, work, and judgment loop. Use when starting a task with /devkit, answering questions for ongoing work, or asked to pause or resume.
---

# Using devkit

Carry out the instructions returned by `devkit`. It determines stage order, judgment, approval, and completion. Follow the schemas, stage definitions and completion rules, check instructions, and principles in the work's `.devkit/kit/<kit_version>/` copy: `schemas/`, `stages/`, `checks/`, and `principles/`, as referenced by the request.

## Invoking commands

Invoke `devkit` as a standalone command with cwd set to the target checkout. Do not add `cd`, redirection, command chaining, or substitution. This lets it run outside the sandbox through the path allowed by the host. Write input JSON to a file first, then pass it as a command argument.

When `next` runs verification commands or the completion commit, set the host timeout to match the operation's deadline. Do not treat failures or timeouts as success or completion; convey the returned reason.

## Starting and locating work

1. Start a new task with `devkit start --plan <name> <task>`. If the human has not specified a plan, omit `--plan` to use `.devkit/plans/default.json`. Select templates only from `.devkit/plans/`.
2. Launch `devkit-planner` with the planner instruction ID returned by `start`. Call `devkit next` after its result arrives.
3. For ongoing work, locate the current state from the SessionStart checkpoint and `devkit status`, then call `devkit next`. If warned of a branch mismatch, check the work branch and do not proceed to writes or completion. If paused, wait for the human to request resume.

If start is refused because no SessionStart observation exists, use `devkit doctor` to check what is missing. After Codex installation, the human must trust the project in interactive mode and trust each hook through `/hooks`. Do not substitute temporary configuration overrides.

## Following next instructions

- **Planning**: Launch the specified `devkit-planner`. For plan checks, detection tests, and judgments, launch the assigned judge. After results arrive, call `devkit next` to accept results captured by the hook.
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

Record intermediate decisions, evidence, and lessons with `devkit note <kind> <file>`. Record the reproduction procedure's location and version before starting a fix, and a hypothesis result before starting the next hypothesis. For classifications allowing provisional decisions, record the decision and applicability in a note so a human can check them at the next gate or PR review.

Write round results to a file following the termination block JSON Schema specified by the request, then submit with `devkit submit <file>`. The default input location is `.devkit/run/work/<work_id>/inbox/`.

The termination block includes the current work instruction's `instruction_id`, work performed, confirmed and unconfirmed matters, obstacles, termination kind, temporary state with its verification method, source, and invalidation conditions, and resume conditions. Devkit generates the changed-file list from the diff. Choose `completion-claimed`, `confirmation-required`, or `work-stopped` as the termination kind; do not assign yourself `PASS`. `confirmation-required` waits for a question without verification or judgment. `work-stopped` does not replace `pause`.

After submission, follow `devkit next`. Devkit commits on stage completion; agents do not commit themselves.

## Questions and answers

Show questions returned by `next` or Stop in chat, preserving the form's wording. Include the question ID, approval target, paused stage and reason, options, recommendation and reason, stage to proceed to after the answer, and references to grounds. Explain that the human may answer freely, give an unlisted or conditional answer, or respond with a question.

When the human answers, interpret it against the original input observed by the hook and record it with `devkit answer <question_id>`. Do not reinterpret quotations, explanations, or another actor's report as approval. If the meaning is ambiguous, clarify rather than proceeding as approved. Follow the CLI's required answer input format; after acceptance, continue with `devkit next`. Record corrections as new answers referencing the answer being replaced.

Record human-requested plan changes with `devkit plan <file>`, withdrawals with `devkit plan --withdraw`, and work cancellation with `devkit abort` at the human's request.

## Pause and resume

When a human asks to stop, regardless of wording, call `devkit pause` without waiting for a termination block submission or for subagents or external operations to finish. The human may also invoke it directly in a terminal or through the host's `!`. Pausing retains work and records.

While paused, do not start new work, judgments, planning, verification, external operations, or completion processing. Do not interpret result arrival, repeated `next`, session changes, or compaction as resume. Do not cancel running external operations or rerun operations whose results are unknown.

Call `devkit resume` only when the human explicitly requests resume, then continue with `devkit next`. Also present questions caused by the Stop continuation limit and wait for an answer. Do not try to reset continuation counts through tool calls.

## External operations

Route external writes through registered `devkit external <kind> <input_file>` operations. If confirmation is pending, present the question, record the answer with `answer`, then make the same call. Follow the returned state for running, unknown-result, and successful operations; do not resend through another path.

## Retrospective

Use findings, failures, and human-discovered misses to investigate conversation records and causes as needed. Record lessons with `devkit note` and propose where to apply improvements and why. The priority order is hooks, lint, and types; plan templates; principles; skill procedures; then the kit.

Record adoption, rejection, or deferral of proposals and the reasons, and obtain required change approvals. In stages that have not declared meta-loop edits, ask the human before changing the loop itself. Do not address recurring rule violations only by adding prose; choose mechanisms based on the failure to prevent and the execution, maintenance, and false-positive burden. Do not combine detection tests with organizing or reducing mechanisms in the same work.
