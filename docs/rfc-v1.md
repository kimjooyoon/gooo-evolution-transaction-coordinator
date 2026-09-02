# Evolution transaction coordinator v2

Version 2 is append-only over the released v1 denominator. The v1 contract is
kept unchanged for historical evidence; v2 adds an orthogonal evolution wave
and final single-writer ledger adoption cell.

## Problem

Two rewrite candidates can each be valid in isolation and still fail as a
bundle. Their semantic footprints can collide, their application order can
change a precondition, or their combined capability/effect set can escape the
allowed policy. A candidate-level pass is therefore insufficient evidence for
atomic composition.

## Protocol

1. Parse the authoritative `.gooo` source and append-only denominator contract,
   including semantic authority, repository identity, read/write sets, and
   immutable release identities.
2. Parse at least two typed candidate sources.
3. Lower each candidate into a footprint summary containing origin,
   capabilities, effect pre/postconditions, read footprint, write footprint,
   and typed operation.
4. Build a deterministic dependency graph. Disjoint candidates enter the same
   wave; overlapping writes, authorities, and repository writers serialize.
   Ledger adoption has one writer and is the atomic final wave.
5. Enumerate every permitted candidate order within that plan.
6. Copy the compiler fixture into a fresh caller-owned directory per
   permutation and apply the operations in order.
7. Lower the applied state into generated Go, parse it with the Go parser, and
   record its artifact digest map, terminal reason, and ordered effect trace.
8. Compare every permutation as an exact vector. The vector is not a scalar
   score and cannot be replaced by a sum or weighted average.
9. Resolve the case using the declared precedence
   `REFUTED > UNKNOWN > CLOSED`.

## Promotion

Only a CLOSED case may emit a combined candidate bundle. An UNKNOWN or
REFUTED case atomically aborts, and any generated fixture state remains a
temporary observation only. The input source root is never a destination for
generated files.

## UNKNOWN frontier

The missing-footprint case demonstrates the minimum open frontier. Its claim
contains all six required fields:

```text
stage, step, reason, unknown_class, next_operation, blocked_by
```

No generic error is substituted for this tuple.

## Measurement

The evidence artifact records repository inventory with the root README
excluded, Go/Gooo file counts, physical lines, promoted generated file/byte
counts, wall milliseconds, peak RSS KiB, and the exact test cardinalities.
All counts are integers. The evaluator's repository and local-test authority
remain zero.

## Release boundary

GitHub repository release immutability is an external prerequisite. The CI
release-boundary job calls the repository immutable-releases API and fails
closed on a missing endpoint, a false `enabled` value, or an unavailable
administration read. A future release is valid only when its Release API object
reports `immutable=true` and each uploaded asset has a sha256 digest. Existing
releases are never rewritten to repair a historical immutability value.
