package planner

import (
	"encoding/json"
	"testing"
)

var testRelease = ReleaseIdentity{Repository: "github.com/kimjooyoon/gooo-evolution-transaction-coordinator", Tag: "v0.2.2", Digest: "sha256:2be5728925e1b85950d88f820814476b995f699abd331e0b93ce5c9c2cff86c1"}

func testMeta() MetaSource {
	return MetaSource{RepositoryIdentity: testRelease.Repository, ImmutableInputRelease: testRelease, ForbiddenAuthorities: []string{"REPOSITORY_WRITE", "CI_MUTATION", "COMMIT", "MERGE", "RELEASE_MUTATION", "EXECUTE"}}
}

func testCandidate(id string, reads, writes []string) CandidateInput {
	return CandidateInput{
		OperationIdentity:     OperationIdentity{CandidateID: id, OperationDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000001", Kind: "rewrite", Origin: "test"},
		ImmutableInputRelease: &testRelease, SemanticReadSet: reads, SemanticWriteSet: writes,
		AuthorityScope: &AuthorityScope{Permissions: []string{"READ_INPUT", "WRITE_CALLER_OUTPUT"}},
		Target:                &TargetIdentity{Repository: testRelease.Repository, Ledger: "none"},
	}
}

func testInput(candidates ...CandidateInput) Input {
	return Input{Schema: InputSchema, Version: "v1", InputRelease: &testRelease, Target: TargetIdentity{Repository: testRelease.Repository, Ledger: "none"}, DependencyEdges: []DependencyEdge{}, Candidates: candidates}
}

func TestDisjointReadOnlyCandidatesShareOneWave(t *testing.T) {
	plan, err := BuildPlan(testInput(testCandidate("alpha", []string{"read/alpha"}, []string{}), testCandidate("beta", []string{"read/beta"}, []string{})), testMeta(), "sha256:input")
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != StateClosed || len(plan.Waves) != 1 || !plan.Waves[0].Parallel {
		t.Fatalf("got state=%s waves=%#v", plan.State, plan.Waves)
	}
}

func TestWriteReadOverlapIsSerialized(t *testing.T) {
	plan, err := BuildPlan(testInput(testCandidate("writer", []string{"read/writer"}, []string{"semantic/shared"}), testCandidate("reader", []string{"semantic/shared"}, []string{})), testMeta(), "sha256:input")
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != StateClosed || len(plan.Waves) != 2 || len(plan.SerializedBoundaries) != 1 || len(plan.SerializedBoundaries[0].Kinds) != 1 || plan.SerializedBoundaries[0].Kinds[0] != "WRITE_READ" {
		t.Fatalf("got state=%s waves=%#v boundaries=%#v", plan.State, plan.Waves, plan.SerializedBoundaries)
	}
}

func TestMissingFootprintProducesExactlySixUnknownFields(t *testing.T) {
	missing := testCandidate("missing", []string{"read/missing"}, nil)
	plan, err := BuildPlan(testInput(missing, testCandidate("known", []string{"read/known"}, []string{})), testMeta(), "sha256:input")
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != StateUnknown || len(plan.BlockedCausalFrontier) != 1 {
		t.Fatalf("got state=%s blocked=%#v", plan.State, plan.BlockedCausalFrontier)
	}
	data, err := json.Marshal(plan.BlockedCausalFrontier[0].Unknown)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 6 {
		t.Fatalf("unknown has %d fields: %s", len(fields), data)
	}
}

func TestForcedConcurrentWriteIsRefuted(t *testing.T) {
	first := testCandidate("first", []string{"read/first"}, []string{"semantic/shared"})
	second := testCandidate("second", []string{"read/second"}, []string{"semantic/shared"})
	input := testInput(first, second)
	input.ForcedConcurrentPairs = []CandidatePair{{First: "first", Second: "second"}}
	plan, err := BuildPlan(input, testMeta(), "sha256:input")
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != StateRefuted || len(plan.DeniedOperations) != 2 {
		t.Fatalf("got state=%s denied=%#v", plan.State, plan.DeniedOperations)
	}
}
