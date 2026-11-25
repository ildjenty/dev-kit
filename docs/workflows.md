# Workflow diagrams

Generated from `kit/.archon/workflows/dev/*/*.yaml`. After changing a workflow, regenerate with `DEVKIT_UPDATE_DOCS=1 mise exec -- go test ./internal/workflowdoc`; the test fails while this file is out of date.

Gray nodes run on the machine (bash and devkit), blue nodes are AI nodes, and yellow frames are loops. A dotted arrow leads to a node that runs only under a condition (`when`). See the [design document](design.md) for each workflow's role.

## optimize

Fit the kit to repository evidence and the owner's instructions, and triage existing open Issues. Review up to three committed drafts with machine checks and a fresh read-only judge. Ask approval of the reviewed summary and owner-reserved points, then publish the PR and issue triage.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit optimize start</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>devkit optimize not-started</small>"]:::machine
  n_approve["<b>approve</b><br/><small>devkit optimize approve</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit optimize prepare</small>"]:::machine
  subgraph n_draft ["draft (loop, max 3)"]
    direction TB
    n_draft_write["<b>write</b><br/><small>AI: write</small>"]:::ai
    n_draft_check["<b>check</b><br/><small>devkit optimize check</small>"]:::machine
    n_draft_judge["<b>judge</b><br/><small>AI: judge</small>"]:::ai
    n_draft_round["<b>round</b><br/><small>devkit optimize round</small>"]:::machine
    n_draft_write --> n_draft_check
    n_draft_check -.-> n_draft_judge
    n_draft_check --> n_draft_round
    n_draft_judge --> n_draft_round
  end
  class n_draft loop
  n_finish["<b>finish</b><br/><small>devkit optimize finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '${finished:-${approved:-$skipped}}'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_approve
  n_start -.-> n_prepare
  n_prepare --> n_draft
  n_draft --> n_finish
  n_not_started --> n_result
  n_approve --> n_result
  n_finish --> n_result
```

## refine

Turn a topic Issue or owner request into decision records, binding changes and Issue drafts. Review at most three drafts and commit one proposal per version. Restart with approve=<version> to agree decisions, publish the PR and create Issues without AI.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit refine start</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>devkit refine not-started</small>"]:::machine
  n_approve["<b>approve</b><br/><small>devkit refine approve</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit refine prepare</small>"]:::machine
  subgraph n_decide ["decide (loop, max 3)"]
    direction TB
    n_decide_write_decide["<b>write-decide</b><br/><small>AI: decide</small>"]:::ai
    n_decide_check_decide["<b>check-decide</b><br/><small>devkit refine check</small>"]:::machine
    n_decide_judge_decide["<b>judge-decide</b><br/><small>AI: judge-decide</small>"]:::ai
    n_decide_round_decide["<b>round-decide</b><br/><small>devkit refine round</small>"]:::machine
    n_decide_write_decide --> n_decide_check_decide
    n_decide_check_decide -.-> n_decide_judge_decide
    n_decide_check_decide --> n_decide_round_decide
    n_decide_judge_decide --> n_decide_round_decide
  end
  class n_decide loop
  n_finish["<b>finish</b><br/><small>devkit refine finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '${finished:-${approved:-$skipped}}'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_approve
  n_start -.-> n_prepare
  n_prepare --> n_decide
  n_decide --> n_finish
  n_not_started --> n_result
  n_approve --> n_result
  n_finish --> n_result
```

## requirements

The requirements part: turns the request into requirements (goal, completion conditions with IDs, constraints, open points) and has a fresh judge compare them with the requests and the owner's answers (review/purpose.md in the configured criteria folder), rewriting until nothing blocks, at most 3 times. Runs on its own on the branch of the request: the first node checks the start conditions on the record of the request ($STATE_DIR/records/<branch>/) and records the owner's answers; the last writes the requirements, the judgment and the result there. Ends with pass, question (one question per open point, answered by starting this workflow again with the answers) or stop. Inputs: issue (an issue number, or empty for a direct request), owner_words (the owner's words of a new request; empty when only answering), supplement (what the session added, kept apart), answers (a JSON array of {question_id, answer}, or empty), continue (the owner's words that let the branch start past the limit of restarts, or empty).

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start requirements</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>devkit requirements not-started '$value'</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit requirements prepare</small>"]:::machine
  subgraph n_draft ["draft (loop, max 3)"]
    direction TB
    n_draft_write["<b>write</b><br/><small>AI: write</small>"]:::ai
    n_draft_judge["<b>judge</b><br/><small>AI: judge</small>"]:::ai
    n_draft_round["<b>round</b><br/><small>devkit requirements round</small>"]:::machine
    n_draft_write --> n_draft_judge
    n_draft_judge --> n_draft_round
  end
  class n_draft loop
  n_finish["<b>finish</b><br/><small>devkit requirements finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '${finished:-$skipped}'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_prepare
  n_prepare --> n_draft
  n_draft --> n_finish
  n_not_started --> n_result
  n_finish --> n_result
```

## plan

Plan the latest passed requirements in two loops, each at most three rounds. First write structure rows and the skeleton, run machine checks and the fresh structure judge, and settle owner questions before detail. Then write other rows, release units and tests inside the fixed structure. Detail may change signatures but no file or folder may be added, removed or moved relative to the structure commit; a required structure change ends with stop. Rows carry evidence. New structure rows carry nature. Prepare exports the draft schema into artifacts. Record judgments and outcomes: pass, carry (detail findings), question, back to requirements, or stop. Inputs: owner answers and optional authorization to pass the request's restart limit.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start plan</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>devkit plan not-started '$value'</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit plan prepare</small>"]:::machine
  n_facts["<b>facts</b><br/><small>devkit plan facts</small>"]:::machine
  subgraph n_structure ["structure (loop, max 3)"]
    direction TB
    n_structure_structure_write["<b>structure-write</b><br/><small>AI: write-structure</small>"]:::ai
    n_structure_structure_check["<b>structure-check</b><br/><small>devkit plan check</small>"]:::machine
    n_structure_judge_structure["<b>judge-structure</b><br/><small>AI: judge-quality (structure)</small>"]:::ai
    n_structure_structure_settle["<b>structure-settle</b><br/><small>INPUTS_STRUCTURE_QUESTION='$structure_question' IN</small>"]:::machine
    n_structure_structure_write --> n_structure_structure_check
    n_structure_structure_check -.-> n_structure_judge_structure
    n_structure_structure_write --> n_structure_structure_settle
    n_structure_structure_check --> n_structure_structure_settle
    n_structure_judge_structure --> n_structure_structure_settle
  end
  class n_structure loop
  n_advance["<b>advance</b><br/><small>devkit plan advance</small>"]:::machine
  subgraph n_detail ["detail (loop, max 3)"]
    direction TB
    n_detail_write["<b>write</b><br/><small>AI: write-detail</small>"]:::ai
    n_detail_check["<b>check</b><br/><small>devkit plan check</small>"]:::machine
    n_detail_judge_data["<b>judge-data</b><br/><small>AI: judge-quality (data)</small>"]:::ai
    n_detail_judge_contract["<b>judge-contract</b><br/><small>AI: judge-quality (contract)</small>"]:::ai
    n_detail_judge_security["<b>judge-security</b><br/><small>AI: judge-quality (security)</small>"]:::ai
    n_detail_judge_ui["<b>judge-ui</b><br/><small>AI: judge-quality (ui)</small>"]:::ai
    n_detail_judge_cost["<b>judge-cost</b><br/><small>AI: judge-quality (cost)</small>"]:::ai
    n_detail_judge_delivery["<b>judge-delivery</b><br/><small>AI: judge-delivery</small>"]:::ai
    n_detail_settle["<b>settle</b><br/><small>devkit plan settle</small>"]:::machine
    n_detail_write --> n_detail_check
    n_detail_check -.-> n_detail_judge_data
    n_detail_check -.-> n_detail_judge_contract
    n_detail_check -.-> n_detail_judge_security
    n_detail_check -.-> n_detail_judge_ui
    n_detail_check -.-> n_detail_judge_cost
    n_detail_check -.-> n_detail_judge_delivery
    n_detail_write --> n_detail_settle
    n_detail_check --> n_detail_settle
    n_detail_judge_data --> n_detail_settle
    n_detail_judge_contract --> n_detail_settle
    n_detail_judge_security --> n_detail_settle
    n_detail_judge_ui --> n_detail_settle
    n_detail_judge_cost --> n_detail_settle
    n_detail_judge_delivery --> n_detail_settle
  end
  class n_detail loop
  n_finish["<b>finish</b><br/><small>devkit plan finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '${finished:-$skipped}'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_prepare
  n_prepare --> n_facts
  n_facts --> n_structure
  n_structure --> n_advance
  n_advance -.-> n_detail
  n_advance --> n_finish
  n_detail --> n_finish
  n_not_started --> n_result
  n_finish --> n_result
```

## implement

The implement part. Input: the latest passed requirements and plan, both from the record of the branch; the answers of the owner when the session passes them (input `answers`). Makes the plan's increments one by one in order: for each unit a fresh session first writes the tests of its completion conditions, then another writes the code, and the checks the change needs run. A unit whose checks pass is committed with the run, the unit and the versions it was made from as trailers, so a later run on the same versions goes on from the next unit. The loop gives each release step at most `limits.passes` passes (default 3) and stops when one step uses them up; the loop itself is bounded at 50 passes as a safety limit. Decisions the implementation made or settled (rows decided later) go into a new version of the plan's decision list; a decision the owner has to make is not implemented: the run returns back to plan with the pending owner row. When the same check fails twice, a fresh judge names the premise that broke and where to go back. Ends with pass, back or stop, written to the record by devkit record result. Output: the result as recorded, or the start's output when the start conditions did not hold.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start implement</small>"]:::machine
  n_start_stopped["<b>start-stopped</b><br/><small>printf '%s' '$value'</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit implement prepare</small>"]:::machine
  subgraph n_units ["units (loop, max 50)"]
    direction TB
    n_units_unit["<b>unit</b><br/><small>devkit implement next</small>"]:::machine
    n_units_tests["<b>tests</b><br/><small>AI: tests</small>"]:::ai
    n_units_tests_written["<b>tests-written</b><br/><small>devkit implement tests-written</small>"]:::machine
    n_units_code["<b>code</b><br/><small>AI: implement</small>"]:::ai
    n_units_uncertain["<b>uncertain</b><br/><small>devkit implement uncertain</small>"]:::machine
    n_units_checks["<b>checks</b><br/><small>devkit checks --base-on-repeat</small>"]:::machine
    n_units_uncertain_step_back["<b>uncertain-step-back</b><br/><small>AI: step-back</small>"]:::ai
    n_units_unit_end["<b>unit-end</b><br/><small>CODE='$code' STEP_BACK='$back' devkit implement en</small>"]:::machine
    n_units_unit -.-> n_units_tests
    n_units_tests --> n_units_tests_written
    n_units_unit --> n_units_code
    n_units_tests_written --> n_units_code
    n_units_code --> n_units_uncertain
    n_units_uncertain --> n_units_checks
    n_units_uncertain -.-> n_units_uncertain_step_back
    n_units_checks -.-> n_units_uncertain_step_back
    n_units_checks --> n_units_unit_end
    n_units_uncertain_step_back --> n_units_unit_end
  end
  class n_units loop
  n_units_state["<b>units-state</b><br/><small>devkit implement state</small>"]:::machine
  n_step_back["<b>step-back</b><br/><small>AI: step-back</small>"]:::ai
  n_finish["<b>finish</b><br/><small>devkit implement finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '$finish$stopped'</small>"]:::machine
  n_start -.-> n_start_stopped
  n_start -.-> n_prepare
  n_prepare -.-> n_units
  n_prepare --> n_units_state
  n_units --> n_units_state
  n_units_state -.-> n_step_back
  n_units_state --> n_finish
  n_step_back --> n_finish
  n_finish --> n_result
  n_start_stopped --> n_result
```

## judge

Judge the committed change using fixed criteria, configured decision paths and quality materials. Checks and fresh quality judges run together. Missing planned tests block; unlisted decisions return to plan. Record pass, back or stop.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start judge</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit judge prepare</small>"]:::machine
  n_changes["<b>changes</b><br/><small>devkit judge changes</small>"]:::machine
  n_route["<b>route</b><br/><small>devkit judge route</small>"]:::machine
  n_launch["<b>launch</b><br/><small>devkit judge launch</small>"]:::machine
  n_checks["<b>checks</b><br/><small>devkit checks</small>"]:::machine
  n_completion["<b>completion</b><br/><small>devkit judge completion</small>"]:::machine
  n_judge_structure["<b>judge-structure</b><br/><small>AI: quality (structure)</small>"]:::ai
  n_judge_data["<b>judge-data</b><br/><small>AI: quality (data)</small>"]:::ai
  n_judge_contract["<b>judge-contract</b><br/><small>AI: quality (contract)</small>"]:::ai
  n_judge_security["<b>judge-security</b><br/><small>AI: quality (security)</small>"]:::ai
  n_judge_ui["<b>judge-ui</b><br/><small>AI: quality (ui)</small>"]:::ai
  n_judge_cost["<b>judge-cost</b><br/><small>AI: quality (cost)</small>"]:::ai
  n_judge_test["<b>judge-test</b><br/><small>AI: quality (test)</small>"]:::ai
  n_judge_scope["<b>judge-scope</b><br/><small>AI: quality (scope)</small>"]:::ai
  n_judge_readability["<b>judge-readability</b><br/><small>AI: quality (readability)</small>"]:::ai
  n_judge_document["<b>judge-document</b><br/><small>AI: quality (document)</small>"]:::ai
  n_collect["<b>collect</b><br/><small>JUDGE_STRUCTURE='$structure' JUDGE_DATA='$data' JU</small>"]:::machine
  n_finish["<b>finish</b><br/><small>CHANGES='$changes' GATE='$gate' COMPLETION='$compl</small>"]:::machine
  n_start -.-> n_prepare
  n_prepare -.-> n_changes
  n_changes -.-> n_route
  n_route -.-> n_launch
  n_changes -.-> n_checks
  n_changes -.-> n_completion
  n_launch -.-> n_judge_structure
  n_launch -.-> n_judge_data
  n_launch -.-> n_judge_contract
  n_launch -.-> n_judge_security
  n_launch -.-> n_judge_ui
  n_launch -.-> n_judge_cost
  n_launch -.-> n_judge_test
  n_launch -.-> n_judge_scope
  n_launch -.-> n_judge_readability
  n_launch -.-> n_judge_document
  n_launch -.-> n_collect
  n_judge_structure -.-> n_collect
  n_judge_data -.-> n_collect
  n_judge_contract -.-> n_collect
  n_judge_security -.-> n_collect
  n_judge_ui -.-> n_collect
  n_judge_cost -.-> n_collect
  n_judge_test -.-> n_collect
  n_judge_scope -.-> n_collect
  n_judge_readability -.-> n_collect
  n_judge_document -.-> n_collect
  n_start -.-> n_finish
  n_prepare -.-> n_finish
  n_changes -.-> n_finish
  n_launch -.-> n_finish
  n_checks -.-> n_finish
  n_completion -.-> n_finish
  n_collect -.-> n_finish
```

## fix

The fix part. Input: the commit checked out (HEAD) and the findings of the code judgments the record binds to it (blocking, not the owner's), with the latest requirements and, when there is one, the latest plan; the owner's answers when the session passes them (input `answers`). Input `review` supplies verbatim review comments or CI failure text with provenance instead of judge findings. It requires passed requirements and plan and the latest judged change to match HEAD by PR identity. Items outside the requirements or plan, including objections to review items, return back to requirements. Stops without fixing when this fix would exceed the configured limits.fixes since the last implement. Otherwise a fresh session fixes the findings, or objects to one with evidence; a fresh dispute judge that neither made the finding nor fixes it accepts or rejects each objection (accepted: not fixed; rejected: fixed in the next pass). The checks the change needs run after each pass, at most 3 passes. A pass whose checks passed is committed with the run and the commit it fixed as trailers; the judge part judges the new commit. A decision only the owner can make returns back to plan with a pending owner row; a decision of the plan that does not hold, or a check that keeps failing, goes to a fresh judge that names the premise and where to go back. Ends with pass, back or stop, written to the record by devkit record result with the dispute judgments. Output: the result as recorded, or the start's output when the start conditions did not hold.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start fix</small>"]:::machine
  n_start_stopped["<b>start-stopped</b><br/><small>printf '%s' '$value'</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit fix prepare</small>"]:::machine
  n_proceed["<b>proceed</b><br/><small>devkit fix proceed</small>"]:::machine
  subgraph n_repair ["repair (loop, max 3)"]
    direction TB
    n_repair_fix_code["<b>fix-code</b><br/><small>AI: fix</small>"]:::ai
    n_repair_objections["<b>objections</b><br/><small>devkit fix objections</small>"]:::machine
    n_repair_dispute["<b>dispute</b><br/><small>AI: dispute</small>"]:::ai
    n_repair_dispute_end["<b>dispute-end</b><br/><small>devkit fix dispute-end</small>"]:::machine
    n_repair_checks["<b>checks</b><br/><small>devkit checks --base-on-repeat</small>"]:::machine
    n_repair_round_end["<b>round-end</b><br/><small>CODE='$code' devkit fix round-end</small>"]:::machine
    n_repair_fix_code --> n_repair_objections
    n_repair_objections -.-> n_repair_dispute
    n_repair_objections --> n_repair_dispute_end
    n_repair_dispute --> n_repair_dispute_end
    n_repair_dispute_end --> n_repair_checks
    n_repair_checks --> n_repair_round_end
  end
  class n_repair loop
  n_fix_state["<b>fix-state</b><br/><small>devkit fix state</small>"]:::machine
  n_step_back["<b>step-back</b><br/><small>AI: step-back</small>"]:::ai
  n_finish["<b>finish</b><br/><small>devkit fix finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '$finish$stopped'</small>"]:::machine
  n_start -.-> n_start_stopped
  n_start -.-> n_prepare
  n_prepare --> n_proceed
  n_proceed -.-> n_repair
  n_proceed --> n_fix_state
  n_repair --> n_fix_state
  n_fix_state -.-> n_step_back
  n_fix_state --> n_finish
  n_step_back --> n_finish
  n_finish --> n_result
  n_start_stopped --> n_result
```

## pr

Verify that passing judgments reach HEAD, record explicit allow or deny answers, ask about ask-changes not covered by owner rows' touches, assemble and judge the PR body for at most three rounds, then push and open or update the pull request with gh. Never merge.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start pr</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>printf '%s' '$value'</small>"]:::machine
  n_allow["<b>allow</b><br/><small>devkit pr answers</small>"]:::machine
  n_facts["<b>facts</b><br/><small>devkit pr facts</small>"]:::machine
  n_questions["<b>questions</b><br/><small>AI: questions</small>"]:::ai
  subgraph n_body ["body (loop, max 3)"]
    direction TB
    n_body_body_write["<b>body-write</b><br/><small>AI: write</small>"]:::ai
    n_body_body_assemble["<b>body-assemble</b><br/><small>devkit pr assemble</small>"]:::machine
    n_body_body_judge["<b>body-judge</b><br/><small>AI: judge</small>"]:::ai
    n_body_body_record["<b>body-record</b><br/><small>devkit pr judgment</small>"]:::machine
    n_body_body_write --> n_body_body_assemble
    n_body_body_assemble --> n_body_body_judge
    n_body_body_judge --> n_body_body_record
  end
  class n_body loop
  n_report_state["<b>report-state</b><br/><small>devkit pr state</small>"]:::machine
  n_open["<b>open</b><br/><small>devkit pr open</small>"]:::machine
  n_finish["<b>finish</b><br/><small>devkit pr finish</small>"]:::machine
  n_result["<b>result</b><br/><small>devkit pr output '$started' '$finished' '$opened'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_allow
  n_allow -.-> n_facts
  n_facts -.-> n_questions
  n_facts -.-> n_body
  n_body --> n_report_state
  n_report_state -.-> n_open
  n_allow --> n_finish
  n_facts --> n_finish
  n_report_state --> n_finish
  n_open --> n_finish
  n_questions --> n_finish
  n_not_started --> n_result
  n_finish --> n_result
```

## sync

Fetch and rebase onto the configured base branch. Resolve conflicts for at most five rounds, abort and restore when both changes cannot hold or conflicts remain, then run configured checks. Return needs_judge after conflict resolution. Do not push.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit record start sync</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>printf '%s' '$value'</small>"]:::machine
  n_rebase["<b>rebase</b><br/><small>devkit sync start</small>"]:::machine
  subgraph n_conflicts ["conflicts (loop, max 5)"]
    direction TB
    n_conflicts_resolve["<b>resolve</b><br/><small>AI: resolve</small>"]:::ai
    n_conflicts_continue["<b>continue</b><br/><small>devkit sync continue</small>"]:::machine
    n_conflicts_resolve --> n_conflicts_continue
  end
  class n_conflicts loop
  n_rebased["<b>rebased</b><br/><small>devkit sync state</small>"]:::machine
  n_checks["<b>checks</b><br/><small>devkit checks</small>"]:::machine
  n_finish["<b>finish</b><br/><small>devkit sync finish</small>"]:::machine
  n_result["<b>result</b><br/><small>devkit sync output '$started' '$finished'</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_rebase
  n_rebase -.-> n_conflicts
  n_start -.-> n_rebased
  n_rebase -.-> n_rebased
  n_conflicts -.-> n_rebased
  n_rebased -.-> n_checks
  n_rebased --> n_finish
  n_checks --> n_finish
  n_not_started --> n_result
  n_finish --> n_result
```

## retro

Turn the checkout's review backlog into traced work units, mechanical checks and standard edits. Review up to three drafts, commit the proposal and ask one approval question. The proposal commit contains the edits and consumed backlog deletions. Restart with approve=<version> to publish the PR and create unit Issues from that clean branch head.

```mermaid
flowchart TD
  classDef machine fill:#f5f5f5,stroke:#666,color:#333
  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000
  classDef loop fill:#fff8e1,stroke:#d6b656
  n_start["<b>start</b><br/><small>devkit retro start</small>"]:::machine
  n_not_started["<b>not-started</b><br/><small>devkit retro not-started</small>"]:::machine
  n_approve["<b>approve</b><br/><small>devkit retro approve</small>"]:::machine
  n_prepare["<b>prepare</b><br/><small>devkit retro prepare</small>"]:::machine
  n_empty["<b>empty</b><br/><small>devkit retro empty</small>"]:::machine
  subgraph n_draft ["draft (loop, max 3)"]
    direction TB
    n_draft_write["<b>write</b><br/><small>AI: write</small>"]:::ai
    n_draft_check["<b>check</b><br/><small>devkit retro check</small>"]:::machine
    n_draft_judge["<b>judge</b><br/><small>AI: judge</small>"]:::ai
    n_draft_round["<b>round</b><br/><small>devkit retro round</small>"]:::machine
    n_draft_write --> n_draft_check
    n_draft_check -.-> n_draft_judge
    n_draft_check --> n_draft_round
    n_draft_judge --> n_draft_round
  end
  class n_draft loop
  n_verify["<b>verify</b><br/><small>devkit retro verify</small>"]:::machine
  n_verification["<b>verification</b><br/><small>AI: verification</small>"]:::ai
  n_finish["<b>finish</b><br/><small>devkit retro finish</small>"]:::machine
  n_result["<b>result</b><br/><small>printf '%s' '${finished:-${approved:-${no_work:-$s</small>"]:::machine
  n_start -.-> n_not_started
  n_start -.-> n_approve
  n_start -.-> n_prepare
  n_prepare -.-> n_empty
  n_prepare -.-> n_draft
  n_draft --> n_verify
  n_verify -.-> n_verification
  n_verify --> n_finish
  n_verification --> n_finish
  n_not_started --> n_result
  n_approve --> n_result
  n_empty --> n_result
  n_finish --> n_result
```
