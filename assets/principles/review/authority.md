# Decision authority

Review decisions and later outputs against the authority rules and recorded human answers.

## Criteria

The authority rules (`authority` source, default `.devkit/authority.md`) are part of the criteria.

## What to check

- Classifications match the authority rules, including applicable delegations.
- Decided decisions requiring the human and decisions marked `decider: "human"` have a matching answer, or a matching gate question and approval, in the fixed work history.
- Decisions proposed for the current stage's own human gate are marked provisional with options, a recommendation, and a reason; they need no prior answer.
- Later outputs match approved decisions in committed design documents, `rules.md`, and `structure.json`.

## Blocking

A decided decision requiring the human or marked `decider: "human"` lacks a matching answer or gate approval, a decision requiring the human is neither decided nor proposed for the current stage's gate, or later work departs from a committed approved decision.
