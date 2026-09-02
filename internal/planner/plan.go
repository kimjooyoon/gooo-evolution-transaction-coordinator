package planner

import (
	"fmt"
	"sort"
	"strings"
)

type candidateStatus struct {
	state   string
	reason  string
	unknown *Unknown
	caused  []string
}

type graph struct {
	edges map[string]map[string]bool
}

func BuildPlan(input Input, meta MetaSource, inputDigest string) (Plan, error) {
	normalizeInput(&input)
	if input.Schema != InputSchema || input.Version != "v1" {
		return Plan{}, fmt.Errorf("input schema or version mismatch")
	}
	if len(input.Candidates) < MinimumCandidate {
		return Plan{}, fmt.Errorf("at least %d candidates are required", MinimumCandidate)
	}
	target := input.Target
	byID := make(map[string]CandidateInput, len(input.Candidates))
	ids := make([]string, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		id := candidate.OperationIdentity.CandidateID
		if id == "" || byID[id].OperationIdentity.CandidateID != "" {
			return Plan{}, fmt.Errorf("candidate operation identities must be non-empty and unique")
		}
		byID[id] = candidate
		ids = append(ids, id)
	}
	sort.Strings(ids)

	statuses := make(map[string]candidateStatus, len(ids))
	for _, id := range ids {
		statuses[id] = preflightCandidate(input, meta, target, byID[id])
		if target.Repository == "" || target.Ledger == "" {
			statuses[id] = preferUnknown(statuses[id], Unknown{Stage: "TARGET", Step: "validate_target_identity", Reason: "TARGET_REPOSITORY_IDENTITY_NOT_DECLARED", UnknownClass: "MISSING_TARGET_IDENTITY", NextOperation: "DECLARE_TARGET_REPOSITORY_IDENTITY", BlockedBy: []string{id}})
		}
	}

	edges, dependencyUnknowns := buildDependencyGraph(input, byID, ids)
	for id, unknown := range dependencyUnknowns {
		status := statuses[id]
		status = preferUnknown(status, unknown)
		statuses[id] = status
	}
	cycleNodes := findCycleNodes(edges, ids)
	for _, id := range cycleNodes {
		statuses[id] = preferRefuted(statuses[id], "CYCLE_DETECTED", cycleNodes)
	}
	propagateCausalStatuses(statuses, edges, ids)

	boundaries := make([]SerializedBoundary, 0)
	for _, edge := range edgesToList(edges) {
		if edge.Before == edge.After {
			continue
		}
		boundaries = append(boundaries, SerializedBoundary{Before: edge.Before, After: edge.After, Kinds: []string{"DEPENDENCY"}, Cells: []string{"dependency:" + edge.Before + "->" + edge.After}, Rationale: "The declared dependency edge requires the predecessor to complete before the dependent operation."})
	}
	conflictEdges := make([]DependencyEdge, 0)
	forcedPairs := make(map[string]bool)
	for _, pair := range input.ForcedConcurrentPairs {
		if pair.First == "" || pair.Second == "" {
			continue
		}
		forcedPairs[pairKey(pair.First, pair.Second)] = true
	}
	for _, candidate := range input.Candidates {
		if candidate.ForcedConcurrent {
			for _, other := range ids {
				if other != candidate.OperationIdentity.CandidateID {
					forcedPairs[pairKey(candidate.OperationIdentity.CandidateID, other)] = true
				}
			}
		}
	}
	for left := 0; left < len(ids); left++ {
		for right := left + 1; right < len(ids); right++ {
			first, second := byID[ids[left]], byID[ids[right]]
			kinds, cells := conflictEvidence(first, second, target)
			if len(kinds) == 0 {
				continue
			}
			before, after := ids[left], ids[right]
			boundary := SerializedBoundary{Before: before, After: after, Kinds: kinds, Cells: cells, Rationale: boundaryRationale(kinds, cells)}
			boundaries = append(boundaries, boundary)
			if forcedPairs[pairKey(before, after)] {
				statuses[before] = preferRefuted(statuses[before], "FORCED_CONCURRENT_CONFLICT:"+strings.Join(kinds, "+"), []string{before, after})
				statuses[after] = preferRefuted(statuses[after], "FORCED_CONCURRENT_CONFLICT:"+strings.Join(kinds, "+"), []string{before, after})
				continue
			}
			if statuses[before].state == StateClosed && statuses[after].state == StateClosed {
				conflictEdges = append(conflictEdges, DependencyEdge{Before: before, After: after})
			}
		}
	}

	ledgerWriters := ledgerWriterIDs(input.Candidates, target)
	for left := 0; left < len(ledgerWriters); left++ {
		for right := left + 1; right < len(ledgerWriters); right++ {
			before, after := ledgerWriters[left], ledgerWriters[right]
			if before > after {
				before, after = after, before
			}
			boundary := SerializedBoundary{Before: before, After: after, Kinds: []string{"SINGLE_LEDGER_WRITER"}, Cells: []string{target.Ledger}, Rationale: "Ledger writes share one mutable ledger writer and are serialized."}
			boundaries = append(boundaries, boundary)
			if forcedPairs[pairKey(before, after)] {
				statuses[before] = preferRefuted(statuses[before], "FORCED_CONCURRENT_LEDGER_WRITES", []string{before, after})
				statuses[after] = preferRefuted(statuses[after], "FORCED_CONCURRENT_LEDGER_WRITES", []string{before, after})
			} else if statuses[before].state == StateClosed && statuses[after].state == StateClosed {
				conflictEdges = append(conflictEdges, DependencyEdge{Before: before, After: after})
			}
		}
	}
	// One ledger writer is the only operation allowed to adopt the mutable
	// ledger. It is deterministically placed after all other eligible work.
	if len(ledgerWriters) == 1 {
		writer := ledgerWriters[0]
		for _, id := range ids {
			if id != writer && statuses[id].state == StateClosed && statuses[writer].state == StateClosed {
				conflictEdges = append(conflictEdges, DependencyEdge{Before: id, After: writer})
				boundaries = append(boundaries, SerializedBoundary{Before: id, After: writer, Kinds: []string{"SINGLE_LEDGER_WRITER"}, Cells: []string{target.Ledger}, Rationale: "The single ledger writer is reserved for the final atomic wave."})
			}
		}
	}

	allEdges := append([]DependencyEdge{}, edgesToList(edges)...)
	allEdges = append(allEdges, conflictEdges...)
	allEdges = uniqueEdges(allEdges)
	finalGraph := graphFromEdges(allEdges, ids)
	cycleNodes = findCycleNodes(finalGraph, ids)
	for _, id := range cycleNodes {
		statuses[id] = preferRefuted(statuses[id], "CYCLE_DETECTED", cycleNodes)
	}
	propagateCausalStatuses(statuses, finalGraph, ids)

	waves := planEligibleWaves(statuses, finalGraph, ids)
	decisions := make([]CandidateDecision, 0, len(ids))
	for _, id := range ids {
		status := statuses[id]
		decisions = append(decisions, CandidateDecision{CandidateID: id, State: status.state, Reason: status.reason})
	}
	blocked := make([]BlockedCausalFrontier, 0)
	denied := make([]DeniedOperation, 0)
	for _, id := range ids {
		status := statuses[id]
		if status.state == StateUnknown && status.unknown != nil {
			blocked = append(blocked, BlockedCausalFrontier{CandidateID: id, Unknown: *status.unknown})
		}
		if status.state == StateRefuted {
			denied = append(denied, DeniedOperation{CandidateID: id, Reason: status.reason, CausedBy: stableIDs(append(status.caused, id))})
		}
	}

	singleWriterCells := make([]SingleWriterCell, 0, len(ledgerWriters))
	for _, id := range ledgerWriters {
		writer := byID[id].AuthorityScope.LedgerWriter
		if writer == "" {
			writer = id
		}
		singleWriterCells = append(singleWriterCells, SingleWriterCell{Cell: target.Ledger, Writer: writer, CandidateID: id, Wave: waveOf(waves, id)})
	}
	sort.Slice(boundaries, func(left, right int) bool { return boundaryKey(boundaries[left]) < boundaryKey(boundaries[right]) })

	state, decision := summarizeState(statuses, ids)
	vector := SemanticVector{State: state, Decision: decision, CandidateDecisions: decisions, Waves: waves, SerializedBoundaries: boundaries, BlockedCausalFrontier: blocked, SingleWriterCells: singleWriterCells, DeniedOperations: denied}
	vectorDigest, err := DigestValue(vector)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Schema: PlanSchema, Version: "v1", InputDigest: inputDigest, Target: target, State: state, Decision: decision, CandidateDecisions: decisions, Waves: waves, SerializedBoundaries: boundaries, BlockedCausalFrontier: blocked, SingleWriterCells: singleWriterCells, DeniedOperations: denied, SemanticVectorDigest: vectorDigest}
	dossier := RenderDossier(plan)
	plan.RationaleDossierDigest = DigestBytes([]byte(dossier))
	return plan, nil
}

func preflightCandidate(input Input, meta MetaSource, target TargetIdentity, candidate CandidateInput) candidateStatus {
	status := candidateStatus{state: StateClosed, reason: "FIXED_POINT"}
	id := candidate.OperationIdentity.CandidateID
	if id == "" || candidate.OperationIdentity.Kind == "" || candidate.OperationIdentity.Origin == "" {
		status = preferUnknown(status, Unknown{Stage: "IDENTITY", Step: "validate_operation_identity", Reason: "CANDIDATE_OPERATION_IDENTITY_NOT_DECLARED", UnknownClass: "MISSING_OPERATION_IDENTITY", NextOperation: "DECLARE_CANDIDATE_OPERATION_IDENTITY", BlockedBy: stableIDs([]string{id})})
	}
	if candidate.OperationIdentity.OperationDigest == "" {
		status = preferUnknown(status, Unknown{Stage: "IDENTITY", Step: "validate_operation_identity", Reason: "OPERATION_DIGEST_NOT_DECLARED", UnknownClass: "MISSING_OPERATION_IDENTITY", NextOperation: "DECLARE_OPERATION_DIGEST", BlockedBy: stableIDs([]string{id})})
	} else if !validDigest(candidate.OperationIdentity.OperationDigest) {
		status = preferRefuted(status, "FORGED_OPERATION_IDENTITY", []string{id})
	}
	if input.InputRelease == nil || candidate.ImmutableInputRelease == nil {
		status = preferUnknown(status, Unknown{Stage: "RELEASE", Step: "resolve_immutable_input_release", Reason: "IMMUTABLE_INPUT_RELEASE_NOT_DECLARED", UnknownClass: "MISSING_IMMUTABLE_RELEASE", NextOperation: "DECLARE_IMMUTABLE_INPUT_RELEASE_DIGEST", BlockedBy: stableIDs([]string{id})})
	} else {
		if !validRelease(*input.InputRelease) || !validRelease(*candidate.ImmutableInputRelease) {
			status = preferRefuted(status, "FORGED_RELEASE_IDENTITY", []string{id})
		} else if *input.InputRelease != *candidate.ImmutableInputRelease || *input.InputRelease != meta.ImmutableInputRelease {
			status = preferRefuted(status, "FORGED_RELEASE_IDENTITY", []string{id})
		}
	}
	if candidate.SemanticReadSet == nil || candidate.SemanticWriteSet == nil {
		status = preferUnknown(status, Unknown{Stage: "FOOTPRINT", Step: "validate_semantic_read_write_set", Reason: "SEMANTIC_READ_WRITE_FOOTPRINT_NOT_DECLARED", UnknownClass: "MISSING_SEMANTIC_FOOTPRINT", NextOperation: "DECLARE_SEMANTIC_READ_WRITE_FOOTPRINT", BlockedBy: stableIDs([]string{id})})
	}
	if candidate.AuthorityScope == nil || candidate.AuthorityScope.Permissions == nil {
		status = preferUnknown(status, Unknown{Stage: "AUTHORITY", Step: "validate_authority_scope", Reason: "AUTHORITY_SCOPE_NOT_DECLARED", UnknownClass: "MISSING_AUTHORITY_SCOPE", NextOperation: "DECLARE_AUTHORITY_SCOPE", BlockedBy: stableIDs([]string{id})})
	} else {
		for _, permission := range candidate.AuthorityScope.Permissions {
			if contains(meta.ForbiddenAuthorities, permission) || permission == "MUTATE" {
				status = preferRefuted(status, "FORBIDDEN_AUTHORITY:"+permission, []string{id})
			}
		}
		if candidate.AuthorityScope.RuntimeMutationAuthority != 0 {
			status = preferRefuted(status, "RUNTIME_MUTATION_AUTHORITY_NONZERO", []string{id})
		}
	}
	if candidate.Target == nil || candidate.Target.Repository == "" || candidate.Target.Ledger == "" {
		status = preferUnknown(status, Unknown{Stage: "TARGET", Step: "validate_target_identity", Reason: "TARGET_REPOSITORY_IDENTITY_NOT_DECLARED", UnknownClass: "MISSING_TARGET_IDENTITY", NextOperation: "DECLARE_TARGET_REPOSITORY_IDENTITY", BlockedBy: stableIDs([]string{id})})
	} else if target.Repository != "" && target.Ledger != "" && (candidate.Target.Repository != target.Repository || candidate.Target.Ledger != target.Ledger || candidate.Target.Repository != meta.RepositoryIdentity) {
		status = preferRefuted(status, "FORGED_TARGET_IDENTITY", []string{id})
	}
	return status
}

func buildDependencyGraph(input Input, byID map[string]CandidateInput, ids []string) (graph, map[string]Unknown) {
	result := graphFromEdges(nil, ids)
	unknowns := make(map[string]Unknown)
	edges := append([]DependencyEdge{}, input.DependencyEdges...)
	if input.DependencyEdges == nil {
		for _, id := range ids {
			candidate := byID[id]
			if candidate.Dependencies == nil {
				unknowns[id] = Unknown{Stage: "DEPENDENCY", Step: "validate_dependency_edges", Reason: "DEPENDENCY_EDGES_NOT_DECLARED", UnknownClass: "MISSING_DEPENDENCY_EVIDENCE", NextOperation: "DECLARE_DEPENDENCY_EDGES", BlockedBy: []string{id}}
				continue
			}
			for _, before := range candidate.Dependencies {
				edges = append(edges, DependencyEdge{Before: before, After: id})
			}
		}
	}
	for _, edge := range edges {
		if edge.Before == "" || edge.After == "" {
			continue
		}
		if _, ok := byID[edge.After]; !ok {
			continue
		}
		if _, ok := byID[edge.Before]; !ok {
			unknowns[edge.After] = Unknown{Stage: "DEPENDENCY", Step: "validate_dependency_edges", Reason: "DEPENDENCY_EDGE_REFERENCES_UNKNOWN_OPERATION", UnknownClass: "MISSING_DEPENDENCY_EVIDENCE", NextOperation: "DECLARE_DEPENDENCY_EDGE_TARGET", BlockedBy: stableIDs([]string{edge.After, edge.Before})}
			continue
		}
		result.edges[edge.Before][edge.After] = true
	}
	return result, unknowns
}

func propagateCausalStatuses(statuses map[string]candidateStatus, dependencies graph, ids []string) {
	changed := true
	for changed {
		changed = false
		for _, before := range ids {
			for after := range dependencies.edges[before] {
				prior, next := statuses[before], statuses[after]
				if prior.state == StateRefuted && next.state != StateRefuted {
					statuses[after] = preferRefuted(next, "CAUSAL_DEPENDENCY_REFUTED", []string{before})
					changed = true
				} else if prior.state == StateUnknown && next.state == StateClosed {
					statuses[after] = preferUnknown(next, Unknown{Stage: "FRONTIER", Step: "advance_blocked_causal_frontier", Reason: "CAUSAL_DEPENDENCY_BLOCKED", UnknownClass: "CAUSAL_DEPENDENCY_FRONTIER", NextOperation: "RESOLVE_BLOCKED_DEPENDENCY", BlockedBy: []string{before}})
					changed = true
				}
			}
		}
	}
}

func planEligibleWaves(statuses map[string]candidateStatus, dependencies graph, ids []string) []Wave {
	eligible := make(map[string]bool)
	for _, id := range ids {
		if statuses[id].state == StateClosed {
			eligible[id] = true
		}
	}
	indegree := make(map[string]int, len(eligible))
	for id := range eligible {
		indegree[id] = 0
	}
	for before, afters := range dependencies.edges {
		if !eligible[before] {
			continue
		}
		for after := range afters {
			if eligible[after] {
				indegree[after]++
			}
		}
	}
	waves := make([]Wave, 0)
	for len(eligible) > 0 {
		ready := make([]string, 0)
		for id := range eligible {
			if indegree[id] == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			break
		}
		sort.Strings(ready)
		wave := Wave{Ordinal: len(waves) + 1, CandidateIDs: ready, Parallel: len(ready) > 1}
		if len(ready) > 1 {
			wave.Rationale = "Candidates have disjoint declared read/write footprints and mutable authorities, so the wave is parallel."
		} else {
			wave.Rationale = "The candidate is placed alone because an explicit dependency or serialization boundary reaches this wave."
		}
		for _, id := range ready {
			delete(eligible, id)
			for after := range dependencies.edges[id] {
				indegree[after]--
			}
		}
		waves = append(waves, wave)
	}
	if len(waves) > 0 {
		last := &waves[len(waves)-1]
		if len(last.CandidateIDs) == 1 {
			last.Final = true
		}
	}
	return waves
}

func graphFromEdges(edges []DependencyEdge, ids []string) graph {
	result := graph{edges: make(map[string]map[string]bool, len(ids))}
	for _, id := range ids {
		result.edges[id] = map[string]bool{}
	}
	for _, edge := range edges {
		if _, ok := result.edges[edge.Before]; !ok {
			continue
		}
		if _, ok := result.edges[edge.After]; !ok || edge.Before == edge.After {
			if edge.Before == edge.After {
				result.edges[edge.Before][edge.After] = true
			}
			continue
		}
		result.edges[edge.Before][edge.After] = true
	}
	return result
}

func edgesToList(value graph) []DependencyEdge {
	result := make([]DependencyEdge, 0)
	for before, afters := range value.edges {
		for after := range afters {
			result = append(result, DependencyEdge{Before: before, After: after})
		}
	}
	sort.Slice(result, func(left, right int) bool { return edgeKey(result[left]) < edgeKey(result[right]) })
	return result
}

func uniqueEdges(edges []DependencyEdge) []DependencyEdge {
	seen := map[string]bool{}
	result := make([]DependencyEdge, 0, len(edges))
	for _, edge := range edges {
		key := edgeKey(edge)
		if !seen[key] {
			seen[key] = true
			result = append(result, edge)
		}
	}
	sort.Slice(result, func(left, right int) bool { return edgeKey(result[left]) < edgeKey(result[right]) })
	return result
}

func findCycleNodes(value graph, ids []string) []string {
	const (
		unseen = 0
		active = 1
		done   = 2
	)
	state := make(map[string]int, len(ids))
	stack := make([]string, 0, len(ids))
	cycle := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		state[id] = active
		stack = append(stack, id)
		afters := make([]string, 0, len(value.edges[id]))
		for after := range value.edges[id] {
			afters = append(afters, after)
		}
		sort.Strings(afters)
		for _, after := range afters {
			if state[after] == unseen {
				visit(after)
			} else if state[after] == active {
				for index := len(stack) - 1; index >= 0; index-- {
					cycle[stack[index]] = true
					if stack[index] == after {
						break
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
	}
	for _, id := range ids {
		if state[id] == unseen {
			visit(id)
		}
	}
	return stableIDs(mapKeys(cycle))
}

func conflictEvidence(first, second CandidateInput, target TargetIdentity) ([]string, []string) {
	kinds := make([]string, 0, 3)
	cells := make([]string, 0)
	writeWrite := intersection(first.SemanticWriteSet, second.SemanticWriteSet)
	writeRead := append(intersection(first.SemanticWriteSet, second.SemanticReadSet), intersection(second.SemanticWriteSet, first.SemanticReadSet)...)
	if len(writeWrite) > 0 {
		kinds = append(kinds, "WRITE_WRITE")
		cells = append(cells, writeWrite...)
	}
	if len(writeRead) > 0 {
		kinds = append(kinds, "WRITE_READ")
		cells = append(cells, writeRead...)
	}
	if first.AuthorityScope != nil && second.AuthorityScope != nil && first.AuthorityScope.MutableAuthority != "" && first.AuthorityScope.MutableAuthority == second.AuthorityScope.MutableAuthority {
		kinds = append(kinds, "MUTABLE_AUTHORITY")
		cells = append(cells, "authority:"+first.AuthorityScope.MutableAuthority)
	}
	return stableStrings(kinds), stableStrings(cells)
}

func ledgerWriterIDs(candidates []CandidateInput, target TargetIdentity) []string {
	if target.Ledger == "" || target.Ledger == "none" {
		return nil
	}
	result := make([]string, 0)
	for _, candidate := range candidates {
		if candidate.AuthorityScope != nil && candidate.AuthorityScope.WritesLedger {
			result = append(result, candidate.OperationIdentity.CandidateID)
		}
	}
	sort.Strings(result)
	return result
}

func preferUnknown(current candidateStatus, unknown Unknown) candidateStatus {
	if current.state == StateRefuted {
		return current
	}
	if current.state != StateUnknown {
		current.state = StateUnknown
		current.reason = unknown.Reason
		current.unknown = &unknown
		return current
	}
	if current.unknown == nil || unknownKey(unknown) < unknownKey(*current.unknown) {
		current.reason, current.unknown = unknown.Reason, &unknown
	}
	return current
}

func preferRefuted(current candidateStatus, reason string, caused []string) candidateStatus {
	if current.state != StateRefuted {
		current.state, current.reason, current.unknown = StateRefuted, reason, nil
	}
	current.caused = stableIDs(append(current.caused, caused...))
	return current
}

func summarizeState(statuses map[string]candidateStatus, ids []string) (string, string) {
	for _, id := range ids {
		if statuses[id].state == StateRefuted {
			return StateRefuted, "REFUTED_EVIDENCE_PRESENT"
		}
	}
	for _, id := range ids {
		if statuses[id].state == StateUnknown {
			return StateUnknown, "UNKNOWN_EVIDENCE_PRESENT"
		}
	}
	return StateClosed, "FIXED_POINT"
}

func validRelease(release ReleaseIdentity) bool {
	return release.Repository != "" && release.Tag != "" && validDigest(release.Digest)
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func intersection(left, right []string) []string {
	result := make([]string, 0)
	for _, value := range left {
		if contains(right, value) {
			result = append(result, value)
		}
	}
	return stableStrings(result)
}

func stableStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func stableIDs(values []string) []string { return stableStrings(values) }

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func mapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func pairKey(first, second string) string {
	if first > second {
		first, second = second, first
	}
	return first + "\x00" + second
}

func edgeKey(edge DependencyEdge) string { return edge.Before + "\x00" + edge.After }

func boundaryKey(boundary SerializedBoundary) string {
	return boundary.Before + "\x00" + boundary.After + "\x00" + strings.Join(boundary.Kinds, "+") + "\x00" + strings.Join(boundary.Cells, "+")
}

func unknownKey(unknown Unknown) string {
	return unknown.Stage + "\x00" + unknown.Step + "\x00" + unknown.Reason + "\x00" + unknown.UnknownClass + "\x00" + unknown.NextOperation + "\x00" + strings.Join(unknown.BlockedBy, ",")
}

func boundaryRationale(kinds, cells []string) string {
	return "Serialization is required by " + strings.Join(kinds, ", ") + " on " + strings.Join(cells, ", ") + "."
}

func waveOf(waves []Wave, candidateID string) int {
	for _, wave := range waves {
		if contains(wave.CandidateIDs, candidateID) {
			return wave.Ordinal
		}
	}
	return 0
}
