## Request

The agreed requirement is only to add a function calculating tax-inclusive amounts from a local file. No external connection, persistence, or substitution is needed. The existing locations compared put calculation functions in pricing. Check the following placement and dependencies.

## Target

# Placement of tax-inclusive amount calculation

- Placement: Put calculation in pricing's calculate. Follow existing pricing naming and error formats.
- Call path: caller → service → manager → calculate.
- service and manager only pass arguments and return values unchanged, without owning contracts, knowledge, or constraints.
- Alternative: caller → calculate preserves the same inputs, outputs, and boundary guarantees, adds no state, and removes two functions to follow during changes.
- Adopted: Retain service and manager despite having no reason to reject the alternative.
