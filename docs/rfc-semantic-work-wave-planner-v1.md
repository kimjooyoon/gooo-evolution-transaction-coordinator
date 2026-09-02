# Semantic work-wave planner v1

## Authority decision

The public `kimjooyoon` repository census found that
`gooo-evolution-transaction-coordinator` already owns the matching semantic
authority: evolution transaction composition, semantic footprints, conflict
handling, causal frontiers, and single-writer ledger adoption. This planner is
therefore an additive product in that repository. It does not edit
`gooo-self-improvement-ledger` or any other active repository.

## Input contract

`gooo/semantic-work-wave-planner/input/v1` is immutable JSON. Each candidate
must identify its operation and digest, immutable input release, semantic read
and write sets, authority scope, and target repository/ledger. The top-level
input supplies the immutable release, target, and dependency edges. An empty
array is an explicit empty footprint; an omitted field is missing evidence.

## Scheduling semantics

The planner first classifies each lane. It then adds deterministic boundaries
for write/write overlap, write/read overlap, equal mutable authority, explicit
dependency edges, and a shared ledger writer. Boundaries are oriented by a
lexical canonical tie-break when no dependency supplies an order. A single
ledger writer is reserved for a final singleton wave. Ready candidates are
sorted by stable operation identity and emitted as one exact wave; this is a
canonical fixed point, not an optimization objective.

`REFUTED > UNKNOWN > CLOSED` is the only resolution precedence. Missing
footprint, dependency, authority, target, or release evidence produces an
UNKNOWN tuple with exactly `stage`, `step`, `reason`, `unknown_class`,
`next_operation`, and `blocked_by`. Forged identity or release digests,
forbidden mutation authority, cycles, and forced concurrent conflicts are
REFUTED. An UNKNOWN or REFUTED lane does not suppress an unrelated closed
lane, but its causal dependents remain on the blocked frontier.

## Evidence boundary

`semantic-plan.json` contains only deterministic semantic output and its vector
digest. `evidence.json`, `metrics.json`, and the human `rationale-dossier.md`
are caller-owned projections. The planner performs no candidate execution and
has zero repository, source, commit, merge, release, test, or formatting
authority. Improvement is `null` and `UNKNOWN` unless exact matched
before/after identities and measurements are present.

The fixed ten-case corpus is declared in the `.gooo` source and denominator
contract. GitHub Actions is the validation authority; the CI receipt records
Go 1.27 build/test duration, wall time, peak RSS, cache state, exact inventory,
and preserved failed runs. Releases are draft-first, immutable, and never
reuse a tag or delete historical assets.
