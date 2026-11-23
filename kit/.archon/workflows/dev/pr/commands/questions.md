## Purpose

Word the approval questions selected by the machine for uncovered changes. You did not select these targets; verify their supplied facts yourself. The owner reads your wording and the machine binds the answer to its target; altering or combining targets can apply an answer to the wrong change.

Read `$ARTIFACTS_DIR/pr/facts.json` for `language` and `questions`, and the diff if needed. For each machine-selected question return its exact `index` and a `text` in the language named by `language` in your input. Explain the affected path and change, ask whether it is allowed before opening the PR, and explicitly require the answer to begin with the literal token `allow` or `deny` followed by optional explanation. Do not add, remove, combine or answer questions. The machine owns targets and allowances; you only word the question. Read the fixed common rules named by `judge_materials.common`.

Return `{ "questions": [{ "index": 0, "text": "..." }] }` using each supplied index unchanged. Write no files.
