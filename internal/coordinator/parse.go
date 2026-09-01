package coordinator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func ParseMeta(path string) (MetaSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MetaSource{}, err
	}
	meta := MetaSource{SourceDigest: DigestBytes(data)}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := splitFields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 || fields[1] != "evolution_transaction_coordinator" {
				return MetaSource{}, fmt.Errorf("line %d: invalid gooo header", lineNumber)
			}
			meta.Schema = MetaSchema
			meta.Version = fields[2]
		case "namespace":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid namespace", lineNumber)
			}
			meta.Namespace = fields[1]
		case "effect":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid effect", lineNumber)
			}
			meta.Effects = append(meta.Effects, fields[1])
		case "capability":
			if len(fields) < 3 {
				return MetaSource{}, fmt.Errorf("line %d: capability requires a name and effects", lineNumber)
			}
			meta.Capabilities = append(meta.Capabilities, CapabilityDecl{Name: fields[1], Effects: append([]string{}, fields[2:]...)})
		case "precedence":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid precedence", lineNumber)
			}
			meta.Precedence = strings.Split(fields[1], ">")
		case "unknown_fields":
			if len(fields) != 2 {
				return MetaSource{}, fmt.Errorf("line %d: invalid unknown_fields", lineNumber)
			}
			meta.UnknownFields = strings.Split(fields[1], ",")
		case "forbidden_combined_effects":
			if len(fields) < 2 {
				return MetaSource{}, fmt.Errorf("line %d: forbidden effect set is empty", lineNumber)
			}
			meta.ForbiddenCombinedEffects = append(meta.ForbiddenCombinedEffects, fields[1:]...)
		case "rule":
			values, err := keyValues(fields[1:])
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			meta.Rules = append(meta.Rules, RuleDecl{ID: values["id"], Condition: values["condition"], Outcome: values["outcome"], Terminal: values["terminal"]})
		case "denominator":
			values, err := keyValues(fields[1:])
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cellCount, err := integer(values, "cell_count")
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			meta.Denominator = DenominatorDecl{ID: values["id"], CellCount: cellCount}
		case "case":
			values, err := keyValues(fields[1:])
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			ordinal, err := integer(values, "ordinal")
			if err != nil {
				// The source intentionally omits ordinal; it is assigned in source order.
				ordinal = len(meta.Cases) + 1
			}
			meta.Cases = append(meta.Cases, CaseDecl{
				Ordinal: ordinal, ID: values["id"], Kind: values["kind"], Rule: values["rule"], Expected: values["expected"],
				CandidateIDs: splitList(values["candidates"]), Replay: values["replay"] == "true",
			})
		case "atomic_abort":
			values, err := keyValues(fields[1:])
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			meta.AtomicAbort = AtomicAbortDecl{States: splitList(values["states"]), PromoteBundle: values["promote_bundle"] == "true", PartialPromotion: values["partial_promotion"] == "true"}
		case "bundle":
			values, err := keyValues(fields[1:])
			if err != nil {
				return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			meta.Bundle = BundleDecl{ClosedOnly: values["closed_only"] == "true", Artifact: values["artifact"], Digest: values["digest"]}
		default:
			return MetaSource{}, fmt.Errorf("line %d: unknown declaration %q", lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return MetaSource{}, err
	}
	return meta, nil
}

func ParseCandidate(path string) (Candidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Candidate{}, err
	}
	candidate := Candidate{Schema: CandidateSchema, SourcePath: filepath.ToSlash(path), SourceDigest: DigestBytes(data), Preconditions: map[string]string{}, Postconditions: map[string]string{}}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := splitFields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 || fields[1] != "rewrite_candidate" || fields[2] != "v1" {
				return Candidate{}, fmt.Errorf("%s:%d: invalid candidate header", path, lineNumber)
			}
			candidate.Version = fields[2]
		case "candidate":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			candidate.ID = values["id"]
			candidate.Type = values["type"]
		case "origin":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			candidate.Origin = Origin{Author: values["author"], Source: values["source"]}
		case "capability":
			candidate.Capabilities = append(candidate.Capabilities, fields[1:]...)
		case "effect":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			candidate.EffectPre = splitList(values["pre"])
			candidate.EffectPost = splitList(values["post"])
		case "footprint":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			candidate.ReadFootprint = splitList(values["read"])
			candidate.WriteFootprint = splitList(values["write"])
		case "precondition":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			for key, value := range values {
				candidate.Preconditions[key] = value
			}
		case "postcondition":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			for key, value := range values {
				candidate.Postconditions[key] = value
			}
		case "operation":
			values, err := keyValues(fields[1:])
			if err != nil {
				return Candidate{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			candidate.Operation = Operation{Kind: values["kind"], Artifact: values["artifact"], Value: values["value"], SuccessTerminal: values["success_terminal"], FailureTerminal: values["failure_terminal"]}
		default:
			return Candidate{}, fmt.Errorf("%s:%d: unknown declaration %q", path, lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

func LoadCandidates(root string, paths []string) ([]Candidate, error) {
	selected := append([]string{}, paths...)
	if len(selected) == 0 {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".gooo" {
				return nil
			}
			selected = append(selected, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(selected)
	result := make([]Candidate, 0, len(selected))
	seen := map[string]bool{}
	for _, path := range selected {
		candidate, err := ParseCandidate(path)
		if err != nil {
			return nil, err
		}
		if seen[candidate.ID] {
			return nil, fmt.Errorf("duplicate candidate id %q", candidate.ID)
		}
		seen[candidate.ID] = true
		result = append(result, candidate)
	}
	return result, nil
}

func LoadContract(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}
	var contract Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		return Contract{}, fmt.Errorf("decode contract: %w", err)
	}
	if contract.Schema != ContractSchema {
		return Contract{}, fmt.Errorf("unexpected contract schema %q", contract.Schema)
	}
	return contract, nil
}

func validateDeclarations(meta MetaSource, contract Contract) error {
	if meta.Schema != MetaSchema || meta.Version != "v1" || meta.Namespace != "gooo://evolution-transaction-coordinator/v1" {
		return fmt.Errorf("meta source identity mismatch")
	}
	if contract.Version != "v1" || !contract.Fixed || contract.ID != meta.Denominator.ID || contract.CellCount != meta.Denominator.CellCount || contract.CellCount != FixedCaseCount {
		return fmt.Errorf("fixed denominator declaration mismatch")
	}
	if !sameStrings(meta.Precedence, []string{StateRefuted, StateUnknown, StateClosed}) {
		return fmt.Errorf("resolution precedence must be REFUTED>UNKNOWN>CLOSED")
	}
	if !sameStrings(meta.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) {
		return fmt.Errorf("UNKNOWN six-field contract mismatch")
	}
	if len(meta.Effects) < 6 || !containsAll(meta.Effects, []string{"READ_INPUT", "WRITE_CALLER_OUTPUT", "REPOSITORY_WRITE", "CI_MUTATION", "COMMIT", "MERGE", "RELEASE_MUTATION"}) {
		return fmt.Errorf("effect vocabulary is incomplete")
	}
	if !containsAll(meta.ForbiddenCombinedEffects, []string{"REPOSITORY_WRITE", "CI_MUTATION", "COMMIT", "MERGE", "RELEASE_MUTATION"}) {
		return fmt.Errorf("forbidden combined effect policy is incomplete")
	}
	if len(meta.Rules) != FixedCaseCount {
		return fmt.Errorf("expected exactly %d semantic rules", FixedCaseCount)
	}
	seenRules := map[string]bool{}
	for _, rule := range meta.Rules {
		if rule.ID == "" || rule.Condition == "" || rule.Outcome == "" || rule.Terminal == "" || seenRules[rule.ID] {
			return fmt.Errorf("invalid or duplicate semantic rule %q", rule.ID)
		}
		seenRules[rule.ID] = true
	}
	if len(meta.Cases) != FixedCaseCount || len(contract.Cases) != FixedCaseCount {
		return fmt.Errorf("expected exactly %d fixed cases", FixedCaseCount)
	}
	for index := 0; index < FixedCaseCount; index++ {
		metaCase, contractCase := meta.Cases[index], contract.Cases[index]
		if metaCase.Ordinal != index+1 || metaCase.ID == "" || metaCase.Rule == "" || len(metaCase.CandidateIDs) < MinimumCandidate || metaCase.ID != contractCase.ID || metaCase.Kind != contractCase.Kind || metaCase.Expected != contractCase.Expected || contractCase.Ordinal != index+1 {
			return fmt.Errorf("fixed case %d does not match the declared contract", index+1)
		}
	}
	if !sameStrings(meta.AtomicAbort.States, []string{StateUnknown, StateRefuted}) || meta.AtomicAbort.PromoteBundle || meta.AtomicAbort.PartialPromotion {
		return fmt.Errorf("atomic abort declaration must deny UNKNOWN/REFUTED promotion")
	}
	if !meta.Bundle.ClosedOnly || meta.Bundle.Artifact == "" || meta.Bundle.Digest != "sha256" {
		return fmt.Errorf("bundle declaration must be closed-only with sha256 digest")
	}
	return nil
}

func validateCandidate(candidate Candidate, meta MetaSource) error {
	if candidate.Schema != CandidateSchema || candidate.Version != "v1" || candidate.ID == "" || candidate.Type == "" {
		return fmt.Errorf("candidate %q has an invalid typed identity", candidate.ID)
	}
	if candidate.Origin.Author == "" || candidate.Origin.Source == "" || len(candidate.Capabilities) == 0 || len(candidate.EffectPre) == 0 || len(candidate.EffectPost) == 0 || candidate.Operation.Kind == "" || candidate.Operation.Artifact == "" {
		return fmt.Errorf("candidate %q is missing origin, capability, effect, or operation", candidate.ID)
	}
	if !containsAll(meta.Effects, append(append([]string{}, candidate.EffectPre...), candidate.EffectPost...)) {
		return fmt.Errorf("candidate %q uses an undeclared effect", candidate.ID)
	}
	for _, capability := range candidate.Capabilities {
		if capability != "REPOSITORY_WRITE" && !capabilityEffectDeclared(meta, capability) {
			return fmt.Errorf("candidate %q uses an undeclared capability/effect %q", candidate.ID, capability)
		}
	}
	return nil
}

func capabilityEffectDeclared(meta MetaSource, effect string) bool {
	for _, declaration := range meta.Capabilities {
		for _, item := range append([]string{declaration.Name}, declaration.Effects...) {
			if item == effect {
				return true
			}
		}
	}
	return contains(meta.Effects, effect)
}

func splitFields(line string) []string {
	var fields []string
	var builder strings.Builder
	inQuote := false
	for _, character := range strings.TrimSpace(line) {
		switch character {
		case '"':
			inQuote = !inQuote
			builder.WriteRune(character)
		case ' ', '\t':
			if inQuote {
				builder.WriteRune(character)
			} else if builder.Len() > 0 {
				fields = append(fields, builder.String())
				builder.Reset()
			}
		default:
			builder.WriteRune(character)
		}
	}
	if builder.Len() > 0 {
		fields = append(fields, builder.String())
	}
	return fields
}

func keyValues(fields []string) (map[string]string, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid key/value %q", field)
		}
		values[parts[0]] = strings.Trim(parts[1], "\"")
	}
	return values, nil
}

func integer(values map[string]string, key string) (int, error) {
	value, ok := values[key]
	if !ok || value == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, value)
	}
	return number, nil
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func containsAll(haystack, needles []string) bool {
	for _, needle := range needles {
		if !contains(haystack, needle) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
