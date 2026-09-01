package coordinator

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

type RunInput struct {
	MetaPath       string
	ContractPath   string
	CandidatesRoot string
	CandidatePaths []string
	FixtureRoot    string
	OutputRoot     string
	SourceRoot     string
}

type runContext struct {
	Meta            MetaSource
	Contract        Contract
	Candidates      []Candidate
	CandidateByID   map[string]Candidate
	SourceRoot      string
	FixtureRoot     string
	OutputRoot      string
	TemplateRoot    string
	SourceDigest    string
	ContractDigest  string
	EvaluatorDigest string
	BundleNames     []string
	BundleBytes     int
}

func Run(input RunInput) (Evidence, error) {
	started := time.Now()
	meta, err := ParseMeta(input.MetaPath)
	if err != nil {
		return Evidence{}, err
	}
	contract, err := LoadContract(input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	if err := validateDeclarations(meta, contract); err != nil {
		return Evidence{}, err
	}
	sourceRoot, err := filepath.Abs(input.SourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	fixtureRoot, err := filepath.Abs(input.FixtureRoot)
	if err != nil {
		return Evidence{}, err
	}
	outputRoot, err := filepath.Abs(input.OutputRoot)
	if err != nil {
		return Evidence{}, err
	}
	candidatesRoot, err := filepath.Abs(input.CandidatesRoot)
	if err != nil {
		return Evidence{}, err
	}
	candidatePaths := make([]string, 0, len(input.CandidatePaths))
	for _, path := range input.CandidatePaths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Evidence{}, err
		}
		candidatePaths = append(candidatePaths, absolute)
	}
	if err := ensureExternalEmptyDirectory(fixtureRoot, sourceRoot); err != nil {
		return Evidence{}, fmt.Errorf("fixture root: %w", err)
	}
	if err := ensureExternalEmptyDirectory(outputRoot, sourceRoot); err != nil {
		return Evidence{}, fmt.Errorf("output root: %w", err)
	}
	candidates, err := LoadCandidates(candidatesRoot, candidatePaths)
	if err != nil {
		return Evidence{}, err
	}
	if len(candidates) < MinimumCandidate {
		return Evidence{}, fmt.Errorf("at least %d typed candidates are required", MinimumCandidate)
	}
	for index := range candidates {
		if err := validateCandidate(candidates[index], meta); err != nil {
			return Evidence{}, err
		}
		relative, err := filepath.Rel(sourceRoot, candidates[index].SourcePath)
		if err == nil && !strings.HasPrefix(relative, "..") {
			candidates[index].SourcePath = filepath.ToSlash(relative)
		}
	}
	contractData, err := os.ReadFile(input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	evaluatorDigest, err := DigestValue(map[string]string{
		"evaluator": "gooo-evolution-transaction-coordinator",
		"version":   "v1",
		"source":    meta.SourceDigest,
		"contract":  DigestBytes(contractData),
	})
	if err != nil {
		return Evidence{}, err
	}
	context := runContext{
		Meta: meta, Contract: contract, Candidates: candidates,
		CandidateByID: make(map[string]Candidate, len(candidates)), SourceRoot: sourceRoot,
		FixtureRoot: fixtureRoot, OutputRoot: outputRoot,
		TemplateRoot: filepath.Join(sourceRoot, "fixtures", "compiler"),
		SourceDigest: meta.SourceDigest, ContractDigest: DigestBytes(contractData), EvaluatorDigest: evaluatorDigest,
	}
	for _, candidate := range candidates {
		context.CandidateByID[candidate.ID] = candidate
	}
	if _, err := os.Stat(context.TemplateRoot); err != nil {
		return Evidence{}, fmt.Errorf("compiler fixture is unavailable: %w", err)
	}

	results := make([]CaseResult, 0, len(meta.Cases))
	for _, declaredCase := range meta.Cases {
		caseResult, err := evaluateCase(&context, declaredCase)
		if err != nil {
			return Evidence{}, err
		}
		results = append(results, caseResult)
	}

	inventory, err := measureInventory(sourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	summary := summarize(results)
	wallMS := int(time.Since(started).Milliseconds())
	if wallMS < 1 {
		wallMS = 1
	}
	metrics := Metrics{
		WallMS:     wallMS,
		PeakRSSKiB: peakRSSKiB(),
		Inventory:  inventory,
		Generated:  GeneratedMetrics{Files: len(context.BundleNames), Bytes: context.BundleBytes},
		Tests:      TestMetrics{Total: FixedCaseCount, Selected: len(results), Executed: len(results), Reused: replayCount(results), Failed: summary.Refuted, Unknown: summary.Unknown},
	}
	artifactNames := append([]string{}, context.BundleNames...)
	artifactNames = append(artifactNames, "coordinator-report.md", "evidence.json")
	sort.Strings(artifactNames)
	evidence := Evidence{
		Schema: EvidenceSchema, Version: "v1", SourceDigest: context.SourceDigest,
		ContractDigest: context.ContractDigest, EvaluatorDigest: context.EvaluatorDigest,
		Precedence: append([]string{}, meta.Precedence...), UnknownFields: append([]string{}, meta.UnknownFields...),
		DenominatorID: meta.Denominator.ID, FixedCaseCount: FixedCaseCount, Summary: summary,
		Candidates: candidates, Cases: results, Metrics: metrics,
		Authority:     Authority{RepositoryWrites: 0, SourceMutations: 0, CommitAuthority: 0, MergeAuthority: 0, ReleaseMutation: 0, LocalTestExecutions: 0},
		ArtifactNames: artifactNames, ArtifactCount: len(artifactNames), AtomicAbortRule: meta.AtomicAbort, BundleRule: meta.Bundle,
	}
	if err := writeEvidence(outputRoot, evidence); err != nil {
		return Evidence{}, err
	}
	if err := writeReport(outputRoot, evidence); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func evaluateCase(context *runContext, declared CaseDecl) (CaseResult, error) {
	candidates := make([]Candidate, 0, len(declared.CandidateIDs))
	for _, id := range declared.CandidateIDs {
		candidate, ok := context.CandidateByID[id]
		if !ok {
			return CaseResult{}, fmt.Errorf("case %q references missing candidate %q", declared.ID, id)
		}
		candidates = append(candidates, candidate)
	}
	rule, ok := findRule(context.Meta.Rules, declared.Rule)
	if !ok {
		return CaseResult{}, fmt.Errorf("case %q references missing rule %q", declared.ID, declared.Rule)
	}
	result := CaseResult{
		Ordinal: declared.Ordinal, ID: declared.ID, Kind: declared.Kind, Expected: declared.Expected,
		CandidateIDs: append([]string{}, declared.CandidateIDs...), CandidateSummaries: summarizeCandidates(candidates),
		CombinedCapabilities: unionCandidateValues(candidates, func(candidate Candidate) []string { return candidate.Capabilities }),
		CombinedEffectPre:    unionCandidateValues(candidates, func(candidate Candidate) []string { return candidate.EffectPre }),
		CombinedEffectPost:   unionCandidateValues(candidates, func(candidate Candidate) []string { return candidate.EffectPost }),
	}
	permutations, err := allPermutations(declared.CandidateIDs)
	if err != nil {
		return CaseResult{}, fmt.Errorf("case %q: %w", declared.ID, err)
	}
	preflightState, preflightReason, preflightUnknown := preflight(context.Meta, candidates)
	for _, order := range permutations {
		observation := PermutationResult{Order: append([]string{}, order...), State: preflightState, GeneratedArtifact: emptyArtifact(), TerminalReason: preflightReason, Unknown: preflightUnknown}
		if preflightState == StateUnknown {
			observation.AtomicAbort = true
			observation.OrderedEffectTrace = []string{"preflight:semantic_footprint_missing"}
		} else if preflightState == StateRefuted {
			observation.AtomicAbort = true
			observation.OrderedEffectTrace = []string{"preflight:" + preflightReason}
		} else {
			observation, err = executePermutation(context, declared, candidates, rule, order, "pass-1")
			if err != nil {
				return CaseResult{}, err
			}
		}
		result.Permutations = append(result.Permutations, observation)
	}

	if preflightState == StateUnknown || preflightState == StateRefuted {
		result.State = preflightState
		result.Decision = preflightReason
		result.AtomicAbort = true
		result.PromotedBundle = false
		result.Unknown = preflightUnknown
	} else {
		result.State = StateClosed
		result.Decision = rule.Terminal
		if !allVectorsEqual(result.Permutations) {
			result.State = StateRefuted
			result.Decision = rule.Terminal
		}
		if declared.Replay && result.State == StateClosed {
			result.ReplayEqual, err = replayCase(context, declared, candidates, rule, permutations, result.Permutations)
			if err != nil {
				return CaseResult{}, err
			}
			if !result.ReplayEqual {
				result.State = StateRefuted
				result.Decision = "REPLAY_VECTOR_DIVERGENCE"
			}
		}
		if result.State == StateClosed {
			result.PromotedBundle = true
			result.AtomicAbort = false
			bundle, err := emitBundle(context, declared, candidates)
			if err != nil {
				return CaseResult{}, err
			}
			result.Bundle = &bundle
		} else {
			result.AtomicAbort = true
			result.PromotedBundle = false
		}
	}
	for index := range result.Permutations {
		result.Permutations[index].AtomicAbort = result.AtomicAbort
		result.Permutations[index].PromotedBundle = result.PromotedBundle
	}
	if result.State != rule.Outcome && !(result.State == StateRefuted && rule.Outcome == StateRefuted) {
		return CaseResult{}, fmt.Errorf("case %q evaluated as %s, declared rule %q requires %s", declared.ID, result.State, rule.ID, rule.Outcome)
	}
	return result, nil
}

func preflight(meta MetaSource, candidates []Candidate) (string, string, *Unknown) {
	combinedEffects := unionCandidateValues(candidates, func(candidate Candidate) []string {
		return append(append([]string{}, candidate.EffectPost...), candidate.Capabilities...)
	})
	for _, forbidden := range meta.ForbiddenCombinedEffects {
		if contains(combinedEffects, forbidden) {
			return StateRefuted, "FORBIDDEN_COMBINED_EFFECT:" + forbidden, nil
		}
	}
	if conflict := footprintConflict(candidates); conflict != "" {
		return StateRefuted, "EXPLICIT_FOOTPRINT_CONFLICT:" + conflict, nil
	}
	missing := make([]string, 0)
	for _, candidate := range candidates {
		if len(candidate.ReadFootprint) == 0 && len(candidate.WriteFootprint) == 0 {
			missing = append(missing, candidate.ID)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return StateUnknown, "MISSING_SEMANTIC_FOOTPRINT", &Unknown{
			Stage: "FOOTPRINT", Step: "compute_semantic_footprint", Reason: "READ_WRITE_FOOTPRINT_NOT_DECLARED",
			UnknownClass: "MISSING_SEMANTIC_FOOTPRINT", NextOperation: "DECLARE_READ_WRITE_FOOTPRINT", BlockedBy: missing,
		}
	}
	return StateClosed, "", nil
}

func executePermutation(context *runContext, declared CaseDecl, candidates []Candidate, rule RuleDecl, order []string, pass string) (PermutationResult, error) {
	name := fmt.Sprintf("%02d-%s-%s-%s", declared.Ordinal, safeName(declared.ID), pass, safeName(strings.Join(order, "_")))
	fixturePath := filepath.Join(context.FixtureRoot, name)
	if err := copyFixture(context.TemplateRoot, fixturePath); err != nil {
		return PermutationResult{}, err
	}
	state := fixtureState{Files: map[string][]byte{}, Appended: map[string][]string{}, Applied: []string{}, EffectTrace: []string{}}
	if err := loadFixtureFiles(fixturePath, &state); err != nil {
		return PermutationResult{}, err
	}
	byID := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	for _, id := range order {
		candidate := byID[id]
		state.EffectTrace = append(state.EffectTrace, "effect:pre:"+strings.Join(candidate.EffectPre, "+"))
		if expectedPrior := candidate.Preconditions["prior"]; expectedPrior != "" && !contains(state.Applied, expectedPrior) {
			state.Terminal = candidate.Operation.FailureTerminal
			if state.Terminal == "" {
				state.Terminal = rule.Terminal
			}
			state.EffectTrace = append(state.EffectTrace, "terminal:"+state.Terminal)
			artifact, err := lowerFixture(fixturePath, state)
			if err != nil {
				return PermutationResult{}, err
			}
			return PermutationResult{Order: append([]string{}, order...), State: StateRefuted, GeneratedArtifact: artifact, TerminalReason: state.Terminal, OrderedEffectTrace: append([]string{}, state.EffectTrace...), AppliedCandidates: append([]string{}, state.Applied...), AtomicAbort: true}, nil
		}
		if base := candidate.Preconditions["base"]; base != "" && !fixtureContains(state.Files, base) {
			state.Terminal = "BASE_PRECONDITION_FAILED"
			state.EffectTrace = append(state.EffectTrace, "terminal:"+state.Terminal)
			artifact, err := lowerFixture(fixturePath, state)
			if err != nil {
				return PermutationResult{}, err
			}
			return PermutationResult{Order: append([]string{}, order...), State: StateRefuted, GeneratedArtifact: artifact, TerminalReason: state.Terminal, OrderedEffectTrace: append([]string{}, state.EffectTrace...), AppliedCandidates: append([]string{}, state.Applied...), AtomicAbort: true}, nil
		}
		if err := applyOperation(state, candidate); err != nil {
			return PermutationResult{}, fmt.Errorf("case %q candidate %q: %w", declared.ID, candidate.ID, err)
		}
		state.EffectTrace = append(state.EffectTrace, "effect:post:"+strings.Join(candidate.EffectPost, "+"))
	}
	if state.Terminal == "" {
		state.Terminal = rule.Terminal
	}
	artifact, err := lowerFixture(fixturePath, state)
	if err != nil {
		return PermutationResult{}, err
	}
	return PermutationResult{Order: append([]string{}, order...), State: StateClosed, GeneratedArtifact: artifact, TerminalReason: state.Terminal, OrderedEffectTrace: append([]string{}, state.EffectTrace...), AppliedCandidates: append([]string{}, state.Applied...)}, nil
}

func replayCase(context *runContext, declared CaseDecl, candidates []Candidate, rule RuleDecl, permutations [][]string, first []PermutationResult) (bool, error) {
	second := make([]PermutationResult, 0, len(permutations))
	for _, order := range permutations {
		observation, err := executePermutation(context, declared, candidates, rule, order, "pass-2")
		if err != nil {
			return false, err
		}
		second = append(second, observation)
	}
	if len(first) != len(second) {
		return false, nil
	}
	for index := range first {
		if !sameVector(first[index], second[index]) {
			return false, nil
		}
	}
	return true, nil
}

func applyOperation(state fixtureState, candidate Candidate) error {
	path := candidate.Operation.Artifact
	if !strings.HasPrefix(filepath.ToSlash(path), "compiler/") {
		return fmt.Errorf("operation artifact %q escapes compiler fixture", path)
	}
	switch candidate.Operation.Kind {
	case "append":
		state.Appended[path] = append(state.Appended[path], candidate.Operation.Value)
	case "set":
		state.Appended[path] = []string{candidate.Operation.Value}
	default:
		return fmt.Errorf("unsupported typed operation kind %q", candidate.Operation.Kind)
	}
	state.Applied = append(state.Applied, candidate.ID)
	state.Terminal = candidate.Operation.SuccessTerminal
	return nil
}

func lowerFixture(fixturePath string, state fixtureState) (ArtifactSnapshot, error) {
	paths := make([]string, 0, len(state.Appended))
	for path := range state.Appended {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		values := append([]string{}, state.Appended[relative]...)
		sort.Strings(values)
		unique := make([]string, 0, len(values))
		for _, value := range values {
			if !contains(unique, value) {
				unique = append(unique, value)
			}
		}
		content := "package fixture\n\n" + strings.Join(unique, "\n") + "\n"
		absolute := filepath.Join(fixturePath, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			return ArtifactSnapshot{}, err
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			return ArtifactSnapshot{}, err
		}
	}
	if err := validateGeneratedGo(filepath.Join(fixturePath, "compiler")); err != nil {
		return ArtifactSnapshot{}, err
	}
	return snapshotGenerated(fixturePath)
}

func snapshotGenerated(fixturePath string) (ArtifactSnapshot, error) {
	files := map[string]string{}
	generatedRoot := filepath.Join(fixturePath, "compiler", "generated")
	err := filepath.WalkDir(generatedRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(fixturePath, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = DigestBytes(data)
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return ArtifactSnapshot{}, err
	}
	digest, err := DigestValue(files)
	if err != nil {
		return ArtifactSnapshot{}, err
	}
	return ArtifactSnapshot{Files: files, Digest: digest}, nil
}

func emptyArtifact() ArtifactSnapshot {
	digest, _ := DigestValue(map[string]string{})
	return ArtifactSnapshot{Files: map[string]string{}, Digest: digest}
}

func emitBundle(context *runContext, declared CaseDecl, candidates []Candidate) (ArtifactSnapshot, error) {
	relative := filepath.ToSlash(filepath.Join("bundles", declared.ID, context.Meta.Bundle.Artifact))
	absolute := filepath.Join(context.OutputRoot, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return ArtifactSnapshot{}, err
	}
	var builder strings.Builder
	builder.WriteString("gooo combined_candidate_bundle v1\n")
	fmt.Fprintf(&builder, "bundle case=%s state=CLOSED\n", declared.ID)
	for _, candidate := range candidates {
		fmt.Fprintf(&builder, "candidate id=%s type=%s\n", candidate.ID, candidate.Type)
		fmt.Fprintf(&builder, "origin author=%s source=%s\n", candidate.Origin.Author, candidate.Origin.Source)
		fmt.Fprintf(&builder, "footprint read=%s write=%s\n", strings.Join(candidate.ReadFootprint, ","), strings.Join(candidate.WriteFootprint, ","))
		fmt.Fprintf(&builder, "effect pre=%s post=%s\n", strings.Join(candidate.EffectPre, ","), strings.Join(candidate.EffectPost, ","))
		fmt.Fprintf(&builder, "operation kind=%s artifact=%s value=\"%s\"\n", candidate.Operation.Kind, candidate.Operation.Artifact, candidate.Operation.Value)
	}
	content := []byte(builder.String())
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		return ArtifactSnapshot{}, err
	}
	context.BundleNames = append(context.BundleNames, relative)
	context.BundleBytes += len(content)
	digest := DigestBytes(content)
	return ArtifactSnapshot{Files: map[string]string{relative: digest}, Digest: digest}, nil
}

func writeEvidence(outputRoot string, evidence Evidence) error {
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(outputRoot, "evidence.json"), data, 0o644)
}

func writeReport(outputRoot string, evidence Evidence) error {
	var builder strings.Builder
	builder.WriteString("# Evolution transaction coordinator evidence\n\n")
	fmt.Fprintf(&builder, "Resolution precedence: `%s`\n\n", strings.Join(evidence.Precedence, " > "))
	builder.WriteString("| fixed cases | CLOSED | UNKNOWN | REFUTED | atomic aborts | promoted bundles |\n")
	builder.WriteString("|---:|---:|---:|---:|---:|---:|\n")
	fmt.Fprintf(&builder, "| %d | %d | %d | %d | %d | %d |\n\n", evidence.Summary.Generated, evidence.Summary.Closed, evidence.Summary.Unknown, evidence.Summary.Refuted, atomicAbortCount(evidence.Cases), promotedBundleCount(evidence.Cases))
	builder.WriteString("## Fixed cases\n\n")
	builder.WriteString("| case | expected | state | permutations | replay_equal | decision |\n|---|---|---|---:|---|---|\n")
	for _, item := range evidence.Cases {
		fmt.Fprintf(&builder, "| `%s` | `%s` | `%s` | %d | %t | `%s` |\n", item.ID, item.Expected, item.State, len(item.Permutations), item.ReplayEqual, item.Decision)
	}
	builder.WriteString("\n## Exact vector evidence\n\n")
	for _, item := range evidence.Cases {
		fmt.Fprintf(&builder, "### %s\n\n", item.ID)
		for _, permutation := range item.Permutations {
			fmt.Fprintf(&builder, "- order `%s`: state `%s`, artifact `%s`, terminal `%s`, trace `%s`, atomic_abort `%t`, promoted_bundle `%t`\n", strings.Join(permutation.Order, " → "), permutation.State, permutation.GeneratedArtifact.Digest, permutation.TerminalReason, strings.Join(permutation.OrderedEffectTrace, " → "), permutation.AtomicAbort, permutation.PromotedBundle)
		}
	}
	builder.WriteString("\n## Authority and exact integer metrics\n\n")
	fmt.Fprintf(&builder, "- repository_writes: `%d`; source_mutations: `%d`; commit_authority: `%d`; merge_authority: `%d`; release_mutation: `%d`\n", evidence.Authority.RepositoryWrites, evidence.Authority.SourceMutations, evidence.Authority.CommitAuthority, evidence.Authority.MergeAuthority, evidence.Authority.ReleaseMutation)
	fmt.Fprintf(&builder, "- inventory files/dirs: `%d`/`%d`; Go/Gooo files: `%d`/`%d`; physical_lines: `%d`\n", evidence.Metrics.Inventory.Files, evidence.Metrics.Inventory.Directories, evidence.Metrics.Inventory.GoFiles, evidence.Metrics.Inventory.GoooFiles, evidence.Metrics.Inventory.PhysicalLines)
	fmt.Fprintf(&builder, "- generated bundle files/bytes: `%d`/`%d`; artifact_count: `%d`; wall_ms: `%d`; peak_rss_kib: `%d`\n", evidence.Metrics.Generated.Files, evidence.Metrics.Generated.Bytes, evidence.ArtifactCount, evidence.Metrics.WallMS, evidence.Metrics.PeakRSSKiB)
	fmt.Fprintf(&builder, "- tests total/selected/executed/reused/failed/unknown: `%d`/`%d`/`%d`/`%d`/`%d`/`%d`\n", evidence.Metrics.Tests.Total, evidence.Metrics.Tests.Selected, evidence.Metrics.Tests.Executed, evidence.Metrics.Tests.Reused, evidence.Metrics.Tests.Failed, evidence.Metrics.Tests.Unknown)
	builder.WriteString("- root README: excluded from inventory; all fixture and output writes are caller-owned.\n")
	return os.WriteFile(filepath.Join(outputRoot, "coordinator-report.md"), []byte(builder.String()), 0o644)
}

func measureInventory(root string) (Inventory, error) {
	result := Inventory{RootReadmeExcluded: true}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			result.Directories++
			return nil
		}
		if relative == "README.md" {
			return nil
		}
		result.Files++
		extension := filepath.Ext(path)
		if extension == ".go" {
			result.GoFiles++
		}
		if extension == ".gooo" {
			result.GoooFiles++
		}
		if extension == ".go" || extension == ".gooo" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result.PhysicalLines += physicalLines(data)
		}
		return nil
	})
	return result, err
}

func physicalLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := 1
	for _, byteValue := range data {
		if byteValue == '\n' {
			lines++
		}
	}
	if data[len(data)-1] == '\n' {
		lines--
	}
	return lines
}

func peakRSSKiB() int {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil && usage.Maxrss > 0 {
		return int(usage.Maxrss)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	value := int(memory.Sys / 1024)
	if value < 1 {
		return 1
	}
	return value
}

func ensureExternalEmptyDirectory(path, sourceRoot string) error {
	if isWithin(sourceRoot, path) {
		return fmt.Errorf("path %q is inside source root %q", path, sourceRoot)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("path %q must be empty", path)
	}
	return nil
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return true
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func copyFixture(source, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func loadFixtureFiles(root string, state *fixtureState) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		state.Files[filepath.ToSlash(relative)] = data
		return nil
	})
}

func fixtureContains(files map[string][]byte, value string) bool {
	for _, data := range files {
		if strings.Contains(string(data), value) {
			return true
		}
	}
	return false
}

func validateGeneratedGo(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			return fmt.Errorf("generated Go parse failed for %q: %w", path, err)
		}
		return nil
	})
}

func summarizeCandidates(candidates []Candidate) []FootprintSummary {
	result := make([]FootprintSummary, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, FootprintSummary{
			CandidateID: candidate.ID, Read: append([]string{}, candidate.ReadFootprint...), Write: append([]string{}, candidate.WriteFootprint...),
			Origin: candidate.Origin, Capabilities: append([]string{}, candidate.Capabilities...), EffectPre: append([]string{}, candidate.EffectPre...), EffectPost: append([]string{}, candidate.EffectPost...),
			Preconditions: cloneMap(candidate.Preconditions), Postconditions: cloneMap(candidate.Postconditions),
		})
	}
	return result
}

func unionCandidateValues(candidates []Candidate, values func(Candidate) []string) []string {
	set := map[string]bool{}
	for _, candidate := range candidates {
		for _, value := range values(candidate) {
			if value != "" {
				set[value] = true
			}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func footprintConflict(candidates []Candidate) string {
	for left := 0; left < len(candidates); left++ {
		for right := left + 1; right < len(candidates); right++ {
			first, second := candidates[left], candidates[right]
			if hasPrior(first) || hasPrior(second) {
				continue
			}
			if overlap(first.WriteFootprint, second.ReadFootprint) || overlap(first.ReadFootprint, second.WriteFootprint) {
				return first.ID + "↔" + second.ID
			}
			for _, path := range first.WriteFootprint {
				if !contains(second.WriteFootprint, path) {
					continue
				}
				if first.Operation.Kind != "set" || second.Operation.Kind != "set" || first.Operation.Value != second.Operation.Value {
					return first.ID + "↔" + second.ID + ":" + path
				}
			}
		}
	}
	return ""
}

func hasPrior(candidate Candidate) bool {
	return candidate.Preconditions["prior"] != ""
}

func overlap(left, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func allPermutations(values []string) ([][]string, error) {
	if len(values) < MinimumCandidate {
		return nil, fmt.Errorf("expected at least %d candidates", MinimumCandidate)
	}
	if len(values) > 8 {
		return nil, fmt.Errorf("permutation set is limited to eight candidates")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			return nil, fmt.Errorf("candidate IDs must be non-empty and unique")
		}
		seen[value] = true
	}
	result := make([][]string, 0)
	var visit func([]string, map[string]bool)
	visit = func(prefix []string, used map[string]bool) {
		if len(prefix) == len(values) {
			result = append(result, append([]string{}, prefix...))
			return
		}
		for _, value := range values {
			if used[value] {
				continue
			}
			used[value] = true
			visit(append(prefix, value), used)
			delete(used, value)
		}
	}
	visit(nil, map[string]bool{})
	sort.Slice(result, func(left, right int) bool {
		return strings.Join(result[left], "\x00") < strings.Join(result[right], "\x00")
	})
	return result, nil
}

func sameVector(left, right PermutationResult) bool {
	return left.GeneratedArtifact.Digest == right.GeneratedArtifact.Digest && left.TerminalReason == right.TerminalReason && reflect.DeepEqual(left.OrderedEffectTrace, right.OrderedEffectTrace)
}

func allVectorsEqual(values []PermutationResult) bool {
	if len(values) < 2 {
		return true
	}
	for index := 1; index < len(values); index++ {
		if !sameVector(values[0], values[index]) {
			return false
		}
	}
	return true
}

func findRule(rules []RuleDecl, id string) (RuleDecl, bool) {
	for _, rule := range rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return RuleDecl{}, false
}

func summarize(cases []CaseResult) Summary {
	result := Summary{Generated: len(cases)}
	for _, item := range cases {
		switch item.State {
		case StateClosed:
			result.Closed++
		case StateUnknown:
			result.Unknown++
		case StateRefuted:
			result.Refuted++
		}
	}
	return result
}

func replayCount(cases []CaseResult) int {
	count := 0
	for _, item := range cases {
		if item.ReplayEqual {
			count++
		}
	}
	return count
}

func atomicAbortCount(cases []CaseResult) int {
	count := 0
	for _, item := range cases {
		if item.AtomicAbort {
			count++
		}
	}
	return count
}

func promotedBundleCount(cases []CaseResult) int {
	count := 0
	for _, item := range cases {
		if item.PromotedBundle {
			count++
		}
	}
	return count
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func safeName(value string) string {
	value = strings.ReplaceAll(value, string(filepath.Separator), "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
