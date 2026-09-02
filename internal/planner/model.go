package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	MetaSchema       = "gooo/semantic-work-wave-planner/source/v1"
	ContractSchema   = "gooo/semantic-work-wave-planner/denominator/v1"
	InputSchema      = "gooo/semantic-work-wave-planner/input/v1"
	PlanSchema       = "gooo/semantic-work-wave-planner/plan/v1"
	EvidenceSchema   = "gooo/semantic-work-wave-planner/evidence/v1"
	StateClosed      = "CLOSED"
	StateUnknown     = "UNKNOWN"
	StateRefuted     = "REFUTED"
	FixedCaseCount   = 10
	MinimumCandidate = 2
)

type MetaSource struct {
	Schema                   string
	Version                  string
	Namespace                string
	SemanticAuthorityID      string
	RepositoryIdentity       string
	ReadSet                  []string
	WriteSet                 []string
	ImmutableInputRelease    ReleaseIdentity
	ExpectedOutputRelease    ReleaseIdentity
	Effects                  []string
	Capabilities             []CapabilityDecl
	Precedence               []string
	UnknownFields            []string
	ForbiddenAuthorities     []string
	ConflictOntology         []string
	FixedPoint               string
	ProofChoices             []string
	IndicatorClasses         []string
	Prohibitions             []string
	RuntimeAuthority         AuthorityReceipt
	Denominator              DenominatorDecl
	Activities               []ActivityDecl
	Cases                    []MetaCase
	SourceDigest             string
}

type CapabilityDecl struct {
	Name    string
	Effects []string
}

type DenominatorDecl struct {
	ID        string
	CellCount int
}

type ActivityDecl struct {
	ID     string
	Stage  string
	Output string
}

type MetaCase struct {
	Ordinal        int
	ID             string
	Expected       string
	Corpus         string
	ProofChoice    string
	IndicatorClass string
}

type Contract struct {
	Schema         string         `json:"schema"`
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	CellCount      int            `json:"cell_count"`
	Fixed          bool           `json:"fixed"`
	AppendOnlyFrom string         `json:"append_only_from"`
	Cases          []ContractCase `json:"cases"`
}

type ContractCase struct {
	Ordinal        int            `json:"ordinal"`
	ID             string         `json:"id"`
	Expected       string         `json:"expected"`
	Corpus         string         `json:"corpus"`
	ProofChoice    string         `json:"proof_choice"`
	IndicatorClass string         `json:"indicator_class"`
}

type Input struct {
	Schema                 string             `json:"schema"`
	Version                string             `json:"version"`
	InputRelease           *ReleaseIdentity   `json:"input_release"`
	ImmutableInputRelease  *ReleaseIdentity   `json:"immutable_input_release"`
	Target                 TargetIdentity     `json:"target"`
	TargetRepository       string             `json:"target_repository,omitempty"`
	TargetLedger           string             `json:"target_ledger,omitempty"`
	DependencyEdges        []DependencyEdge  `json:"dependency_edges"`
	ForcedConcurrentPairs  []CandidatePair   `json:"forced_concurrent_pairs"`
	Candidates             []CandidateInput   `json:"candidates"`
}

type OperationIdentity struct {
	CandidateID     string `json:"candidate_id"`
	OperationDigest string `json:"operation_digest"`
	Kind            string `json:"kind"`
	Origin          string `json:"origin"`
}

type ReleaseIdentity struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
}

type TargetIdentity struct {
	Repository string `json:"repository"`
	Ledger     string `json:"ledger"`
}

type AuthorityScope struct {
	Permissions               []string `json:"permissions"`
	MutableAuthority          string   `json:"mutable_authority"`
	LedgerWriter              string   `json:"ledger_writer"`
	WritesLedger              bool     `json:"writes_ledger"`
	RuntimeMutationAuthority  int      `json:"runtime_mutation_authority"`
}

type CandidateInput struct {
	OperationIdentity    OperationIdentity  `json:"operation_identity"`
	CandidateID          string             `json:"candidate_id,omitempty"`
	OperationDigest      string             `json:"operation_digest,omitempty"`
	OperationKind        string             `json:"operation_kind,omitempty"`
	ImmutableInputRelease *ReleaseIdentity  `json:"immutable_input_release"`
	SemanticReadSet      []string           `json:"semantic_read_set"`
	SemanticWriteSet     []string           `json:"semantic_write_set"`
	AuthorityScope       *AuthorityScope    `json:"authority_scope"`
	Target               *TargetIdentity    `json:"target"`
	Dependencies         []string           `json:"dependencies"`
	ForcedConcurrent     bool               `json:"forced_concurrent"`
}

type DependencyEdge struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type CandidatePair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}

type CandidateDecision struct {
	CandidateID string `json:"candidate_id"`
	State       string `json:"state"`
	Reason      string `json:"reason"`
}

type Wave struct {
	Ordinal      int      `json:"ordinal"`
	CandidateIDs []string `json:"candidate_ids"`
	Parallel     bool     `json:"parallel"`
	Final        bool     `json:"final"`
	Rationale    string   `json:"rationale"`
}

type SerializedBoundary struct {
	Before   string   `json:"before"`
	After    string   `json:"after"`
	Kinds    []string `json:"kinds"`
	Cells    []string `json:"cells"`
	Rationale string  `json:"rationale"`
}

type BlockedCausalFrontier struct {
	CandidateID string  `json:"candidate_id"`
	Unknown     Unknown `json:"unknown"`
}

type SingleWriterCell struct {
	Cell        string `json:"cell"`
	Writer      string `json:"writer"`
	CandidateID string `json:"candidate_id"`
	Wave        int    `json:"wave"`
}

type DeniedOperation struct {
	CandidateID string   `json:"candidate_id"`
	Reason      string   `json:"reason"`
	CausedBy    []string `json:"caused_by"`
}

// Unknown intentionally has exactly six fields. It is the only shape used for
// unresolved evidence in both the machine plan and the human dossier.
type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type SemanticVector struct {
	State                  string                 `json:"state"`
	Decision               string                 `json:"decision"`
	CandidateDecisions     []CandidateDecision    `json:"candidate_decisions"`
	Waves                  []Wave                 `json:"waves"`
	SerializedBoundaries   []SerializedBoundary   `json:"serialized_boundaries"`
	BlockedCausalFrontier  []BlockedCausalFrontier `json:"blocked_causal_frontier"`
	SingleWriterCells      []SingleWriterCell     `json:"single_writer_cells"`
	DeniedOperations       []DeniedOperation      `json:"denied_operations"`
}

type Plan struct {
	Schema                 string                `json:"schema"`
	Version                string                `json:"version"`
	InputDigest            string                `json:"input_digest"`
	Target                 TargetIdentity        `json:"target"`
	State                  string                `json:"state"`
	Decision               string                `json:"decision"`
	CandidateDecisions     []CandidateDecision   `json:"candidate_decisions"`
	Waves                  []Wave                `json:"waves"`
	SerializedBoundaries   []SerializedBoundary  `json:"serialized_boundaries"`
	BlockedCausalFrontier  []BlockedCausalFrontier `json:"blocked_causal_frontier"`
	SingleWriterCells      []SingleWriterCell    `json:"single_writer_cells"`
	DeniedOperations       []DeniedOperation     `json:"denied_operations"`
	SemanticVectorDigest   string                `json:"semantic_vector_digest"`
	RationaleDossierDigest string                `json:"rationale_dossier_digest"`
}

type AuthorityReceipt struct {
	RepositoryWrites     int `json:"repository_writes"`
	SourceMutations      int `json:"source_mutations"`
	CommitAuthority      int `json:"commit_authority"`
	MergeAuthority       int `json:"merge_authority"`
	ReleaseMutation      int `json:"release_mutation"`
	ExecutionAuthority   int `json:"execution_authority"`
	LocalTestExecutions  int `json:"local_test_executions"`
	LocalFormatExecution int `json:"local_format_executions"`
}

type ImprovementEvidence struct {
	State  string `json:"state"`
	Value  any    `json:"value"`
	Reason string `json:"reason"`
}

type Inventory struct {
	Files              int  `json:"files"`
	Directories        int  `json:"directories"`
	GoFiles            int  `json:"go_files"`
	GoooFiles          int  `json:"gooo_files"`
	PhysicalLines      int  `json:"physical_lines"`
	RootReadmeExcluded bool `json:"root_readme_excluded"`
}

type Metrics struct {
	WallMS       int       `json:"wall_ms"`
	PeakRSSKiB   int       `json:"peak_rss_kib"`
	Inventory    Inventory `json:"inventory"`
	Improvement  ImprovementEvidence `json:"improvement"`
	CI           CIMetrics `json:"ci"`
}

type CIMetrics struct {
	State     string `json:"state"`
	Source    string `json:"source"`
	RunID     *int64 `json:"run_id"`
	CommitSHA *string `json:"commit_sha"`
	WallMS    *int   `json:"wall_ms"`
	BuildMS   *int   `json:"build_ms"`
	TestMS    *int   `json:"test_ms"`
	PeakRSSKiB *int  `json:"peak_rss_kib"`
	Cache     *bool  `json:"cache_hit"`
}

type Evidence struct {
	Schema                string             `json:"schema"`
	Version               string             `json:"version"`
	MetaSourceDigest      string             `json:"meta_source_digest"`
	ContractDigest        string             `json:"contract_digest"`
	InputDigest           string             `json:"input_digest"`
	Plan                  Plan               `json:"plan"`
	SemanticVector        SemanticVector     `json:"semantic_vector"`
	ReplayEqual           bool               `json:"replay_equal"`
	ImprovementState      string             `json:"improvement_state"`
	Improvement           *ImprovementEvidence `json:"improvement"`
	Authority             AuthorityReceipt  `json:"authority"`
	Metrics               Metrics            `json:"metrics"`
	RationaleDossier      string             `json:"rationale_dossier"`
}

type ConformanceSummary struct {
	Schema       string `json:"schema"`
	FixedCases   int    `json:"fixed_cases"`
	Generated    int    `json:"generated"`
	Closed       int    `json:"closed"`
	Unknown      int    `json:"unknown"`
	Refuted      int    `json:"refuted"`
	ReplayEqual  bool   `json:"replay_equal"`
	Authority    AuthorityReceipt `json:"authority"`
	Metrics      Metrics          `json:"metrics"`
}

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(data), nil
}
