#!/usr/bin/env bash
set -euo pipefail

bin=${1:?path to semantic planner binary is required}
work=$(mktemp -d "${RUNNER_TEMP:-/tmp}/gooo-semantic-work-wave-planner.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/first" "$work/second"

before=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
"$bin" conformance \
  --source .gooo/semantic-work-wave-planner.gooo \
  --contract contracts/semantic-work-wave-planner-v1.json \
  --corpus fixtures/semantic-work-wave-planner \
  --source-root . \
  --out "$work/first"
after=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before" = "$after"

"$bin" conformance \
  --source .gooo/semantic-work-wave-planner.gooo \
  --contract contracts/semantic-work-wave-planner-v1.json \
  --corpus fixtures/semantic-work-wave-planner \
  --source-root . \
  --out "$work/second"

jq -e '
  .fixed_cases == 10 and .generated == 10 and .closed == 3 and
  .unknown == 3 and .refuted == 4 and .replay_equal == true and
  .authority == {repository_writes:0,source_mutations:0,commit_authority:0,merge_authority:0,release_mutation:0,execution_authority:0,local_test_executions:0,local_format_executions:0} and
  .metrics.improvement.state == "UNKNOWN" and .metrics.improvement.value == null
' "$work/first/conformance-summary.json"

test "$(find "$work/first/cases" -name semantic-plan.json -type f | wc -l | tr -d ' ')" = 10
for case in \
  case-01-disjoint-read-only \
  case-02-write-read-serialization \
  case-03-final-ledger-writer \
  case-04-missing-footprint \
  case-05-missing-dependency \
  case-06-missing-authority \
  case-07-forged-identity \
  case-08-forbidden-authority \
  case-09-cycle \
  case-10-forced-concurrent-writes; do
  cmp -s "$work/first/cases/$case/semantic-plan.json" "$work/second/cases/$case/semantic-plan.json"
done

jq -e '
  .plan.schema == "gooo/semantic-work-wave-planner/plan/v1" and
  (.plan.semantic_vector_digest | startswith("sha256:"))
' "$work/first/cases/case-01-disjoint-read-only/evidence.json"
jq -e '
  .state == "CLOSED" and (.waves | length) == 1 and
  .waves[0].parallel == true and (.waves[0].candidate_ids | length) == 2 and
  (.serialized_boundaries | length) == 0
' "$work/first/cases/case-01-disjoint-read-only/semantic-plan.json"
jq -e '
  .state == "CLOSED" and (.waves | length) == 2 and
  ([.serialized_boundaries[].kinds[]] | index("WRITE_READ")) != null
' "$work/first/cases/case-02-write-read-serialization/semantic-plan.json"
jq -e '
  .state == "CLOSED" and (.waves[-1].final == true) and
  (.waves[-1].candidate_ids | length) == 1 and
  (.single_writer_cells | length) == 1
' "$work/first/cases/case-03-final-ledger-writer/semantic-plan.json"
jq -e '
  .state == "UNKNOWN" and (.blocked_causal_frontier | length) == 1 and
  ([.blocked_causal_frontier[0].unknown | keys[]] | sort) == ["blocked_by","next_operation","reason","stage","step","unknown_class"] and
  (.waves | length) == 1
' "$work/first/cases/case-04-missing-footprint/semantic-plan.json"
jq -e '.state == "UNKNOWN" and (.waves | length) == 0' "$work/first/cases/case-05-missing-dependency/semantic-plan.json"
jq -e '.state == "UNKNOWN" and (.blocked_causal_frontier | length) == 1' "$work/first/cases/case-06-missing-authority/semantic-plan.json"
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | any(startswith("FORGED_RELEASE_IDENTITY")))' "$work/first/cases/case-07-forged-identity/semantic-plan.json"
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | any(startswith("FORBIDDEN_AUTHORITY")))' "$work/first/cases/case-08-forbidden-authority/semantic-plan.json"
jq -e '.state == "REFUTED" and (.denied_operations | length) == 2 and ([.denied_operations[].reason] | all(startswith("CYCLE_DETECTED")))' "$work/first/cases/case-09-cycle/semantic-plan.json"
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | all(startswith("FORCED_CONCURRENT_CONFLICT")))' "$work/first/cases/case-10-forced-concurrent-writes/semantic-plan.json"

rm -rf "${RUNNER_TEMP:-/tmp}/gooo-semantic-work-wave-planner-evidence"
cp -R "$work/first" "${RUNNER_TEMP:-/tmp}/gooo-semantic-work-wave-planner-evidence"
printf 'semantic planner conformance: closed\n'
