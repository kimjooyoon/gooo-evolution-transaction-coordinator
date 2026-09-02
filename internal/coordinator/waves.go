package coordinator

import (
	"fmt"
	"sort"
)

func planWaves(candidates []Candidate, adoptionTarget string) ([]EvolutionWave, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("EMPTY_WAVE_PLAN")
	}
	byID := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" || byID[candidate.ID].ID != "" {
			return nil, fmt.Errorf("DOUBLE_CANDIDATE_ID")
		}
		byID[candidate.ID] = candidate
	}
	edges := make(map[string]map[string]bool, len(candidates))
	indegree := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		edges[candidate.ID] = map[string]bool{}
		indegree[candidate.ID] = 0
	}
	addEdge := func(before, after string) {
		if before == after || edges[before][after] {
			return
		}
		edges[before][after] = true
		indegree[after]++
	}
	for _, candidate := range candidates {
		for _, dependency := range candidate.DependsOn {
			if _, ok := byID[dependency]; !ok {
				return nil, fmt.Errorf("MISSING_DEPENDENCY:%s:%s", candidate.ID, dependency)
			}
			addEdge(dependency, candidate.ID)
		}
	}
	for left := 0; left < len(candidates); left++ {
		for right := left + 1; right < len(candidates); right++ {
			first, second := candidates[left], candidates[right]
			if !mustSerialize(first, second) {
				continue
			}
			if first.ID < second.ID {
				addEdge(first.ID, second.ID)
			} else {
				addEdge(second.ID, first.ID)
			}
		}
	}
	adopters := make([]string, 0)
	for _, candidate := range candidates {
		if candidate.AdoptionTarget == "" || candidate.AdoptionTarget == "none" {
			continue
		}
		if adoptionTarget != "" && candidate.AdoptionTarget != adoptionTarget {
			return nil, fmt.Errorf("INVALID_ADOPTION_TARGET:%s", candidate.ID)
		}
		adopters = append(adopters, candidate.ID)
	}
	if len(adopters) > 1 {
		return nil, fmt.Errorf("DOUBLE_WRITER:%s", joinIDs(adopters))
	}
	if len(adopters) == 1 {
		adopter := adopters[0]
		if len(byID[adopter].DependsOn) == 0 && len(candidates) > 1 {
			return nil, fmt.Errorf("EARLY_LEDGER_ADOPTION:%s", adopter)
		}
		for _, candidate := range candidates {
			if candidate.ID != adopter {
				addEdge(candidate.ID, adopter)
			}
		}
	}

	waves := make([]EvolutionWave, 0, len(candidates))
	remaining := make(map[string]bool, len(candidates))
	for _, candidate := range candidates { remaining[candidate.ID] = true }
	for len(remaining) > 0 {
		ready := make([]string, 0)
		for id := range remaining {
			if indegree[id] == 0 { ready = append(ready, id) }
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("CYCLIC_DEPENDENCY")
		}
		sort.Strings(ready)
		wave := EvolutionWave{Ordinal: len(waves) + 1, CandidateIDs: ready, Parallel: len(ready) > 1}
		if len(adopters) == 1 && ready[0] == adopters[0] {
			wave.Final = true
			wave.SingleWriter = adopters[0]
		}
		for _, id := range ready {
			delete(remaining, id)
			for after := range edges[id] { indegree[after]-- }
		}
		waves = append(waves, wave)
	}
	if len(adopters) == 1 && (len(waves) == 0 || !waves[len(waves)-1].Final || len(waves[len(waves)-1].CandidateIDs) != 1) {
		return nil, fmt.Errorf("EARLY_LEDGER_ADOPTION")
	}
	return waves, nil
}

func mustSerialize(first, second Candidate) bool {
	firstWrites, secondWrites := first.WriteSet, second.WriteSet
	if len(firstWrites) == 0 { firstWrites = first.WriteFootprint }
	if len(secondWrites) == 0 { secondWrites = second.WriteFootprint }
	return overlap(firstWrites, secondWrites) ||
		(first.SemanticAuthorityID != "" && first.SemanticAuthorityID == second.SemanticAuthorityID) ||
		(first.RepositoryWriter != "" && first.RepositoryWriter == second.RepositoryWriter)
}

func joinIDs(ids []string) string {
	copyIDs := append([]string{}, ids...)
	sort.Strings(copyIDs)
	result := ""
	for index, id := range copyIDs {
		if index > 0 { result += "," }
		result += id
	}
	return result
}

func lanePreflight(meta MetaSource, candidate Candidate) (string, *Unknown) {
	for _, forbidden := range meta.ForbiddenCombinedEffects {
		if contains(append(append([]string{}, candidate.EffectPost...), candidate.Capabilities...), forbidden) {
			return StateRefuted, nil
		}
	}
	missing := make([]string, 0)
	if candidate.SemanticAuthorityID == "" { missing = append(missing, "semantic_authority_id") }
	if candidate.RepositoryIdentity == "" { missing = append(missing, "repository_identity") }
	if candidate.RepositoryWriter == "" { missing = append(missing, "repository_writer") }
	if len(candidate.ReadSet) == 0 || len(candidate.WriteSet) == 0 { missing = append(missing, "read_set/write_set") }
	if !validRelease(candidate.ImmutableInputRelease) || !validRelease(candidate.ExpectedOutputRelease) { missing = append(missing, "release_identity") }
	if candidate.AdoptionTarget == "" { missing = append(missing, "adoption_target") }
	if releasePresent(candidate.ImmutableInputRelease) && !validRelease(candidate.ImmutableInputRelease) {
		return StateRefuted, nil
	}
	if releasePresent(candidate.ExpectedOutputRelease) && !validRelease(candidate.ExpectedOutputRelease) {
		return StateRefuted, nil
	}
	if candidate.ImmutableInputRelease != meta.ImmutableInputRelease && validRelease(candidate.ImmutableInputRelease) {
		return StateRefuted, nil
	}
	if candidate.ExpectedOutputRelease != meta.ExpectedOutputRelease && validRelease(candidate.ExpectedOutputRelease) {
		return StateRefuted, nil
	}
	if len(missing) > 0 {
		return StateUnknown, &Unknown{Stage: "IDENTITY", Step: "resolve_candidate_evidence", Reason: "IDENTITY_OR_EVIDENCE_MISSING", UnknownClass: "MISSING_IDENTITY_OR_EVIDENCE", NextOperation: "DECLARE_MISSING_IDENTITY_OR_EVIDENCE", BlockedBy: []string{candidate.ID}}
	}
	return StateClosed, nil
}

func releasePresent(release ReleaseIdentity) bool {
	return release.Repository != "" || release.Tag != "" || release.Digest != ""
}

func preflightLanes(meta MetaSource, candidates []Candidate) []LaneResult {
	lanes := make([]LaneResult, 0, len(candidates))
	states := make(map[string]string, len(candidates))
	unknowns := make(map[string]*Unknown, len(candidates))
	for _, candidate := range candidates {
		state, unknown := lanePreflight(meta, candidate)
		states[candidate.ID], unknowns[candidate.ID] = state, unknown
	}
	for _, candidate := range candidates {
		if states[candidate.ID] == StateClosed {
			for _, dependency := range candidate.DependsOn {
				if states[dependency] == StateUnknown || states[dependency] == StateRefuted {
					states[candidate.ID] = states[dependency]
					unknowns[candidate.ID] = &Unknown{Stage: "DEPENDENCY", Step: "advance_causal_frontier", Reason: "CAUSAL_DEPENDENCY_BLOCKED", UnknownClass: "CAUSAL_DEPENDENCY_FRONTIER", NextOperation: "RESOLVE_BLOCKED_DEPENDENCY", BlockedBy: []string{dependency}}
				}
			}
		}
	}
	for _, candidate := range candidates {
		lanes = append(lanes, LaneResult{CandidateID: candidate.ID, State: states[candidate.ID], Decision: laneDecision(states[candidate.ID]), Unknown: unknowns[candidate.ID]})
	}
	return lanes
}

func laneDecision(state string) string {
	switch state {
	case StateClosed: return "LANE_CLOSED"
	case StateUnknown: return "LANE_UNKNOWN"
	default: return "LANE_REFUTED"
	}
}

func laneSummaryState(lanes []LaneResult) (string, string, *Unknown) {
	for _, lane := range lanes { if lane.State == StateRefuted { return StateRefuted, "LANE_REFUTED:" + lane.CandidateID, nil } }
	for _, lane := range lanes { if lane.State == StateUnknown { return StateUnknown, "LANE_UNKNOWN:" + lane.CandidateID, lane.Unknown } }
	return StateClosed, "", nil
}

func observedImprovement(candidates []Candidate) *ImprovementEvidence {
	if len(candidates) == 0 || candidates[0].WorkReceipt == nil {
		return nil
	}
	reference := *candidates[0].WorkReceipt
	for _, candidate := range candidates[1:] {
		if candidate.WorkReceipt == nil || candidate.WorkReceipt.BeforeRelease != reference.BeforeRelease || candidate.WorkReceipt.AfterRelease != reference.AfterRelease {
			return nil
		}
	}
	return &ImprovementEvidence{
		BeforeRelease: reference.BeforeRelease, AfterRelease: reference.AfterRelease,
		SequentialWaveCount: reference.SequentialWaves, ParallelWaveCount: reference.ParallelWaves,
		CriticalPath: reference.CriticalPath, CIWallMS: reference.CIWallMS, CIBuildMS: reference.CIBuildMS, CITestMS: reference.CITestMS,
	}
}

func plannedOrders(waves []EvolutionWave) [][]string {
	order := make([]string, 0)
	for _, wave := range waves { order = append(order, wave.CandidateIDs...) }
	return [][]string{order}
}
