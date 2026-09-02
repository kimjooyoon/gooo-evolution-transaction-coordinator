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
jq -e '
  .cell_count == 7 and .fixed == true and (.cases | length) == 7 and
  ([.cases[].expected] | sort) == ["CLOSED", "CLOSED", "CLOSED", "REFUTED", "REFUTED", "REFUTED", "UNKNOWN"]
' contracts/denominator-v1.json >/dev/null
jq -e '
  .cell_count == 8 and .fixed == true and .append_only_from == "contracts/denominator-v1.json" and
  (.cases | length) == 8 and
  ([.cases[].proof.choice] | sort | unique) == ["COHERENCE", "FOUNDATION", "REGRESSION"]
' contracts/denominator-v2.json >/dev/null
jq -e '
  .summary == {generated:8,closed:4,unknown:1,refuted:3} and
  ([.cases[] | select(.state == "UNKNOWN") | .unknown | keys | sort] | all(. == ["blocked_by","next_operation","reason","stage","step","unknown_class"])) and
  ([.cases[] | select(.state == "REFUTED") | select(.atomic_abort == true and .promoted_bundle == false)] | length) == 3 and
  ([.cases[] | select(.state == "CLOSED") | select(.bundle != null and .promoted_bundle == true and .atomic_abort == false)] | length) == 4 and
  ([.cases[].permutations[] | select((.generated_artifact.digest | startswith("sha256:")) and (.terminal_reason != "") and (.ordered_effect_trace | length) > 0)] | length) == 15
' "$evidence/evidence.json" >/dev/null
printf 'semantic audit: closed\n'
