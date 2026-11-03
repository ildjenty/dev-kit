## Request

The agreed requirement is only to add a function calculating tax-inclusive amounts from a local file. No external connection, persistence, or substitution is needed. The existing locations compared put calculation functions in pricing. Check the following placement and dependencies.

## Target

# Placement of tax-inclusive amount calculation

- Placement: Put calculation in calculate in pricing, which handles amount calculations. Follow existing pricing naming and error formats.
- Call path: caller → calculate.
- Alternative: Reject adding service and manager because they add two functions to follow without adding hidden knowledge or enforced constraints.
- Boundary: Read and validate the file in caller, then pass the amount. calculate does not update mutable state.
