package observer

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"strconv"
	"strings"
)

func LoadDenominator(path string) (Denominator, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Denominator{}, nil, err
	}
	var denominator Denominator
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&denominator); err != nil {
		return Denominator{}, nil, err
	}
	if err := ValidateDenominator(denominator); err != nil {
		return Denominator{}, nil, err
	}
	return denominator, raw, nil
}

func ValidateDenominator(denominator Denominator) error {
	if denominator.Schema != ContractSchema {
		return errors.New("INVALID_DENOMINATOR_SCHEMA")
	}
	if denominator.Total != 12 || len(denominator.Cells) != 12 {
		return errors.New("DENOMINATOR_MUST_HAVE_EXACTLY_12_CELLS")
	}
	if !validBalance(denominator.Proofs, []string{"FOUNDATION", "COHERENCE", "REGRESSION"}, true) {
		return errors.New("UNBALANCED_PROOF_CHOICES")
	}
	if !validBalance(denominator.IndicatorClasses, []string{"DRIVER", "OUTCOME", "GUARDRAIL"}, false) {
		return errors.New("UNBALANCED_INDICATOR_CLASSES")
	}
	seenIDs := map[string]bool{}
	seenActivities := map[string]bool{}
	proofCounts := map[string]int{}
	indicatorCounts := map[string]int{}
	for index, cell := range denominator.Cells {
		if cell.Ordinal != index+1 || cell.ID == "" || cell.Activity == "" || cell.MetricID == "" || cell.MetricPath == "" || cell.Artifact == "" || cell.Evaluator == "" {
			return errors.New("INVALID_DENOMINATOR_CELL")
		}
		if !contains(ArtifactKinds, cell.Artifact) || seenIDs[cell.ID] || seenActivities[cell.Activity] {
			return errors.New("INVALID_OR_DUPLICATE_DENOMINATOR_CELL")
		}
		seenIDs[cell.ID] = true
		seenActivities[cell.Activity] = true
		proofCounts[cell.ProofChoice]++
		indicatorCounts[cell.IndicatorClass]++
	}
	if proofCounts["FOUNDATION"] != 4 || proofCounts["COHERENCE"] != 4 || proofCounts["REGRESSION"] != 4 {
		return errors.New("PROOF_CHOICES_MUST_EACH_HAVE_4_CELLS")
	}
	if indicatorCounts["DRIVER"] != 4 || indicatorCounts["OUTCOME"] != 4 || indicatorCounts["GUARDRAIL"] != 4 {
		return errors.New("INDICATOR_CLASSES_MUST_EACH_HAVE_4_CELLS")
	}
	return nil
}

func validBalance(values []Balance, expected []string, proof bool) bool {
	if len(values) != len(expected) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range values {
		name := value.Class
		if proof {
			name = value.Choice
		}
		if value.Total != 4 || !contains(expected, name) || seen[name] {
			return false
		}
		seen[name] = true
	}
	return len(seen) == len(expected)
}

func CompileSource(sourcePath string, source []byte, denominator Denominator) (SemanticIR, error) {
	if err := ValidateDenominator(denominator); err != nil {
		return SemanticIR{}, err
	}
	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "@gooo schema=\"gooo/semantic-observer/v1\"") {
		return SemanticIR{}, errors.New("GOOO_SOURCE_SCHEMA_HEADER_MISSING")
	}
	var nodes []ActivityIR
	for lineNumber, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "activity ") {
			continue
		}
		attrs, err := parseAttributes(strings.TrimPrefix(line, "activity "))
		if err != nil {
			return SemanticIR{}, fmt.Errorf("source line %d: %w", lineNumber+1, err)
		}
		node := ActivityIR{
			ID:             attrs["id"],
			Activity:       attrs["activity"],
			Name:           attrs["name"],
			ProofChoice:    attrs["proof"],
			IndicatorClass: attrs["indicator"],
			MetricID:       attrs["metric_id"],
			MetricPath:     attrs["metric_path"],
			Artifact:       attrs["artifact"],
			Evaluator:      attrs["evaluator"],
			SourceLine:     lineNumber + 1,
		}
		if node.ID == "" || node.Activity == "" || node.Name == "" || node.ProofChoice == "" || node.IndicatorClass == "" || node.MetricID == "" || node.MetricPath == "" || node.Artifact == "" || node.Evaluator == "" {
			return SemanticIR{}, fmt.Errorf("source line %d: INCOMPLETE_ACTIVITY_METADATA", lineNumber+1)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) != 12 {
		return SemanticIR{}, errors.New("GOOO_SOURCE_MUST_DEFINE_EXACTLY_12_ACTIVITIES")
	}
	for index, cell := range denominator.Cells {
		node := nodes[index]
		if node.ID != activityIDForCell(cell.ID) || node.Activity != cell.Activity || node.ProofChoice != cell.ProofChoice || node.IndicatorClass != cell.IndicatorClass || node.MetricID != cell.MetricID || node.MetricPath != cell.MetricPath || node.Artifact != cell.Artifact || node.Evaluator != cell.Evaluator {
			return SemanticIR{}, fmt.Errorf("activity binding mismatch at denominator cell %d", cell.Ordinal)
		}
	}
	return SemanticIR{Schema: IRSchema, SourcePath: sourcePath, SourceDigest: DigestBytes(source), Nodes: nodes}, nil
}

func activityIDForCell(cellID string) string {
	return "gooo://semantic-observer/activity/" + strings.TrimPrefix(cellID, "gooo.cell.semantic-observer.")
}

func SemanticIRBytes(ir SemanticIR) ([]byte, error) {
	raw, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func GenerateGo(ir SemanticIR, semanticDigest string, contractDigest string) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("// Code generated by gooo. DO NOT EDIT.\n\n")
	builder.WriteString("package generated\n\n")
	builder.WriteString("type Activity struct {\n")
	builder.WriteString("\tID string\n\tActivity string\n\tName string\n\tProofChoice string\n\tIndicatorClass string\n\tMetricID string\n\tMetricPath string\n\tArtifact string\n\tEvaluator string\n")
	builder.WriteString("}\n\n")
	fmt.Fprintf(&builder, "const SourcePath = %q\n", ir.SourcePath)
	fmt.Fprintf(&builder, "const SourceDigest = %q\n", ir.SourceDigest)
	fmt.Fprintf(&builder, "const SemanticIRPath = %q\n", "internal/generated/semantic-ir.json")
	fmt.Fprintf(&builder, "const SemanticIRDigest = %q\n", semanticDigest)
	fmt.Fprintf(&builder, "const ContractPath = %q\n", "contracts/semantic-observer-denominator-v1.json")
	fmt.Fprintf(&builder, "const ContractDigest = %q\n", contractDigest)
	fmt.Fprintf(&builder, "const ActivityCount = %d\n\n", len(ir.Nodes))
	builder.WriteString("var Activities = []Activity{\n")
	for _, node := range ir.Nodes {
		fmt.Fprintf(&builder, "\t{ID: %q, Activity: %q, Name: %q, ProofChoice: %q, IndicatorClass: %q, MetricID: %q, MetricPath: %q, Artifact: %q, Evaluator: %q},\n", node.ID, node.Activity, node.Name, node.ProofChoice, node.IndicatorClass, node.MetricID, node.MetricPath, node.Artifact, node.Evaluator)
	}
	builder.WriteString("}\n")
	return format.Source([]byte(builder.String()))
}

func parseAttributes(input string) (map[string]string, error) {
	attrs := map[string]string{}
	for position := 0; position < len(input); {
		for position < len(input) && input[position] == ' ' {
			position++
		}
		if position == len(input) {
			break
		}
		keyStart := position
		for position < len(input) && input[position] != '=' && input[position] != ' ' {
			position++
		}
		if keyStart == position || position >= len(input) || input[position] != '=' {
			return nil, errors.New("INVALID_ATTRIBUTE")
		}
		key := input[keyStart:position]
		position++
		if position >= len(input) || input[position] != '"' {
			return nil, errors.New("ATTRIBUTES_MUST_BE_QUOTED")
		}
		valueStart := position
		position++
		for position < len(input) {
			if input[position] == '"' && input[position-1] != '\\' {
				position++
				break
			}
			position++
		}
		if position > len(input) || position == 0 || input[position-1] != '"' {
			return nil, errors.New("UNTERMINATED_ATTRIBUTE")
		}
		value, err := strconv.Unquote(input[valueStart:position])
		if err != nil {
			return nil, err
		}
		attrs[key] = value
	}
	return attrs, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
