package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	MetaSchema       = "gooo/evolution-transaction-coordinator/source/v1"
	ContractSchema   = "gooo/evolution-transaction-coordinator/denominator/v1"
	EvidenceSchema   = "gooo/evolution-transaction-coordinator/evidence/v1"
	CandidateSchema  = "gooo/evolution-transaction-coordinator/candidate/v1"
	StateClosed      = "CLOSED"
	StateUnknown     = "UNKNOWN"
	StateRefuted     = "REFUTED"
	FixedCaseCount   = 7
	MinimumCandidate = 2
)

type MetaSource struct {
	Schema                   string
	Version                  string
	Namespace                string
	Effects                  []string
	Capabilities             []CapabilityDecl
	Precedence               []string
	UnknownFields            []string
	ForbiddenCombinedEffects []string
	Rules                    []RuleDecl
	Denominator              DenominatorDecl
	Cases                    []CaseDecl
	AtomicAbort              AtomicAbortDecl
	Bundle                   BundleDecl
	SourceDigest             string
}

type CapabilityDecl struct {
	Name    string
	Effects []string
}

type RuleDecl struct {
	ID        string
	Condition string
	Outcome   string
	Terminal  string
}

type DenominatorDecl struct {
	ID        string
	CellCount int
}

type CaseDecl struct {
	Ordinal      int
	ID           string
	Kind         string
	Rule         string
	Expected     string
	CandidateIDs []string
	Replay       bool
}

type AtomicAbortDecl struct {
	States           []string
	PromoteBundle    bool
	PartialPromotion bool
}

type BundleDecl struct {
	ClosedOnly bool
	Artifact   string
	Digest     string
}

type Contract struct {
	Schema    string         `json:"schema"`
	ID        string         `json:"id"`
	Version   string         `json:"version"`
	CellCount int            `json:"cell_count"`
	Fixed     bool           `json:"fixed"`
	Cases     []ContractCase `json:"cases"`
}

type ContractCase struct {
	Ordinal  int    `json:"ordinal"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Expected string `json:"expected"`
}

type Candidate struct {
	Schema         string            `json:"schema"`
	Version        string            `json:"version"`
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Origin         Origin            `json:"origin"`
	Capabilities   []string          `json:"capabilities"`
	EffectPre      []string          `json:"effect_pre"`
	EffectPost     []string          `json:"effect_post"`
	ReadFootprint  []string          `json:"read_footprint"`
	WriteFootprint []string          `json:"write_footprint"`
	Preconditions  map[string]string `json:"preconditions"`
	Postconditions map[string]string `json:"postconditions"`
	Operation      Operation         `json:"operation"`
	SourcePath     string            `json:"source_path"`
	SourceDigest   string            `json:"source_digest"`
}

type Origin struct {
	Author string `json:"author"`
	Source string `json:"source"`
}

type Operation struct {
	Kind            string `json:"kind"`
	Artifact        string `json:"artifact"`
	Value           string `json:"value"`
	SuccessTerminal string `json:"success_terminal"`
	FailureTerminal string `json:"failure_terminal"`
}

type FootprintSummary struct {
	CandidateID    string            `json:"candidate_id"`
	Read           []string          `json:"read"`
	Write          []string          `json:"write"`
	Origin         Origin            `json:"origin"`
	Capabilities   []string          `json:"capabilities"`
	EffectPre      []string          `json:"effect_pre"`
	EffectPost     []string          `json:"effect_post"`
	Preconditions  map[string]string `json:"preconditions"`
	Postconditions map[string]string `json:"postconditions"`
}

type ArtifactSnapshot struct {
	Files  map[string]string `json:"files"`
	Digest string            `json:"digest"`
}

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type PermutationResult struct {
	Order              []string         `json:"order"`
	State              string           `json:"state"`
	GeneratedArtifact  ArtifactSnapshot `json:"generated_artifact"`
	TerminalReason     string           `json:"terminal_reason"`
	OrderedEffectTrace []string         `json:"ordered_effect_trace"`
	AppliedCandidates  []string         `json:"applied_candidates"`
	AtomicAbort        bool             `json:"atomic_abort"`
	PromotedBundle     bool             `json:"promoted_bundle"`
	Unknown            *Unknown         `json:"unknown,omitempty"`
}

type CaseResult struct {
	Ordinal              int                 `json:"ordinal"`
	ID                   string              `json:"id"`
	Kind                 string              `json:"kind"`
	Expected             string              `json:"expected"`
	State                string              `json:"state"`
	Decision             string              `json:"decision"`
	CandidateIDs         []string            `json:"candidate_ids"`
	CandidateSummaries   []FootprintSummary  `json:"candidate_summaries"`
	CombinedCapabilities []string            `json:"combined_capabilities"`
	CombinedEffectPre    []string            `json:"combined_effect_pre"`
	CombinedEffectPost   []string            `json:"combined_effect_post"`
	Permutations         []PermutationResult `json:"permutations"`
	ReplayEqual          bool                `json:"replay_equal"`
	AtomicAbort          bool                `json:"atomic_abort"`
	PromotedBundle       bool                `json:"promoted_bundle"`
	Bundle               *ArtifactSnapshot   `json:"bundle,omitempty"`
	Unknown              *Unknown            `json:"unknown,omitempty"`
}

type Summary struct {
	Generated int `json:"generated"`
	Closed    int `json:"closed"`
	Unknown   int `json:"unknown"`
	Refuted   int `json:"refuted"`
}

type Inventory struct {
	Files              int  `json:"files"`
	Directories        int  `json:"directories"`
	GoFiles            int  `json:"go_files"`
	GoooFiles          int  `json:"gooo_files"`
	PhysicalLines      int  `json:"physical_lines"`
	RootReadmeExcluded bool `json:"root_readme_excluded"`
}

type GeneratedMetrics struct {
	Files int `json:"files"`
	Bytes int `json:"bytes"`
}

type TestMetrics struct {
	Total    int `json:"total"`
	Selected int `json:"selected"`
	Executed int `json:"executed"`
	Reused   int `json:"reused"`
	Failed   int `json:"failed"`
	Unknown  int `json:"unknown"`
}

type Metrics struct {
	WallMS     int              `json:"wall_ms"`
	PeakRSSKiB int              `json:"peak_rss_kib"`
	Inventory  Inventory        `json:"inventory"`
	Generated  GeneratedMetrics `json:"generated"`
	Tests      TestMetrics      `json:"tests"`
}

type Authority struct {
	RepositoryWrites    int `json:"repository_writes"`
	SourceMutations     int `json:"source_mutations"`
	CommitAuthority     int `json:"commit_authority"`
	MergeAuthority      int `json:"merge_authority"`
	ReleaseMutation     int `json:"release_mutation"`
	LocalTestExecutions int `json:"local_test_executions"`
}

type Evidence struct {
	Schema          string          `json:"schema"`
	Version         string          `json:"version"`
	SourceDigest    string          `json:"source_digest"`
	ContractDigest  string          `json:"contract_digest"`
	EvaluatorDigest string          `json:"evaluator_digest"`
	Precedence      []string        `json:"precedence"`
	UnknownFields   []string        `json:"unknown_fields"`
	DenominatorID   string          `json:"denominator_id"`
	FixedCaseCount  int             `json:"fixed_case_count"`
	Summary         Summary         `json:"summary"`
	Candidates      []Candidate     `json:"candidates"`
	Cases           []CaseResult    `json:"cases"`
	Metrics         Metrics         `json:"metrics"`
	Authority       Authority       `json:"authority"`
	ArtifactNames   []string        `json:"artifact_names"`
	ArtifactCount   int             `json:"artifact_count"`
	AtomicAbortRule AtomicAbortDecl `json:"atomic_abort_rule"`
	BundleRule      BundleDecl      `json:"bundle_rule"`
}

type fixtureState struct {
	Files       map[string][]byte
	Appended    map[string][]string
	Applied     []string
	EffectTrace []string
	Terminal    string
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
