package planner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
			if len(fields) != 3 || fields[1] != "semantic_work_wave_planner" {
				return MetaSource{}, fmt.Errorf("line %d: invalid gooo header", lineNumber)
			}
			meta.Schema, meta.Version = MetaSchema, fields[2]
		case "namespace":
			meta.Namespace, err = singleValue(fields, lineNumber, "namespace")
		case "semantic_authority_id":
			meta.SemanticAuthorityID, err = singleValue(fields, lineNumber, "semantic_authority_id")
		case "repository_identity":
			meta.RepositoryIdentity, err = singleValue(fields, lineNumber, "repository_identity")
		case "read_set":
			meta.ReadSet, err = listValue(fields, lineNumber, "read_set")
		case "write_set":
			meta.WriteSet, err = listValue(fields, lineNumber, "write_set")
		case "immutable_input_release":
			meta.ImmutableInputRelease, err = parseRelease(fields[1:])
		case "expected_output_release":
			meta.ExpectedOutputRelease, err = parseRelease(fields[1:])
		case "effect":
			if len(fields) != 2 {
				err = fmt.Errorf("line %d: invalid effect", lineNumber)
			} else {
				meta.Effects = append(meta.Effects, fields[1])
			}
		case "capability":
			if len(fields) < 3 {
				err = fmt.Errorf("line %d: capability requires a name and effects", lineNumber)
			} else {
				meta.Capabilities = append(meta.Capabilities, CapabilityDecl{Name: fields[1], Effects: append([]string{}, fields[2:]...)})
			}
		case "precedence":
			meta.Precedence, err = commaValue(fields, lineNumber, "precedence", ">")
		case "unknown_fields":
			meta.UnknownFields, err = commaValue(fields, lineNumber, "unknown_fields", ",")
		case "forbidden_authority":
			meta.ForbiddenAuthorities, err = listFields(fields, lineNumber, "forbidden_authority")
		case "conflict":
			if len(fields) != 2 {
				err = fmt.Errorf("line %d: invalid conflict", lineNumber)
			} else {
				meta.ConflictOntology = append(meta.ConflictOntology, fields[1])
			}
		case "fixed_point":
			meta.FixedPoint, err = singleValue(fields, lineNumber, "fixed_point")
		case "proof":
			if len(fields) != 2 {
				err = fmt.Errorf("line %d: invalid proof", lineNumber)
			} else {
				meta.ProofChoices = append(meta.ProofChoices, fields[1])
			}
		case "indicator":
			if len(fields) != 2 {
				err = fmt.Errorf("line %d: invalid indicator", lineNumber)
			} else {
				meta.IndicatorClasses = append(meta.IndicatorClasses, fields[1])
			}
		case "prohibition":
			if len(fields) != 2 {
				err = fmt.Errorf("line %d: invalid prohibition", lineNumber)
			} else {
				meta.Prohibitions = append(meta.Prohibitions, fields[1])
			}
		case "runtime_authority":
			values, parseErr := keyValues(fields[1:])
			if parseErr == nil {
				meta.RuntimeAuthority, parseErr = parseAuthorityReceipt(values)
			}
			err = parseErr
		case "denominator":
			values, parseErr := keyValues(fields[1:])
			if parseErr == nil {
				var cellCount int
				cellCount, parseErr = integer(values, "cell_count")
				meta.Denominator = DenominatorDecl{ID: values["id"], CellCount: cellCount}
			}
			err = parseErr
		case "meta_activity":
			values, parseErr := keyValues(fields[1:])
			meta.Activities = append(meta.Activities, ActivityDecl{ID: values["id"], Stage: values["stage"], Output: values["output"]})
			err = parseErr
		case "case":
			values, parseErr := keyValues(fields[1:])
			if parseErr == nil {
				var ordinal int
				ordinal, parseErr = integer(values, "ordinal")
				meta.Cases = append(meta.Cases, MetaCase{Ordinal: ordinal, ID: values["id"], Expected: values["expected"], Corpus: values["corpus"], ProofChoice: values["proof_choice"], IndicatorClass: values["indicator_class"]})
			}
			err = parseErr
		default:
			err = fmt.Errorf("line %d: unknown declaration %q", lineNumber, fields[0])
		}
		if err != nil {
			return MetaSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return MetaSource{}, err
	}
	return meta, nil
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

func LoadInput(path string) (Input, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Input{}, "", err
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return Input{}, "", fmt.Errorf("decode input: %w", err)
	}
	normalizeInput(&input)
	return input, DigestBytes(data), nil
}

func normalizeInput(input *Input) {
	if input.InputRelease == nil && input.ImmutableInputRelease != nil {
		input.InputRelease = input.ImmutableInputRelease
	}
	if input.Target.Repository == "" {
		input.Target.Repository = input.TargetRepository
	}
	if input.Target.Ledger == "" {
		input.Target.Ledger = input.TargetLedger
	}
	for index := range input.Candidates {
		candidate := &input.Candidates[index]
		if candidate.OperationIdentity.CandidateID == "" {
			candidate.OperationIdentity.CandidateID = candidate.CandidateID
		}
		if candidate.OperationIdentity.OperationDigest == "" {
			candidate.OperationIdentity.OperationDigest = candidate.OperationDigest
		}
		if candidate.OperationIdentity.Kind == "" {
			candidate.OperationIdentity.Kind = candidate.OperationKind
		}
	}
}

func ValidateDeclarations(meta MetaSource, contract Contract) error {
	if meta.Schema != MetaSchema || meta.Version != "v1" || meta.Namespace != "gooo://semantic-work-wave-planner/v1" {
		return fmt.Errorf("meta source identity mismatch")
	}
	if contract.Version != "v1" || !contract.Fixed || contract.AppendOnlyFrom != "contracts/denominator-v2.json" || contract.ID != meta.Denominator.ID || contract.CellCount != FixedCaseCount || contract.CellCount != meta.Denominator.CellCount {
		return fmt.Errorf("fixed denominator declaration mismatch")
	}
	if meta.SemanticAuthorityID == "" || meta.RepositoryIdentity == "" || len(meta.ReadSet) == 0 || len(meta.WriteSet) == 0 || meta.ImmutableInputRelease.Repository == "" || meta.ExpectedOutputRelease.Repository == "" {
		return fmt.Errorf("source identity declaration is incomplete")
	}
	if !sameStrings(meta.Precedence, []string{StateRefuted, StateUnknown, StateClosed}) {
		return fmt.Errorf("resolution precedence must be REFUTED>UNKNOWN>CLOSED")
	}
	if !sameStrings(meta.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) {
		return fmt.Errorf("UNKNOWN six-field contract mismatch")
	}
	if !sameStrings(meta.ForbiddenAuthorities, []string{"REPOSITORY_WRITE", "CI_MUTATION", "COMMIT", "MERGE", "RELEASE_MUTATION", "EXECUTE"}) {
		return fmt.Errorf("forbidden authority ontology mismatch")
	}
	if !sameStrings(meta.ConflictOntology, []string{"WRITE_WRITE", "WRITE_READ", "MUTABLE_AUTHORITY", "SINGLE_LEDGER_WRITER", "DEPENDENCY"}) {
		return fmt.Errorf("conflict ontology mismatch")
	}
	if meta.FixedPoint != "FIXED_POINT" || !sameStrings(meta.ProofChoices, []string{"FOUNDATION", "COHERENCE", "REGRESSION"}) || !sameStrings(meta.IndicatorClasses, []string{"DRIVER", "OUTCOME", "GUARDRAIL"}) || !sameStrings(meta.Prohibitions, []string{"AGGREGATE_SCORE", "PERCENTAGE", "HEURISTIC_OPTIMIZATION"}) {
		return fmt.Errorf("fixed proof, indicator, or prohibition declaration mismatch")
	}
	if meta.RuntimeAuthority != (AuthorityReceipt{}) {
		return fmt.Errorf("planner runtime mutation authority must be zero")
	}
	if len(meta.Activities) != 10 {
		return fmt.Errorf("expected exactly 10 meta activities")
	}
	seenActivities := map[string]bool{}
	for _, activity := range meta.Activities {
		if activity.ID == "" || activity.Stage == "" || activity.Output == "" || seenActivities[activity.ID] {
			return fmt.Errorf("invalid or duplicate meta activity %q", activity.ID)
		}
		seenActivities[activity.ID] = true
	}
	if len(meta.Cases) != FixedCaseCount || len(contract.Cases) != FixedCaseCount {
		return fmt.Errorf("expected exactly %d fixed corpus cases", FixedCaseCount)
	}
	for index := 0; index < FixedCaseCount; index++ {
		left, right := meta.Cases[index], contract.Cases[index]
		if left.Ordinal != index+1 || right.Ordinal != index+1 || left.ID == "" || left.ID != right.ID || left.Expected != right.Expected || left.Corpus != right.Corpus || left.ProofChoice != right.ProofChoice || left.IndicatorClass != right.IndicatorClass || !validState(left.Expected) || !validProofChoice(left.ProofChoice) || !validIndicator(left.IndicatorClass) {
			return fmt.Errorf("fixed case %d does not match the declared contract", index+1)
		}
	}
	return nil
}

func ParseAndValidate(metaPath, contractPath string) (MetaSource, Contract, error) {
	meta, err := ParseMeta(metaPath)
	if err != nil {
		return MetaSource{}, Contract{}, err
	}
	contract, err := LoadContract(contractPath)
	if err != nil {
		return MetaSource{}, Contract{}, err
	}
	if err := ValidateDeclarations(meta, contract); err != nil {
		return MetaSource{}, Contract{}, err
	}
	return meta, contract, nil
}

func parseRelease(fields []string) (ReleaseIdentity, error) {
	values, err := keyValues(fields)
	if err != nil {
		return ReleaseIdentity{}, err
	}
	release := ReleaseIdentity{Repository: values["repository"], Tag: values["tag"], Digest: values["digest"]}
	if release.Repository == "" || release.Tag == "" || release.Digest == "" {
		return ReleaseIdentity{}, fmt.Errorf("release identity requires repository, tag, and digest")
	}
	return release, nil
}

func parseAuthorityReceipt(values map[string]string) (AuthorityReceipt, error) {
	read := func(key string) (int, error) { return integer(values, key) }
	fields := []struct {
		key   string
		value *int
	}{
		{"repository_writes", nil}, {"source_mutations", nil}, {"commit_authority", nil}, {"merge_authority", nil},
		{"release_mutation", nil}, {"execution_authority", nil}, {"local_test_executions", nil}, {"local_format_executions", nil},
	}
	for index := range fields {
		value, err := read(fields[index].key)
		if err != nil {
			return AuthorityReceipt{}, err
		}
		fields[index].value = &value
	}
	return AuthorityReceipt{RepositoryWrites: *fields[0].value, SourceMutations: *fields[1].value, CommitAuthority: *fields[2].value, MergeAuthority: *fields[3].value, ReleaseMutation: *fields[4].value, ExecutionAuthority: *fields[5].value, LocalTestExecutions: *fields[6].value, LocalFormatExecution: *fields[7].value}, nil
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

func singleValue(fields []string, lineNumber int, name string) (string, error) {
	if len(fields) == 2 && fields[0] == name && strings.TrimSpace(fields[1]) != "" {
		return strings.Trim(fields[1], "\""), nil
	}
	if len(fields) == 1 {
		parts := strings.SplitN(fields[0], "=", 2)
		if len(parts) == 2 && parts[0] == name && strings.TrimSpace(parts[1]) != "" {
			return strings.Trim(parts[1], "\""), nil
		}
	}
	return "", fmt.Errorf("line %d: invalid %s", lineNumber, name)
}

func listValue(fields []string, lineNumber int, name string) ([]string, error) {
	value, err := singleValue(fields, lineNumber, name)
	if err != nil {
		return nil, err
	}
	return splitList(value), nil
}

func listFields(fields []string, lineNumber int, name string) ([]string, error) {
	if len(fields) < 2 {
		return nil, fmt.Errorf("line %d: empty %s", lineNumber, name)
	}
	result := make([]string, 0, len(fields)-1)
	for _, value := range fields[1:] {
		if value == "" {
			return nil, fmt.Errorf("line %d: empty %s value", lineNumber, name)
		}
		result = append(result, value)
	}
	return result, nil
}

func commaValue(fields []string, lineNumber, name, separator string) ([]string, error) {
	value, err := singleValue(fields, lineNumber, name)
	if err != nil {
		return nil, err
	}
	if separator == ">" {
		return strings.Split(value, separator), nil
	}
	return splitList(value), nil
}

func integer(values map[string]string, key string) (int, error) {
	value, ok := values[key]
	if !ok || value == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, value)
	}
	return parsed, nil
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

func validState(state string) bool {
	return state == StateClosed || state == StateUnknown || state == StateRefuted
}

func validProofChoice(choice string) bool {
	return choice == "FOUNDATION" || choice == "COHERENCE" || choice == "REGRESSION"
}

func validIndicator(indicator string) bool {
	return indicator == "DRIVER" || indicator == "OUTCOME" || indicator == "GUARDRAIL"
}
