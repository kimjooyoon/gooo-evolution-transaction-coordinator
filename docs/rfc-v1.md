# Evolution transaction coordinator v1

## Problem

Two rewrite candidates can each be valid in isolation and still fail as a
bundle. Their semantic footprints can collide, their application order can
change a precondition, or their combined capability/effect set can escape the
allowed policy. A candidate-level pass is therefore insufficient evidence for
atomic composition.

## Protocol

1. Parse the authoritative `.gooo` source and fixed denominator contract.
2. Parse at least two typed candidate sources.
3. Lower each candidate into a footprint summary containing origin,
   capabilities, effect pre/postconditions, read footprint, write footprint,
   and typed operation.
4. Enumerate all permutations of the caller's candidate IDs.
5. Copy the compiler fixture into a fresh caller-owned directory per
   permutation and apply the operations in order.
6. Lower the applied state into generated Go, parse it with the Go parser, and
   record its artifact digest map, terminal reason, and ordered effect trace.
7. Compare every permutation as an exact vector. The vector is not a scalar
   score and cannot be replaced by a sum or weighted average.
8. Resolve the case using the declared precedence
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
