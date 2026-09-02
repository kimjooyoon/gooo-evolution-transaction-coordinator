#!/usr/bin/env bash
set -euo pipefail

evidence=${1:?evidence directory is required}
test -f .gooo/evolution-transaction-coordinator.gooo
test -f contracts/denominator-v1.json
test -f contracts/denominator-v2.json
test -f "$evidence/evidence.json"

grep -q '^gooo evolution_transaction_coordinator v2$' .gooo/evolution-transaction-coordinator.gooo
grep -q '^precedence REFUTED>UNKNOWN>CLOSED$' .gooo/evolution-transaction-coordinator.gooo
grep -q '^unknown_fields stage,step,reason,unknown_class,next_operation,blocked_by$' .gooo/evolution-transaction-coordinator.gooo
grep -q '^atomic_abort states=UNKNOWN,REFUTED promote_bundle=false partial_promotion=false$' .gooo/evolution-transaction-coordinator.gooo
grep -q '^bundle closed_only=true artifact=combined-candidate-bundle.gooo digest=sha256$' .gooo/evolution-transaction-coordinator.gooo
grep -q 'indicator_class=DRIVER' .gooo/evolution-transaction-coordinator.gooo
grep -q 'indicator_class=OUTCOME' .gooo/evolution-transaction-coordinator.gooo
grep -q 'indicator_class=GUARDRAIL' .gooo/evolution-transaction-coordinator.gooo
jq -e '
  .cell_count == 7 and .fixed == true and (.cases | length) == 7 and
  ([.cases[].expected] | sort) == ["CLOSED", "CLOSED", "CLOSED", "REFUTED", "REFUTED", "REFUTED", "UNKNOWN"]
' contracts/denominator-v1.json >/dev/null
jq -e '
  .cell_count == 8 and .fixed == true and .append_only_from == "contracts/denominator-v1.json" and
  (.cases | length) == 8 and
  ([.cases[].indicator_class] | sort | unique) == ["DRIVER", "GUARDRAIL", "OUTCOME"] and
  ([.cases[].proof.choice] | sort | unique) == ["COHERENCE", "FOUNDATION", "REGRESSION"]
' contracts/denominator-v2.json >/dev/null
jq -e '
  .summary == {generated:8,closed:4,unknown:1,refuted:3} and
  ([.cases[].state] == ["CLOSED","CLOSED","REFUTED","REFUTED","REFUTED","UNKNOWN","CLOSED","CLOSED"]) and
  ([.cases[] | select((.indicator_class == "DRIVER" or .indicator_class == "OUTCOME" or .indicator_class == "GUARDRAIL") and (.proof.choice != .indicator_class))] | length) == 8 and
  .authority.local_test_executions == 0 and .authority.local_format_executions == 3 and
  .operational_incidents == ["OPERATIONAL_REFUTED", "LOCAL_FORMAT_EXECUTED"] and
  ([.cases[] | select(.state == "UNKNOWN") | .unknown | keys | sort] | all(. == ["blocked_by","next_operation","reason","stage","step","unknown_class"])) and
  ([.cases[] | select(.id == "case-06-missing-footprint") | select(.unknown.blocked_by == ["missing-footprint"])] | length) == 1 and
  ([.cases[] | select(.id == "case-06-missing-footprint") | .lanes[] | select(.candidate_id == "missing-footprint" and .state == "UNKNOWN") | .unknown | keys | sort] | all(. == ["blocked_by","next_operation","reason","stage","step","unknown_class"])) and
  ([.cases[] | select(.state == "REFUTED") | select(.atomic_abort == true and .promoted_bundle == false)] | length) == 3 and
  ([.cases[] | select(.state == "CLOSED") | select(.bundle != null and .promoted_bundle == true and .atomic_abort == false)] | length) == 4 and
  ([.cases[].permutations[] | select((.generated_artifact.digest | startswith("sha256:")) and (.terminal_reason != "") and (.ordered_effect_trace | length) > 0)] | length) == 15 and
  ([.incidents[] | select(.state == "CONFIRMED" and .release.release_id != null and .release.immutable == true and .pull_request.number != null and .merge.commit_sha != null and .run.id != null and .receipt.asset_id != null and (.receipt.digest | startswith("sha256:")))] | length) == 2 and
  ([.incidents[] | select(.id == "v0.2.0-applied-candidates-empty-vector") | select(.release.release_id == 381177402 and .release.tag == "v0.2.0" and .pull_request.number == 3 and .run.id == 33621988146 and .merge.commit_sha == "3e33b1ce3aab953ed6ffb2533b39c2df6f2d8e63" and .receipt.asset_id == 541043015 and .receipt.digest == "sha256:90697ebb6c18441fd130414e3037fd8cda93e4e7e8df5e33d560fca89938d3a8")] | length) == 1 and
  ([.incidents[] | select(.id == "v0.2.1-applied-candidates-receipt-fix") | select(.release.release_id == 381179864 and .release.tag == "v0.2.1" and .pull_request.number == 4 and .run.id == 33622347822 and .merge.commit_sha == "4234b39ea044609cd12b55d50c403b7f9dcd99c0" and .receipt.asset_id == 541048352 and .receipt.digest == "sha256:709795a9c93745f575386e42f45e430483c3198a314540d75f96f26a6b67d0eb")] | length) == 1 and
  ([.incidents[] | select(.state == "UNKNOWN") | select(.release.release_id == null and .pull_request.number == null and .merge.commit_sha == null and .run.id == null and .receipt.asset_id == null and .receipt.digest == null)] | length) == ([.incidents[] | select(.state == "UNKNOWN")] | length) and
  .metrics.improvement_state == "UNKNOWN" and .metrics.improvement == null and
  .metrics.ci.state == "UNKNOWN" and .metrics.ci.wall_ms == null and .metrics.ci.build_ms == null and .metrics.ci.test_ms == null
' "$evidence/evidence.json" >/dev/null
printf 'semantic audit: closed\n'
