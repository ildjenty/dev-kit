---
name: devkit-discuss
description: Enter a discussion mode for an unsettled product or development topic. Keep an owner-confirmed memo, compare decision records, and relay refine through approval until settled work and open questions have become GitHub Issues.
---

# Discuss with the owner

Before handing anything to refine, invoke the `devkit-runs` skill (the Skill tool, or `/devkit-runs`) and follow it to start refine, relay its questions and approval, and handle a stop. Do not work out workflow inputs or outcomes by reading workflow files or `devkit --help` instead. The installer places
these skills under the session tool selected by `sessions`, independently of the workflow `provider`.

Enter this mode when invoked. Stay in it until the discussion has become Issues: settled work as
unit issues under its topic, and remaining questions as topic issues. Starting refine, relaying its
questions and obtaining approval of its proposal are part of this mode. A confirmed memo alone does
not finish it. If the owner makes an unrelated request, handle it, then resume the discussion where
it was, restating the current memo. Preserve the memo and pending handoff when the conversation is
summarised; do not treat an interruption or summarisation as completion.

The aim is not to settle everything before anything becomes an Issue. Ask only what the next unit of
work needs: a goal and completion conditions someone can observe. Put every other question into the
memo as still open instead of asking it now; open points become topic issues and are discussed later.
As soon as one area is settled enough to cut at least one unit, propose handing the settled part to
refine now, saying that the open points will remain as topic issues. Propose the same when a few
questions in a row settle nothing new. Do not wait for the owner to declare the discussion finished.

Let the owner lead the conversation. Start with open questions about their situation, difficulty and
intention. Ask one useful question at a time; do not begin by forcing a list of solutions. Restate what
you heard and ask the owner to confirm it. Surface contradictions and uncertain assumptions explicitly.

Read the current decisions with:

```sh
devkit decisions list
```

This reads the configured decision log, kept for people and this discussion mode.
Compare the discussion with existing agreed decisions. When they conflict, name the decision ID and
explain the conflict so the owner can say whether to overturn it. Do not silently reinterpret agreement.

Find ready open topics with `devkit issues ready` and read a selected topic and its relationships
with `devkit issues show <number>`. Read another known topic by number when investigating a blocker.

Offer candidates, a recommendation and the reason only when there is a choice to make. A factual
question about the owner's situation has no recommendation. Keep what the owner said separate from
your suggestions; do not answer your own questions for them or guess a fact that changes the outcome.
Do not turn a reversible implementation suggestion into an owner's decision.

Do not edit repository files during the discussion. Keep a running memo in the conversation, in the
configured language, with three parts: decided, overturned (include existing decision IDs), and still
open. Update it whenever a point is settled or revised. Do not label items as the owner's words;
mark only an item that is your suggestion the owner has not confirmed. Include the selected topic number and any pending refine branch, question or proposal
version so the next session can resume. Restate the memo on resuming and ask the owner to confirm or
correct it before passing it to refine.

After confirmation, use `devkit-runs` to start `refine` on a `refine/<name>` branch with the confirmed memo
verbatim as `memo`. A run handles one topic: pass its Issue number as `topic`, or leave it empty for
a new request whose parent topic refine will create on approval. Refine proposes decision records,
changes to mechanisms that must bind agents, and Issue drafts. Its workflow makes the proposal
edits; do not make them yourself in the conversation. Decision records are a log, not delivery
instructions: the Issue drafts must contain everything delivery needs.

Relay refine's questions and the owner's answers verbatim through `devkit-runs`, updating the memo
as choices change. Show the full approval question and exact proposal version; the discussion memo
does not approve that proposal. Only the owner's approval of that version permits the approval run.
Never supply answers, approval or `continue` for the owner.

On approval, refine creates unit issues as sub-issues of the topic and open questions as topic
issues, with blocked-by relationships expressing unit order and topics waiting for other topics.
Check the returned `issue_numbers` with `devkit issues show`; an approval failure leaves the mode
active, and `devkit-runs` handles retrying the same approved version. If part of the discussion is
still unrepresented, resume from the memo and refine that part. Once all parts are represented,
report the Issue numbers and end this mode. Use `devkit-runs` to publish and merge the refine PR
through the repository's PR process before starting the ready unit issues it created.
