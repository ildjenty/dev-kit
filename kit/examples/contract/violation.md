## Request

Check the input contract for approval state. The requirement is that an approved state always has an approver. Only the save function accepts external JSON, which is stored as received.

## Target

# Approval input

- Input: status is draft or approved; approved_by is an optional string.
- Acceptance: Store without validating approved_by even when status is approved. Storage has no constraint either.
- Failures: Return storage failures as errors.
