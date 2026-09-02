#!/usr/bin/env bash
set -euo pipefail

evidence=${1:?planner evidence directory is required}
test -f .gooo/semantic-work-wave-planner.gooo
test -f contracts/semantic-work-wave-planner-v1.json
test -f "$evidence/conformance-summary.json"

grep -q '^gooo semantic_work_wave_planner v1$' .gooo/semantic-work-wave-planner.gooo
grep -q '^precedence REFUTED>UNKNOWN>CLOSED$' .gooo/semantic-work-wave-planner.gooo
grep -q '^unknown_fields stage,step,reason,unknown_class,next_operation,blocked_by$' .gooo/semantic-work-wave-planner.gooo
grep -q '^fixed_point FIXED_POINT$' .gooo/semantic-work-wave-planner.gooo
grep -q '^runtime_authority repository_writes=0 source_mutations=0 commit_authority=0 merge_authority=0 release_mutation=0 execution_authority=0 local_test_executions=0 local_format_executions=0$' .gooo/semantic-work-wave-planner.gooo
for conflict in WRITE_WRITE WRITE_READ MUTABLE_AUTHORITY SINGLE_LEDGER_WRITER DEPENDENCY; do
  grep -q "^conflict $conflict$" .gooo/semantic-work-wave-planner.gooo
done
for proof in FOUNDATION COHERENCE REGRESSION; do
  grep -q "^proof $proof$" .gooo/semantic-work-wave-planner.gooo
done
for indicator in DRIVER OUTCOME GUARDRAIL; do
  grep -q "^indicator $indicator$" .gooo/semantic-work-wave-planner.gooo
done

jq -e '
  .cell_count == 10 and .fixed == true and (.cases | length) == 10 and
  ([.cases[].expected] | sort) == ["CLOSED","CLOSED","CLOSED","REFUTED","REFUTED","REFUTED","REFUTED","UNKNOWN","UNKNOWN","UNKNOWN"]
' contracts/semantic-work-wave-planner-v1.json >/dev/null
jq -e '
  .fixed_cases == 10 and .generated == 10 and .closed == 3 and .unknown == 3 and .refuted == 4 and
  .replay_equal == true and .authority.execution_authority == 0 and
  .authority.local_test_executions == 0 and .authority.local_format_executions == 0 and
  .metrics.inventory.root_readme_excluded == true and
  .metrics.improvement.state == "UNKNOWN" and .metrics.improvement.value == null
' "$evidence/conformance-summary.json" >/dev/null

jq -e '.state == "CLOSED" and (.waves | length) == 1 and .waves[0].parallel == true and (.serialized_boundaries | length) == 0' "$evidence/cases/case-01-disjoint-read-only/semantic-plan.json" >/dev/null
jq -e '.state == "CLOSED" and ([.serialized_boundaries[].kinds[]] | index("WRITE_READ")) != null and (.waves | length) == 2' "$evidence/cases/case-02-write-read-serialization/semantic-plan.json" >/dev/null
jq -e '.state == "CLOSED" and .waves[-1].final == true and (.waves[-1].candidate_ids | length) == 1 and (.single_writer_cells | length) == 1' "$evidence/cases/case-03-final-ledger-writer/semantic-plan.json" >/dev/null
jq -e '.state == "UNKNOWN" and ([.blocked_causal_frontier[0].unknown | keys[]] | sort) == ["blocked_by","next_operation","reason","stage","step","unknown_class"]' "$evidence/cases/case-04-missing-footprint/semantic-plan.json" >/dev/null
jq -e '.state == "UNKNOWN" and (.waves | length) == 0' "$evidence/cases/case-05-missing-dependency/semantic-plan.json" >/dev/null
jq -e '.state == "UNKNOWN" and (.blocked_causal_frontier | length) == 1' "$evidence/cases/case-06-missing-authority/semantic-plan.json" >/dev/null
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | any(startswith("FORGED_RELEASE_IDENTITY")))' "$evidence/cases/case-07-forged-identity/semantic-plan.json" >/dev/null
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | any(startswith("FORBIDDEN_AUTHORITY")))' "$evidence/cases/case-08-forbidden-authority/semantic-plan.json" >/dev/null
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | all(startswith("CYCLE_DETECTED")))' "$evidence/cases/case-09-cycle/semantic-plan.json" >/dev/null
jq -e '.state == "REFUTED" and ([.denied_operations[].reason] | all(startswith("FORCED_CONCURRENT_CONFLICT")))' "$evidence/cases/case-10-forced-concurrent-writes/semantic-plan.json" >/dev/null

printf 'semantic planner audit: closed\n'
