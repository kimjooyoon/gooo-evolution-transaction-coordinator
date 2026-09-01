#!/usr/bin/env bash
set -euo pipefail

bin=${1:?path to the built coordinator binary is required}
work=$(mktemp -d "${RUNNER_TEMP:-/tmp}/gooo-evolution-transaction-coordinator.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/first-fixture" "$work/first-output" "$work/second-fixture" "$work/second-output"

before=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
"$bin" generate \
  --source .gooo/evolution-transaction-coordinator.gooo \
  --contract contracts/denominator-v1.json \
  --candidates-root examples/candidates \
  --fixture-root "$work/first-fixture" \
  --out "$work/first-output" \
  --source-root .
after=$(git status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before" = "$after"

"$bin" generate \
  --source .gooo/evolution-transaction-coordinator.gooo \
  --contract contracts/denominator-v1.json \
  --candidates-root examples/candidates \
  --fixture-root "$work/second-fixture" \
  --out "$work/second-output" \
  --source-root .

test "$(find "$work/first-output" -type f | wc -l | tr -d ' ')" = 5
jq -e '
  .schema == "gooo/evolution-transaction-coordinator/evidence/v1" and
  .fixed_case_count == 7 and
  .precedence == ["REFUTED", "UNKNOWN", "CLOSED"] and
  .summary == {generated:7,closed:3,unknown:1,refuted:3} and
  .metrics.generated.files == 3 and
  .metrics.generated.bytes > 0 and
  .metrics.tests == {total:7,selected:7,executed:7,reused:1,failed:3,unknown:1} and
  .authority == {repository_writes:0,source_mutations:0,commit_authority:0,merge_authority:0,release_mutation:0,local_test_executions:0} and
  .artifact_count == 5 and
  ([.cases[] | select(.state == "CLOSED")] | length) == 3 and
  ([.cases[] | select(.state == "UNKNOWN")] | length) == 1 and
  ([.cases[] | select(.state == "REFUTED")] | length) == 3 and
  ([.cases[] | select(.permutations | length != 2)] | length) == 0 and
  ([.cases[] | select(.state != "CLOSED" and (.atomic_abort != true or .promoted_bundle != false))] | length) == 0 and
  ([.cases[] | select(.state == "CLOSED" and (.bundle == null or .promoted_bundle != true))] | length) == 0 and
  ([.cases[] | select(.state != "CLOSED" and (.bundle != null))] | length) == 0 and
  ([.cases[] | select(.id == "case-06-missing-footprint") | .unknown | (.stage != "" and .step != "" and .reason != "" and .unknown_class != "" and .next_operation != "" and (.blocked_by | length) > 0)] | all) and
  ([.cases[] | select(.id == "case-07-replay") | .replay_equal] | all) and
  .metrics.inventory.root_readme_excluded == true and
  .metrics.inventory.files > 0 and
  .metrics.inventory.directories > 0 and
  .metrics.inventory.go_files > 0 and
  .metrics.inventory.gooo_files > 0 and
  .metrics.inventory.physical_lines > 0 and
  .metrics.wall_ms > 0 and
  .metrics.peak_rss_kib > 0
' "$work/first-output/evidence.json"

test "$(find "$work/first-output/bundles" -type f | wc -l | tr -d ' ')" = 3
test ! -e "$work/first-output/bundles/case-03-explicit-footprint-conflict"
test ! -e "$work/first-output/bundles/case-04-order-dependent-terminal-reason"
test ! -e "$work/first-output/bundles/case-05-forbidden-combined-effect"
test ! -e "$work/first-output/bundles/case-06-missing-footprint"

jq -S 'del(.metrics, .artifact_count)' "$work/first-output/evidence.json" > "$work/first-semantic.json"
jq -S 'del(.metrics, .artifact_count)' "$work/second-output/evidence.json" > "$work/second-semantic.json"
cmp -s "$work/first-semantic.json" "$work/second-semantic.json"
cmp -s "$work/first-output/bundles/case-01-disjoint-commuting/combined-candidate-bundle.gooo" "$work/second-output/bundles/case-01-disjoint-commuting/combined-candidate-bundle.gooo"
cmp -s "$work/first-output/bundles/case-02-overlapping-equivalent/combined-candidate-bundle.gooo" "$work/second-output/bundles/case-02-overlapping-equivalent/combined-candidate-bundle.gooo"
cmp -s "$work/first-output/bundles/case-07-replay/combined-candidate-bundle.gooo" "$work/second-output/bundles/case-07-replay/combined-candidate-bundle.gooo"

rm -rf "$RUNNER_TEMP/gooo-evolution-transaction-coordinator-evidence"
cp -R "$work/first-output" "$RUNNER_TEMP/gooo-evolution-transaction-coordinator-evidence"
