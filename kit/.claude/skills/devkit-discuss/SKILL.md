---
name: devkit-discuss
description: Discuss an unsettled product or development topic with its owner, compare existing decisions, and prepare a confirmed memo for refine without editing documents or decision records.
---

# Discuss with the owner

Read the companion `.claude/skills/devkit-runs/SKILL.md` for starting runs. The installer places
these skills under the session tool selected by `sessions`, independently of the workflow `provider`.

Let the owner lead the conversation. Start with open questions about their situation, difficulty and
intention. Ask one useful question at a time; do not begin by forcing a list of solutions. Restate what
you heard and ask the owner to confirm it. Surface contradictions and uncertain assumptions explicitly.

Read the current decisions with:

```sh
devkit decisions list
```

This reads configured decision, document, unit and topic folders. Read the relevant prose documents as needed.
Compare the discussion with existing agreed decisions. When they conflict, name the decision ID and
explain the conflict so the owner can say whether to overturn it. Do not silently reinterpret agreement.

Offer candidates, a recommendation and the reason only when there is a choice to make. A factual
question about the owner's situation has no recommendation. Keep what the owner said separate from
your suggestions; do not answer your own questions for them or guess a fact that changes the outcome.
Do not turn a reversible implementation suggestion into an owner's decision.

Never edit repository documents or decision records during this skill. Keep the discussion in the
session. When the owner is ready to finish, write a memo in the conversation, in the configured language,
with three parts: decided, overturned (include existing decision IDs), and still open. Distinguish the
owner's choices from your suggestions. Ask the owner to confirm or correct the memo.

After confirmation, use `devkit-runs` to start `refine` on a `refine/<name>` branch with the confirmed memo
verbatim as `memo`. A run settles one topic: use the selected pickable topic ID as `topic`, or leave it
empty for the first request topic. `devkit decisions topics` lists open topics whose decision
dependencies are agreed. Refine decides, documents, and asks approval of an exact proposal version.
The discussion memo does not approve that proposal.

After approval, publish and merge the refine pull request through the repository's PR process before
starting any unit it created. Continue topic by topic, discussing unresolved choices when needed.
For each settled unit, `devkit-runs` passes `unit=<id>` to requirements and follows plan → implement →
judge ⇄ fix → pr, with sync when needed and retro for the merged backlog. Relay recorded questions
and `back` results through that skill; do not supply approval, answers or `continue` for the owner.
