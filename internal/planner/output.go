package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type RunInput struct {
	MetaPath     string
	ContractPath string
	InputPath    string
	OutputRoot   string
	SourceRoot   string
}

type ConformanceInput struct {
	MetaPath     string
	ContractPath string
	CorpusRoot   string
	OutputRoot   string
	SourceRoot   string
}

func Run(input RunInput) (Evidence, error) {
	started := time.Now()
	meta, contract, err := ParseAndValidate(input.MetaPath, input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	sourceRoot, err := filepath.Abs(input.SourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	outputRoot, err := filepath.Abs(input.OutputRoot)
	if err != nil {
		return Evidence{}, err
	}
	if err := ensureExternalEmptyDirectory(outputRoot, sourceRoot); err != nil {
		return Evidence{}, err
	}
	plannerInput, inputDigest, err := LoadInput(input.InputPath)
	if err != nil {
		return Evidence{}, err
	}
	contractData, err := os.ReadFile(input.ContractPath)
	if err != nil {
		return Evidence{}, err
	}
	plan, err := BuildPlan(plannerInput, meta, inputDigest)
	if err != nil {
		return Evidence{}, err
	}
	replayed, err := BuildPlan(plannerInput, meta, inputDigest)
	if err != nil {
		return Evidence{}, err
	}
	replayEqual, err := equalJSON(plan, replayed)
	if err != nil {
		return Evidence{}, err
	}
	dossier := RenderDossier(plan)
	inventory, err := measureInventory(sourceRoot)
	if err != nil {
		return Evidence{}, err
	}
	metrics := Metrics{WallMS: elapsedMS(started), PeakRSSKiB: peakRSSKiB(), Inventory: inventory, Improvement: unknownImprovement(), CI: currentCIMetrics()}
	evidence := Evidence{
		Schema: EvidenceSchema, Version: "v1", MetaSourceDigest: meta.SourceDigest,
		ContractDigest: DigestBytes(contractData), InputDigest: inputDigest, Plan: plan,
		SemanticVector: vectorFromPlan(plan), ReplayEqual: replayEqual, ImprovementState: StateUnknown,
		Improvement: nil, Authority: meta.RuntimeAuthority, Metrics: metrics, RationaleDossier: dossier,
	}
	if !replayEqual {
		return Evidence{}, fmt.Errorf("replay plan is not byte-identical")
	}
	if err := writePlanArtifacts(outputRoot, plan, evidence, dossier); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func RunConformance(input ConformanceInput) (ConformanceSummary, error) {
	started := time.Now()
	meta, contract, err := ParseAndValidate(input.MetaPath, input.ContractPath)
	if err != nil {
		return ConformanceSummary{}, err
	}
	sourceRoot, err := filepath.Abs(input.SourceRoot)
	if err != nil {
		return ConformanceSummary{}, err
	}
	outputRoot, err := filepath.Abs(input.OutputRoot)
	if err != nil {
		return ConformanceSummary{}, err
	}
	corpusRoot, err := filepath.Abs(input.CorpusRoot)
	if err != nil {
		return ConformanceSummary{}, err
	}
	if err := ensureExternalEmptyDirectory(outputRoot, sourceRoot); err != nil {
		return ConformanceSummary{}, err
	}
	contractData, err := os.ReadFile(input.ContractPath)
	if err != nil {
		return ConformanceSummary{}, err
	}
	summary := ConformanceSummary{Schema: EvidenceSchema, FixedCases: FixedCaseCount, Generated: len(meta.Cases), Authority: meta.RuntimeAuthority}
	for _, declared := range meta.Cases {
		casePath := filepath.Join(corpusRoot, filepath.FromSlash(declared.Corpus))
		plannerInput, inputDigest, loadErr := LoadInput(casePath)
		if loadErr != nil {
			return ConformanceSummary{}, fmt.Errorf("case %q: %w", declared.ID, loadErr)
		}
		plan, planErr := BuildPlan(plannerInput, meta, inputDigest)
		if planErr != nil {
			return ConformanceSummary{}, fmt.Errorf("case %q: %w", declared.ID, planErr)
		}
		replayed, planErr := BuildPlan(plannerInput, meta, inputDigest)
		if planErr != nil {
			return ConformanceSummary{}, fmt.Errorf("case %q replay: %w", declared.ID, planErr)
		}
		replayEqual, planErr := equalJSON(plan, replayed)
		if planErr != nil {
			return ConformanceSummary{}, planErr
		}
		if !replayEqual {
			return ConformanceSummary{}, fmt.Errorf("case %q replay is not byte-identical", declared.ID)
		}
		if plan.State != declared.Expected {
			return ConformanceSummary{}, fmt.Errorf("case %q evaluated as %s, expected %s", declared.ID, plan.State, declared.Expected)
		}
		dossier := RenderDossier(plan)
		caseRoot := filepath.Join(outputRoot, "cases", declared.ID)
		if err := os.MkdirAll(caseRoot, 0o755); err != nil {
			return ConformanceSummary{}, err
		}
		inventory, inventoryErr := measureInventory(sourceRoot)
		if inventoryErr != nil {
			return ConformanceSummary{}, inventoryErr
		}
		metrics := Metrics{WallMS: elapsedMS(started), PeakRSSKiB: peakRSSKiB(), Inventory: inventory, Improvement: unknownImprovement(), CI: currentCIMetrics()}
		evidence := Evidence{Schema: EvidenceSchema, Version: "v1", MetaSourceDigest: meta.SourceDigest, ContractDigest: DigestBytes(contractData), InputDigest: inputDigest, Plan: plan, SemanticVector: vectorFromPlan(plan), ReplayEqual: true, ImprovementState: StateUnknown, Improvement: nil, Authority: meta.RuntimeAuthority, Metrics: metrics, RationaleDossier: dossier}
		if err := writePlanArtifacts(caseRoot, plan, evidence, dossier); err != nil {
			return ConformanceSummary{}, err
		}
		switch plan.State {
		case StateClosed:
			summary.Closed++
		case StateUnknown:
			summary.Unknown++
		case StateRefuted:
			summary.Refuted++
		}
	}
	summary.ReplayEqual = true
	summary.Metrics = Metrics{WallMS: elapsedMS(started), PeakRSSKiB: peakRSSKiB(), Inventory: mustInventory(sourceRoot), Improvement: unknownImprovement(), CI: currentCIMetrics()}
	if err := writeJSON(filepath.Join(outputRoot, "conformance-summary.json"), summary); err != nil {
		return ConformanceSummary{}, err
	}
	if err := writeConformanceReport(outputRoot, meta, summary); err != nil {
		return ConformanceSummary{}, err
	}
	return summary, nil
}

func vectorFromPlan(plan Plan) SemanticVector {
	return SemanticVector{State: plan.State, Decision: plan.Decision, CandidateDecisions: plan.CandidateDecisions, Waves: plan.Waves, SerializedBoundaries: plan.SerializedBoundaries, BlockedCausalFrontier: plan.BlockedCausalFrontier, SingleWriterCells: plan.SingleWriterCells, DeniedOperations: plan.DeniedOperations}
}

func RenderDossier(plan Plan) string {
	var builder strings.Builder
	builder.WriteString("# Semantic work-wave planner rationale dossier\n\n")
	fmt.Fprintf(&builder, "- decision: `%s`\n- state: `%s`\n- input digest: `%s`\n- target repository: `%s`\n- target ledger: `%s`\n- runtime mutation authority: `0`\n\n", plan.Decision, plan.State, plan.InputDigest, plan.Target.Repository, plan.Target.Ledger)
	builder.WriteString("The plan is a fixed point of explicit evidence. Candidate order is a canonical lexical tie-break only; no score, percentage, or heuristic optimization is used.\n\n")
	builder.WriteString("## Exact parallel waves\n\n")
	if len(plan.Waves) == 0 {
		builder.WriteString("No candidate is currently executable.\n\n")
	}
	for _, wave := range plan.Waves {
		fmt.Fprintf(&builder, "- wave `%d`: `%s`; parallel=`%t`; final=`%t` — %s\n", wave.Ordinal, strings.Join(wave.CandidateIDs, ", "), wave.Parallel, wave.Final, wave.Rationale)
	}
	builder.WriteString("\n## Serialized boundaries\n\n")
	if len(plan.SerializedBoundaries) == 0 {
		builder.WriteString("No semantic serialization boundary was declared.\n\n")
	}
	for _, boundary := range plan.SerializedBoundaries {
		fmt.Fprintf(&builder, "- `%s` → `%s`: `%s` on `%s` — %s\n", boundary.Before, boundary.After, strings.Join(boundary.Kinds, ", "), strings.Join(boundary.Cells, ", "), boundary.Rationale)
	}
	builder.WriteString("\n## Blocked causal frontier\n\n")
	if len(plan.BlockedCausalFrontier) == 0 {
		builder.WriteString("No candidate is blocked by unresolved causal evidence.\n\n")
	}
	for _, item := range plan.BlockedCausalFrontier {
		fmt.Fprintf(&builder, "- `%s`: `%s` / `%s` / `%s`; next=`%s`; blocked_by=`%s`\n", item.CandidateID, item.Unknown.Stage, item.Unknown.UnknownClass, item.Unknown.Reason, item.Unknown.NextOperation, strings.Join(item.Unknown.BlockedBy, ", "))
	}
	builder.WriteString("\n## Single-writer cells\n\n")
	if len(plan.SingleWriterCells) == 0 {
		builder.WriteString("No mutable ledger writer was declared.\n\n")
	}
	for _, cell := range plan.SingleWriterCells {
		fmt.Fprintf(&builder, "- cell `%s`: writer `%s` from candidate `%s`, wave `%d`\n", cell.Cell, cell.Writer, cell.CandidateID, cell.Wave)
	}
	builder.WriteString("\n## Denied operations\n\n")
	if len(plan.DeniedOperations) == 0 {
		builder.WriteString("No operation was denied.\n\n")
	}
	for _, denied := range plan.DeniedOperations {
		fmt.Fprintf(&builder, "- `%s`: `%s`; caused_by=`%s`\n", denied.CandidateID, denied.Reason, strings.Join(denied.CausedBy, ", "))
	}
	builder.WriteString("\n## Candidate decisions\n\n")
	for _, candidate := range plan.CandidateDecisions {
		fmt.Fprintf(&builder, "- `%s`: `%s` (`%s`)\n", candidate.CandidateID, candidate.State, candidate.Reason)
	}
	return builder.String()
}

func writePlanArtifacts(root string, plan Plan, evidence Evidence, dossier string) error {
	if err := writeJSON(filepath.Join(root, "semantic-plan.json"), plan); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(root, "evidence.json"), evidence); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "rationale-dossier.md"), []byte(dossier), 0o644); err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, "metrics.json"), evidence.Metrics)
}

func writeConformanceReport(root string, meta MetaSource, summary ConformanceSummary) error {
	var builder strings.Builder
	builder.WriteString("# Semantic work-wave planner conformance\n\n")
	fmt.Fprintf(&builder, "Fixed denominator: `%d` cases; resolution precedence: `REFUTED > UNKNOWN > CLOSED`; fixed point: `FIXED_POINT`.\n\n", summary.FixedCases)
	builder.WriteString("| generated | CLOSED | UNKNOWN | REFUTED | replay_equal |\n|---:|---:|---:|---:|---|\n")
	fmt.Fprintf(&builder, "| %d | %d | %d | %d | %t |\n\n", summary.Generated, summary.Closed, summary.Unknown, summary.Refuted, summary.ReplayEqual)
	builder.WriteString("The `.gooo` source owns scheduling semantics, conflict ontology, and meta activities. Go only parses, plans, projects, and generates caller-owned evidence.\n\n")
	builder.WriteString("## Meta activities\n\n")
	for _, activity := range meta.Activities {
		fmt.Fprintf(&builder, "- `%s` (%s) → `%s`\n", activity.ID, activity.Stage, activity.Output)
	}
	builder.WriteString("\n## Metrics\n\n")
	fmt.Fprintf(&builder, "- inventory files/dirs `%d`/`%d`; Go/Gooo files `%d`/`%d`; physical lines `%d`; root README excluded `%t`\n", summary.Metrics.Inventory.Files, summary.Metrics.Inventory.Directories, summary.Metrics.Inventory.GoFiles, summary.Metrics.Inventory.GoooFiles, summary.Metrics.Inventory.PhysicalLines, summary.Metrics.Inventory.RootReadmeExcluded)
	fmt.Fprintf(&builder, "- wall_ms `%d`; peak_rss_kib `%d`; improvement `%s` (`null` unless exact matched before/after)\n", summary.Metrics.WallMS, summary.Metrics.PeakRSSKiB, summary.Metrics.Improvement.State)
	return os.WriteFile(filepath.Join(root, "conformance-report.md"), []byte(builder.String()), 0o644)
}

func equalJSON(left, right any) (bool, error) {
	first, err := json.MarshalIndent(left, "", "  ")
	if err != nil {
		return false, err
	}
	second, err := json.MarshalIndent(right, "", "  ")
	if err != nil {
		return false, err
	}
	return bytes.Equal(append(first, '\n'), append(second, '\n')), nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func unknownImprovement() ImprovementEvidence {
	return ImprovementEvidence{State: StateUnknown, Value: nil, Reason: "NO_EXACT_MATCHED_BEFORE_AFTER"}
}

func currentCIMetrics() CIMetrics {
	metrics := CIMetrics{State: StateUnknown, Source: "none"}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		metrics.Source = "github-actions"
	}
	if value := os.Getenv("GITHUB_RUN_ID"); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			metrics.RunID = &parsed
		}
	}
	if value := os.Getenv("GITHUB_SHA"); value != "" {
		metrics.CommitSHA = &value
	}
	return metrics
}

func elapsedMS(start time.Time) int {
	value := int(time.Since(start).Milliseconds())
	if value < 1 {
		return 1
	}
	return value
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

func mustInventory(root string) Inventory {
	result, err := measureInventory(root)
	if err != nil {
		return Inventory{RootReadmeExcluded: true}
	}
	return result
}

func physicalLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := 1
	for _, value := range data {
		if value == '\n' {
			lines++
		}
	}
	if data[len(data)-1] == '\n' {
		lines--
	}
	return lines
}

func ensureExternalEmptyDirectory(path, sourceRoot string) error {
	if isWithin(sourceRoot, path) {
		return fmt.Errorf("output path %q is inside source root %q", path, sourceRoot)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output path %q must be empty", path)
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
