# Repository ownership

This repository is the single recorder for the evolution-transaction-coordinator
scope. Do not modify sibling repositories under `/Users/alice/meta-go` from
this working tree.

The evaluator accepts only typed `.gooo` candidates and writes only to
caller-owned temporary fixture/output directories. It never mutates the input
source root and has zero commit, merge, or release authority. GitHub Actions
is the validation authority; do not run local test, build, vet, or conformance
commands when producing release evidence.
