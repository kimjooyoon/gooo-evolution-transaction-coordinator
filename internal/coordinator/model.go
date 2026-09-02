package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	MetaSchema       = "gooo/evolution-transaction-coordinator/source/v2"
	ContractSchema   = "gooo/evolution-transaction-coordinator/denominator/v2"
	EvidenceSchema   = "gooo/evolution-transaction-coordinator/evidence/v2"
	CandidateSchema  = "gooo/evolution-transaction-coordinator/candidate/v1"
	StateClosed      = "CLOSED"
	StateUnknown     = "UNKNOWN"
	StateRefuted     = "REFUTED"
	FixedCaseCount   = 8
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
	Incidents                []IncidentDecl
	AtomicAbort              AtomicAbortDecl
	Bundle                   BundleDecl
	SemanticAuthorityID      string
	RepositoryIdentity       string
	ReadSet                  []string
	WriteSet                 []string
	ImmutableInputRelease    ReleaseIdentity
	ExpectedOutputRelease    ReleaseIdentity
	AdoptionTarget           string
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
	Ordinal        int
	ID             string
	Kind           string
	Rule           string
	Expected       string
	CandidateIDs   []string
	Replay         bool
	IndicatorClass string
	Proof          ProofSelection
}

type IncidentDecl struct {
	ID                  string
	Kind                string
	State               string
	ReleaseRepository   string
	ReleaseTag          string
	ReleaseID           string
	ReleaseImmutable    string
	ReleaseTagObjectSHA string
	ReleaseCommitSHA    string
	ReleaseURL          string
	PRNumber            string
	PRURL               string
	PRHeadSHA           string
	PRRunID             string
	PRRunURL            string
	MergeCommitSHA      string
	MergeURL            string
	RunID               string
	RunWorkflow         string
	RunEvent            string
	RunConclusion       string
	RunSHA              string
	RunURL              string
	MainRunID           string
	MainRunURL          string
	ReceiptAssetID      string
	ReceiptName         string
	ReceiptDigest       string
	ReceiptURL          string
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
	Kind           string         `json:"kind"`
	Expected       string         `json:"expected"`
	IndicatorClass string         `json:"indicator_class"`
	Proof          ProofSelection `json:"proof"`
}

type Candidate struct {
	Schema                string            `json:"schema"`
	Version               string            `json:"version"`
	ID                    string            `json:"id"`
	Type                  string            `json:"type"`
	Origin                Origin            `json:"origin"`
	Capabilities          []string          `json:"capabilities"`
	EffectPre             []string          `json:"effect_pre"`
	EffectPost            []string          `json:"effect_post"`
	ReadFootprint         []string          `json:"read_footprint"`
	WriteFootprint        []string          `json:"write_footprint"`
	Preconditions         map[string]string `json:"preconditions"`
	Postconditions        map[string]string `json:"postconditions"`
	Operation             Operation         `json:"operation"`
	SemanticAuthorityID   string            `json:"semantic_authority_id"`
	RepositoryIdentity    string            `json:"repository_identity"`
	RepositoryWriter      string            `json:"repository_writer"`
	ReadSet               []string          `json:"read_set"`
	WriteSet              []string          `json:"write_set"`
	ImmutableInputRelease ReleaseIdentity   `json:"immutable_input_release"`
	ExpectedOutputRelease ReleaseIdentity   `json:"expected_output_release"`
	AdoptionTarget        string            `json:"adoption_target"`
	DependsOn             []string          `json:"depends_on"`
	WorkReceipt           *WorkReceipt      `json:"work_receipt,omitempty"`
	Proof                 ProofSelection    `json:"proof"`
	SourcePath            string            `json:"source_path"`
	SourceDigest          string            `json:"source_digest"`
}

type ReleaseIdentity struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
}

type WorkReceipt struct {
	BeforeRelease   ReleaseIdentity `json:"before_release"`
	AfterRelease    ReleaseIdentity `json:"after_release"`
	SequentialWaves int             `json:"sequential_wave_count"`
	ParallelWaves   int             `json:"parallel_wave_count"`
	CriticalPath    int             `json:"critical_path"`
	CIWallMS        int             `json:"ci_wall_ms"`
	CIBuildMS       int             `json:"ci_build_ms"`
	CITestMS        int             `json:"ci_test_ms"`
}

type ProofSelection struct {
	Choice    string `json:"choice"`
	Driver    string `json:"driver"`
	Outcome   string `json:"outcome"`
	Guardrail string `json:"guardrail"`
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
	CandidateID           string            `json:"candidate_id"`
	Read                  []string          `json:"read"`
	Write                 []string          `json:"write"`
	Origin                Origin            `json:"origin"`
	Capabilities          []string          `json:"capabilities"`
	EffectPre             []string          `json:"effect_pre"`
	EffectPost            []string          `json:"effect_post"`
	Preconditions         map[string]string `json:"preconditions"`
	Postconditions        map[string]string `json:"postconditions"`
	SemanticAuthorityID   string            `json:"semantic_authority_id"`
	RepositoryIdentity    string            `json:"repository_identity"`
	RepositoryWriter      string            `json:"repository_writer"`
	ReadSet               []string          `json:"read_set"`
	WriteSet              []string          `json:"write_set"`
	ImmutableInputRelease ReleaseIdentity   `json:"immutable_input_release"`
	ExpectedOutputRelease ReleaseIdentity   `json:"expected_output_release"`
	AdoptionTarget        string            `json:"adoption_target"`
	DependsOn             []string          `json:"depends_on"`
	Proof                 ProofSelection    `json:"proof"`
}

type EvolutionWave struct {
	Ordinal      int      `json:"ordinal"`
	CandidateIDs []string `json:"candidate_ids"`
	Parallel     bool     `json:"parallel"`
	Final        bool     `json:"final"`
	SingleWriter string   `json:"single_writer,omitempty"`
}

type LaneResult struct {
	CandidateID string   `json:"candidate_id"`
	State       string   `json:"state"`
	Decision    string   `json:"decision"`
	Unknown     *Unknown `json:"unknown,omitempty"`
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
	Waves                []EvolutionWave     `json:"waves"`
	Lanes                []LaneResult        `json:"lanes"`
	SequentialWaveCount  int                 `json:"sequential_wave_count"`
	ParallelWaveCount    int                 `json:"parallel_wave_count"`
	CriticalPath         int                 `json:"critical_path"`
	IndicatorClass       string              `json:"indicator_class"`
	Proof                ProofSelection      `json:"proof"`
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
	WallMS              int                  `json:"wall_ms"`
	PeakRSSKiB          int                  `json:"peak_rss_kib"`
	Inventory           Inventory            `json:"inventory"`
	Generated           GeneratedMetrics     `json:"generated"`
	Tests               TestMetrics          `json:"tests"`
	SequentialWaveCount int                  `json:"sequential_wave_count"`
	ParallelWaveCount   int                  `json:"parallel_wave_count"`
	CriticalPath        int                  `json:"critical_path"`
	CIWallMS            int                  `json:"ci_wall_ms"`
	CIBuildMS           int                  `json:"ci_build_ms"`
	CITestMS            int                  `json:"ci_test_ms"`
	ImprovementState    string               `json:"improvement_state"`
	Improvement         *ImprovementEvidence `json:"improvement"`
	Local               LocalMetrics         `json:"local"`
	CI                  CIMetrics            `json:"ci"`
}

type LocalMetrics struct {
	WallMS     int `json:"wall_ms"`
	PeakRSSKiB int `json:"peak_rss_kib"`
}

type CIMetrics struct {
	State     string  `json:"state"`
	Source    string  `json:"source"`
	Reason    string  `json:"reason"`
	RunID     *int64  `json:"run_id"`
	CommitSHA *string `json:"commit_sha"`
	WallMS    *int    `json:"wall_ms"`
	BuildMS   *int    `json:"build_ms"`
	TestMS    *int    `json:"test_ms"`
}

type IncidentEvidence struct {
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	State       string                       `json:"state"`
	Release     IncidentReleaseIdentity     `json:"release"`
	PullRequest IncidentPullRequestIdentity `json:"pull_request"`
	Merge       IncidentMergeIdentity       `json:"merge"`
	Run         IncidentRunIdentity         `json:"run"`
	Receipt     IncidentReceiptIdentity     `json:"receipt"`
}

type IncidentReleaseIdentity struct {
	Repository   *string `json:"repository"`
	Tag          *string `json:"tag"`
	ReleaseID    *int64  `json:"release_id"`
	Immutable    *bool   `json:"immutable"`
	TagObjectSHA *string `json:"tag_object_sha"`
	CommitSHA    *string `json:"commit_sha"`
	URL          *string `json:"url"`
}

type IncidentPullRequestIdentity struct {
	Number  *int    `json:"number"`
	URL     *string `json:"url"`
	HeadSHA *string `json:"head_sha"`
	RunID   *int64  `json:"run_id"`
	RunURL  *string `json:"run_url"`
}

type IncidentMergeIdentity struct {
	CommitSHA *string `json:"commit_sha"`
	URL       *string `json:"url"`
}

type IncidentRunIdentity struct {
	ID         *int64  `json:"id"`
	Workflow   *string `json:"workflow"`
	Event      *string `json:"event"`
	Conclusion *string `json:"conclusion"`
	HeadSHA    *string `json:"head_sha"`
	URL        *string `json:"url"`
	MainID     *int64  `json:"main_ci_id"`
	MainURL    *string `json:"main_ci_url"`
}

type IncidentReceiptIdentity struct {
	AssetID *int64  `json:"asset_id"`
	Name    *string `json:"name"`
	Digest  *string `json:"digest"`
	URL     *string `json:"url"`
}

type ImprovementEvidence struct {
	BeforeRelease       ReleaseIdentity `json:"before_release"`
	AfterRelease        ReleaseIdentity `json:"after_release"`
	SequentialWaveCount int             `json:"sequential_wave_count"`
	ParallelWaveCount   int             `json:"parallel_wave_count"`
	CriticalPath        int             `json:"critical_path"`
	CIWallMS            int             `json:"ci_wall_ms"`
	CIBuildMS           int             `json:"ci_build_ms"`
	CITestMS            int             `json:"ci_test_ms"`
}

type Authority struct {
	RepositoryWrites      int `json:"repository_writes"`
	SourceMutations       int `json:"source_mutations"`
	CommitAuthority       int `json:"commit_authority"`
	MergeAuthority        int `json:"merge_authority"`
	ReleaseMutation       int `json:"release_mutation"`
	LocalTestExecutions   int `json:"local_test_executions"`
	OperatorAuthoring     int `json:"operator_authoring"`
	CIRuntimeAuthority    int `json:"ci_runtime_authority"`
	LocalFormatExecutions int `json:"local_format_executions"`
}

type Evidence struct {
	Schema               string          `json:"schema"`
	Version              string          `json:"version"`
	SourceDigest         string          `json:"source_digest"`
	ContractDigest       string          `json:"contract_digest"`
	EvaluatorDigest      string          `json:"evaluator_digest"`
	Precedence           []string        `json:"precedence"`
	UnknownFields        []string        `json:"unknown_fields"`
	DenominatorID        string          `json:"denominator_id"`
	FixedCaseCount       int             `json:"fixed_case_count"`
	Summary              Summary         `json:"summary"`
	Candidates           []Candidate     `json:"candidates"`
	Cases                []CaseResult    `json:"cases"`
	Incidents            []IncidentEvidence `json:"incidents"`
	Metrics              Metrics         `json:"metrics"`
	Authority            Authority       `json:"authority"`
	ArtifactNames        []string        `json:"artifact_names"`
	ArtifactCount        int             `json:"artifact_count"`
	AtomicAbortRule      AtomicAbortDecl `json:"atomic_abort_rule"`
	BundleRule           BundleDecl      `json:"bundle_rule"`
	OperationalIncidents []string        `json:"operational_incidents"`
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
