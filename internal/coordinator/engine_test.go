package coordinator

import "testing"

func TestAllPermutationsPreserveEveryOrder(t *testing.T) {
	permutations, err := allPermutations([]string{"alpha", "beta", "gamma"})
	if err != nil {
		t.Fatal(err)
	}
	if len(permutations) != 6 {
		t.Fatalf("got %d permutations, want 6", len(permutations))
	}
}

func TestFootprintConflictDistinguishesEquivalentSet(t *testing.T) {
	equivalent := []Candidate{
		{ID: "a", Operation: Operation{Kind: "set", Value: "same"}, WriteFootprint: []string{"compiler/generated/shared.go"}},
		{ID: "b", Operation: Operation{Kind: "set", Value: "same"}, WriteFootprint: []string{"compiler/generated/shared.go"}},
	}
	if got := footprintConflict(equivalent); got != "" {
		t.Fatalf("equivalent writes conflicted: %s", got)
	}
	conflicting := equivalent
	conflicting[1].Operation.Value = "different"
	if got := footprintConflict(conflicting); got == "" {
		t.Fatal("non-equivalent writes did not conflict")
	}
}

func TestPreflightResolutionUsesRefutedBeforeUnknown(t *testing.T) {
	meta := MetaSource{ForbiddenCombinedEffects: []string{"REPOSITORY_WRITE"}}
	state, reason, unknown := preflight(meta, []Candidate{
		{ID: "missing", EffectPost: []string{"REPOSITORY_WRITE"}},
		{ID: "known", ReadFootprint: []string{"compiler/main.go"}, WriteFootprint: []string{"compiler/generated/known.go"}},
	})
	if state != StateRefuted || unknown != nil || reason != "FORBIDDEN_COMBINED_EFFECT:REPOSITORY_WRITE" {
		t.Fatalf("got state=%s reason=%s unknown=%v", state, reason, unknown)
	}
}

func TestMissingFootprintCarriesSixFieldUnknown(t *testing.T) {
	state, reason, unknown := preflight(MetaSource{}, []Candidate{{ID: "missing"}, {ID: "known", ReadFootprint: []string{"compiler/main.go"}}})
	if state != StateUnknown || reason != "MISSING_SEMANTIC_FOOTPRINT" || unknown == nil {
		t.Fatalf("got state=%s reason=%s unknown=%v", state, reason, unknown)
	}
	if unknown.Stage == "" || unknown.Step == "" || unknown.Reason == "" || unknown.UnknownClass == "" || unknown.NextOperation == "" || len(unknown.BlockedBy) == 0 {
		t.Fatalf("incomplete unknown tuple: %#v", unknown)
	}
}
