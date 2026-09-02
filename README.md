# Gooo Evolution Transaction Coordinator

This repository implements an executable coordinator for composing two or
more independently typed `.gooo` rewrite candidates. The semantic authority is
[`evolution-transaction-coordinator.gooo`](.gooo/evolution-transaction-coordinator.gooo):
it declares the effect vocabulary, capability vocabulary, identity/release
contract, UNKNOWN frontier, resolution precedence, append-only denominator,
wave planner, proof selections, atomic-abort policy, and candidate-bundle
emission rule. Go is the parser, lowerer, fixture
executor, and exact-vector verifier.

## Semantic work-wave planner

This repository is also the authoritative home of the independent semantic
work-wave planner. Its source is
[`semantic-work-wave-planner.gooo`](.gooo/semantic-work-wave-planner.gooo),
not an inference prompt: candidate operation identities, immutable release
digests, dependency edges, semantic read/write sets, authority scopes, and
target repository/ledger identities are explicit input evidence.

The planner emits exact parallel waves, serialized boundaries, a blocked causal
frontier, single-writer cells, denied operations, a deterministic semantic
vector digest, and a human rationale dossier. `WRITE_WRITE`, `WRITE_READ`,
shared mutable authority, dependency, and single-ledger-writer evidence are
never placed in the same wave. Explicit disjoint read-only operations remain
parallel candidates. Missing footprint/dependency/authority evidence is
`UNKNOWN` with exactly six fields; forged identity, forbidden authority,
cycles, and forced concurrent conflicts are `REFUTED`.

```text
gooo-semantic-work-wave-planner conformance \
  --source .gooo/semantic-work-wave-planner.gooo \
  --contract contracts/semantic-work-wave-planner-v1.json \
  --corpus fixtures/semantic-work-wave-planner \
  --source-root . \
  --out /caller-owned/output
```

The semantic plan is byte-identical on replay of the same immutable input.
The planner has runtime mutation authority `0`; it does not execute candidate
operations or mutate a repository, commit, merge, or release. `FIXED_POINT`,
`FOUNDATION`/`COHERENCE`/`REGRESSION`, and
`DRIVER`/`OUTCOME`/`GUARDRAIL` are separate declared dimensions. No aggregate
score, percentage, or heuristic optimization is emitted. Improvement remains
`null` plus `UNKNOWN` unless exact matched before/after evidence is supplied.

The planner's fixed denominator contains ten corpus cases: three normal
closures, three UNKNOWN frontiers, and four REFUTED boundaries. GitHub Actions
is the only validation authority for this product and records build/test/wall/
RSS/cache metrics; root `README.md` is excluded from the exact inventory.

The coordinator computes each candidate's origin, capabilities, effect
preconditions/postconditions, and read/write semantic footprint. It enumerates
every candidate permutation and applies it to a fresh copy of the compiler
fixture under a caller-owned temporary directory. For every order it records
the generated artifact digest map, terminal reason, ordered effect trace, and
applied candidates. A case is CLOSED only when all permitted permutations have
the same exact semantic vector. Candidate successes are never summed into a
score. Orthogonal candidates share a deterministic parallel wave; overlapping
write sets, semantic authorities, or repository writers are serialized. A
ledger adoption is reserved for one writer and forced into an atomic final
wave. UNKNOWN and REFUTED frontiers are recorded per lane, so only causal
dependencies block.

The released v1 denominator remains unchanged in
[`contracts/denominator-v1.json`](contracts/denominator-v1.json). The v2
contract is append-only and adds the final-ledger-adoption case:

| case | expected result | purpose |
|---|---|---|
| `case-01-disjoint-commuting` | `CLOSED` | disjoint read/write footprints commute |
| `case-02-overlapping-equivalent` | `CLOSED` | overlapping equivalent writes remain exact |
| `case-03-explicit-footprint-conflict` | `REFUTED` | non-equivalent writes are rejected atomically |
| `case-04-order-dependent-terminal-reason` | `REFUTED` | order changes terminal reason and vector |
| `case-05-forbidden-combined-effect` | `REFUTED` | combined effect escapes the declared policy |
| `case-06-missing-footprint` | `UNKNOWN` | missing semantic footprint keeps the frontier open |
| `case-07-replay` | `CLOSED` | replayed permutations are byte-stable semantically |
| `case-08-final-ledger-adoption` | `CLOSED` | one single-writer atomic ledger adoption is the final wave |

Resolution is declared and enforced as `REFUTED > UNKNOWN > CLOSED`. Every
UNKNOWN contains exactly `stage`, `step`, `reason`, `unknown_class`,
`next_operation`, and `blocked_by`. UNKNOWN and REFUTED cases set
`atomic_abort=true`, emit no promoted bundle, and never promote a partial
fixture artifact. Only CLOSED cases write
`bundles/<case-id>/combined-candidate-bundle.gooo`.

Each fixed case also declares an `indicator_class` independently from its
`proof_choice`: `DRIVER`, `OUTCOME`, or `GUARDRAIL`. The class is preserved in
the source, denominator IR, case IR, report, and promoted bundle.

The evidence includes append-only historical identities for the v0.2.0
`AppliedCandidates` empty-vector incident and its v0.2.1 receipt fix. These
records carry release, annotated-tag, pull-request, merge, Actions-run, and
receipt-asset identities. An unavailable identity is serialized as `null` and
the incident state is `UNKNOWN`; it is never inferred. Historical immutable
releases and their receipt assets are not rewritten.

## Run in caller-owned directories

The output and fixture directories must be empty and outside the input source
root. The command does not write the input repository.

```text
go run ./cmd/gooo-evolution-transaction-coordinator generate \
  --source .gooo/evolution-transaction-coordinator.gooo \
  --contract contracts/denominator-v2.json \
  --candidates-root examples/candidates \
  --fixture-root /caller-owned/fixture \
  --out /caller-owned/output \
  --source-root .
```

The fixed run writes `evidence.json`, `coordinator-report.md`, and exactly
four promoted bundle files to the output directory. Candidate files may also
be supplied explicitly with repeated `--candidate` flags; the parser requires
at least two typed candidates.

Inventory counts exclude only the root `README.md`. Generated files/bytes
count promoted bundle files, while `artifact_count` counts every output file.
The evidence records exact integer inventory, generated bytes, wall time, peak
RSS, and test totals/selected/executed/reused/failed/unknown. CI run identity
and metrics are kept in a separate `metrics.ci` object from local evaluator
timings and the `authority.local_format_executions` incident. CI durations are
`null` when completed-run timing is unavailable at evidence generation; no
improvement estimate is emitted.

## Authority boundary

The coordinator has zero source-mutation, repository-write, commit, merge,
release-mutation, and local-test authority. It only writes the caller-owned
fixture and output directories. It does not invoke a rewriter or sandbox and
does not require a mutable repository. Operator authoring and CI runtime
authority are separate and both remain zero. A future immutable-digest adapter can
be added at the boundary without changing this evaluator.

GitHub Actions is the validation authority. The workflow uses Go 1.27 for
formatting, vet, tests, build, generated conformance, and the separate
semantic audit. Every PR also has a fail-closed release-boundary job that
requires the GitHub immutable-releases repository setting. A release tag is
accepted only after the Release API reports `immutable=true` and every asset
has a sha256 digest. The manual immutable-release workflow creates a draft,
uploads the CI evidence asset, publishes once, and then verifies the immutable
Release API object and exact annotated-tag target. Local validation is
intentionally not used for release claims.
