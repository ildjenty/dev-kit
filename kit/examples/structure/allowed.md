## Request

The agreed requirement is only to add a function calculating tax-inclusive amounts from a local file. No external connection, persistence, or substitution is needed. The existing locations compared put calculation functions in pricing. Check the following placement and dependencies.

## Target

# Placement of tax-inclusive amount calculation

- Placement: Put calculation in pricing's calculate, with caller passing validated amounts. Do not update mutable state.
- Alternative: Reject adding forwarding layers because they add functions to follow without adding contracts, knowledge, or constraints.
- Fixed name: Keep the generated input type name PriceV1 required by external code generation rules. schema.name in the generation definition api.json specifies PriceV1; changing it would break references on regeneration.
- Exemption: Waive only the naming requirement of principle 1 of principles/structure.md. caller accepts generated types, while pricing retains calculation responsibility. Follow existing pricing naming and error formats.
