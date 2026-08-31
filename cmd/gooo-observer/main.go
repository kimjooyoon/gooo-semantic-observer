package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kimjooyoon/gooo-semantic-observer/internal/observer"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "compile":
		return compile(args[1:], stdout, stderr)
	case "observe":
		return observe(args[1:], stdout, stderr)
	case "conformance":
		return conformance(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "gooo-semantic-observer/v0.1.1")
		return 0
	default:
		usage(stderr)
		return 2
	}
}

func compile(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("compile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcePath := flags.String("source", "examples/semantic-observer/main.gooo", "Gooo source")
	contractPath := flags.String("contract", "contracts/semantic-observer-denominator-v1.json", "fixed denominator")
	outputIR := flags.String("output-ir", "internal/generated/semantic-ir.json", "semantic IR output")
	outputGo := flags.String("output-go", "internal/generated/semantic.gooo.go", "generated Go output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	source, err := os.ReadFile(*sourcePath)
	if err != nil {
		fmt.Fprintf(stderr, "read source: %v\n", err)
		return 1
	}
	denominator, contract, err := observer.LoadDenominator(*contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "read denominator: %v\n", err)
		return 1
	}
	ir, err := observer.CompileSource(*sourcePath, source, denominator)
	if err != nil {
		fmt.Fprintf(stderr, "compile source: %v\n", err)
		return 1
	}
	irBytes, err := observer.SemanticIRBytes(ir)
	if err != nil {
		fmt.Fprintf(stderr, "encode semantic IR: %v\n", err)
		return 1
	}
	goBytes, err := observer.GenerateGo(ir, observer.DigestBytes(irBytes), observer.DigestBytes(contract))
	if err != nil {
		fmt.Fprintf(stderr, "generate Go: %v\n", err)
		return 1
	}
	if err := writeFile(*outputIR, irBytes); err != nil {
		fmt.Fprintf(stderr, "write semantic IR: %v\n", err)
		return 1
	}
	if err := writeFile(*outputGo, goBytes); err != nil {
		fmt.Fprintf(stderr, "write generated Go: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "compiled source=%s semantic_ir=%s generated_go=%s\n", *sourcePath, *outputIR, *outputGo)
	return 0
}

func observe(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("observe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	inputPath := flags.String("input", "", "caller-owned input JSON")
	outputDir := flags.String("output-dir", "artifacts/observation", "caller-owned temporary output directory")
	inventoryRoot := flags.String("inventory-root", "", "input inventory root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inputPath == "" {
		fmt.Fprintln(stderr, "observe requires -input")
		return 2
	}
	report, err := observeFile(*inputPath, *outputDir, *inventoryRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "observe: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s decision=%s %s\n", report.CaseID, report.Decision, report.ExactStateSummary())
	return 0
}

func conformance(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixtureDir := flags.String("fixtures", "fixtures", "fixture directory")
	outputDir := flags.String("output-dir", "artifacts/conformance", "caller-owned output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	entries, err := os.ReadDir(*fixtureDir)
	if err != nil {
		fmt.Fprintf(stderr, "read fixtures: %v\n", err)
		return 1
	}
	var reports []observer.Report
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		caseName := strings.TrimSuffix(entry.Name(), ".json")
		expected, ok := expectedDecision(caseName)
		if !ok {
			fmt.Fprintf(stderr, "unexpected fixture: %s\n", entry.Name())
			return 1
		}
		report, evalErr := observeFile(filepath.Join(*fixtureDir, entry.Name()), filepath.Join(*outputDir, caseName), *fixtureDir, stderr)
		if evalErr != nil {
			fmt.Fprintf(stderr, "%s: %v\n", entry.Name(), evalErr)
			return 1
		}
		if report.Decision != expected {
			fmt.Fprintf(stderr, "%s: expected %s, got %s\n", entry.Name(), expected, report.Decision)
			return 1
		}
		if err := report.ValidateUnknowns(); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", entry.Name(), err)
			return 1
		}
		reports = append(reports, report)
	}
	if len(reports) != 12 {
		fmt.Fprintf(stderr, "canonical fixture count = %d, want 12\n", len(reports))
		return 1
	}
	if err := verifyReceiptSubstitutionCounterexample(*outputDir); err != nil {
		fmt.Fprintf(stderr, "receipt substitution counterexample: %v\n", err)
		return 1
	}
	counts := map[string]int{}
	classes := map[string]bool{}
	for _, report := range reports {
		counts[report.Decision]++
		for _, unknown := range report.Unknowns {
			classes[unknown.UnknownClass] = true
		}
		if len(report.ArtifactKinds) != 6 || !sameStrings(report.ArtifactKinds, observer.ArtifactKinds) {
			fmt.Fprintln(stderr, "artifact kind contract violated")
			return 1
		}
	}
	if counts[observer.DecisionClosed] != 3 || counts[observer.DecisionUnknown] != 4 || counts[observer.DecisionRefuted] != 5 {
		fmt.Fprintf(stderr, "decision counts = %#v, want CLOSED=3 UNKNOWN=4 REFUTED=5\n", counts)
		return 1
	}
	for _, class := range []string{"DIRECT_MISSING", "DEPENDENCY_BLOCKED", "STALE", "AMBIGUOUS"} {
		if !classes[class] {
			fmt.Fprintf(stderr, "missing UNKNOWN class %s\n", class)
			return 1
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].CaseID < reports[j].CaseID })
	summary := observer.HumanConformanceSummary(reports)
	if err := writeFile(filepath.Join(*outputDir, "ci-summary.md"), []byte(summary)); err != nil {
		fmt.Fprintf(stderr, "write CI summary: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, summary)
	return 0
}

func observeFile(inputPath, outputDir, inventoryRoot string, stderr io.Writer) (observer.Report, error) {
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return observer.Report{}, err
	}
	if err := ensureEmptyDirectory(outputDir); err != nil {
		return observer.Report{}, err
	}
	sourcePath := "examples/semantic-observer/main.gooo"
	contractPath := "contracts/semantic-observer-denominator-v1.json"
	semanticPath := "internal/generated/semantic-ir.json"
	generatedPath := "internal/generated/semantic.gooo.go"
	evaluatorPath := "internal/observer/evaluate.go"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return observer.Report{}, err
	}
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		return observer.Report{}, err
	}
	semantic, err := os.ReadFile(semanticPath)
	if err != nil {
		return observer.Report{}, err
	}
	if err := validateSemanticIR(semantic); err != nil {
		return observer.Report{}, err
	}
	generatedGo, err := os.ReadFile(generatedPath)
	if err != nil {
		return observer.Report{}, err
	}
	evaluator, err := os.ReadFile(evaluatorPath)
	if err != nil {
		return observer.Report{}, err
	}
	denominator, _, err := observer.LoadDenominator(contractPath)
	if err != nil {
		return observer.Report{}, err
	}
	if inventoryRoot == "" {
		inventoryRoot = filepath.Dir(inputPath)
	}
	inventory, err := measureInventory(inventoryRoot)
	if err != nil {
		return observer.Report{}, err
	}
	manifestPath := filepath.Join(outputDir, "observation-manifest.json")
	receiptPath := filepath.Join(outputDir, "observer-receipt.json")
	humanPath := filepath.Join(outputDir, "observer-report.md")
	meta := observer.Meta{
		SourcePath:        sourcePath,
		SourceDigest:      observer.DigestBytes(source),
		SemanticIRPath:    semanticPath,
		SemanticIRDigest:  observer.DigestBytes(semantic),
		GeneratedGoPath:   generatedPath,
		GeneratedGoDigest: observer.DigestBytes(generatedGo),
		EvaluatorPath:     evaluatorPath,
		EvaluatorDigest:   observer.DigestBytes(evaluator),
		ContractPath:      contractPath,
		ContractDigest:    observer.DigestBytes(contract),
		ManifestPath:      filepath.Base(manifestPath),
		ReceiptPath:       receiptPath,
		HumanReportPath:   humanPath,
		Denominator:       denominator,
	}
	report, claims, evidence, transitions := observer.Observe(input, meta, inventory)
	human := observer.HumanReport(report)
	report.AuthorityChain.HumanReport.Digest = observer.DigestBytes([]byte(human))
	manifest := observer.ObservationManifest{
		Schema:          "gooo/semantic-observer/manifest/v1",
		CaseID:          report.CaseID,
		InputDigest:     report.InputDigest,
		ArtifactKinds:   append([]string(nil), observer.ArtifactKinds...),
		ClaimRecords:    len(claims),
		EvidenceRecords: len(evidence),
		Transitions:     len(transitions),
		AuthorityChain:  report.AuthorityChain,
	}
	if err := writeNDJSON(filepath.Join(outputDir, "claims.ndjson"), claims); err != nil {
		return observer.Report{}, err
	}
	if err := writeNDJSON(filepath.Join(outputDir, "evidence.ndjson"), evidence); err != nil {
		return observer.Report{}, err
	}
	if err := writeNDJSON(filepath.Join(outputDir, "transitions.ndjson"), transitions); err != nil {
		return observer.Report{}, err
	}
	if err := writeJSON(filepath.Join(outputDir, "observer-receipt.json"), report); err != nil {
		return observer.Report{}, err
	}
	if err := writeFile(filepath.Join(outputDir, "observer-report.md"), []byte(human)); err != nil {
		return observer.Report{}, err
	}
	receiptBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		return observer.Report{}, err
	}
	manifest.AuthorityChain.Receipt.Digest = observer.DigestBytes(receiptBytes)
	if err := writeJSON(manifestPath, manifest); err != nil {
		return observer.Report{}, err
	}
	if err := verifyArtifactSet(outputDir); err != nil {
		return observer.Report{}, err
	}
	if err := verifyDetachedReceiptBinding(outputDir); err != nil {
		return observer.Report{}, err
	}
	return report, nil
}

func validateSemanticIR(raw []byte) error {
	var ir observer.SemanticIR
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ir); err != nil {
		return err
	}
	if ir.Schema != observer.IRSchema || ir.SourcePath == "" || len(ir.Nodes) != 12 {
		return errors.New("INVALID_SEMANTIC_IR")
	}
	return nil
}

func measureInventory(root string) (observer.InventoryMetrics, error) {
	var metrics observer.InventoryMetrics
	metrics.RootReadmeExcluded = true
	info, err := os.Stat(root)
	if err != nil {
		return metrics, err
	}
	if !info.IsDir() {
		if err := countFile(root, &metrics); err != nil {
			return metrics, err
		}
		return metrics, nil
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			metrics.InputDescendantDirs++
			return nil
		}
		return countFile(path, &metrics)
	})
	return metrics, err
}

func countFile(path string, metrics *observer.InventoryMetrics) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	metrics.InputDescendantFiles++
	if len(data) > 0 {
		metrics.InputPhysicalLines += strings.Count(string(data), "\n")
		if data[len(data)-1] != '\n' {
			metrics.InputPhysicalLines++
		}
	}
	switch filepath.Ext(path) {
	case ".go":
		metrics.InputGoLines += physicalLines(data)
	case ".gooo":
		metrics.InputGoooLines += physicalLines(data)
	}
	return nil
}

func physicalLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func ensureEmptyDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0o755)
		}
		return err
	}
	if len(entries) != 0 {
		return errors.New("OUTPUT_DIRECTORY_NOT_EMPTY")
	}
	return nil
}

func verifyArtifactSet(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return errors.New("OUTPUT_DIRECTORY_CONTAINS_DIRECTORY")
		}
		actual = append(actual, entry.Name())
	}
	sort.Strings(actual)
	expected := append([]string(nil), observer.ArtifactKinds...)
	sort.Strings(expected)
	if !sameStrings(actual, expected) {
		return fmt.Errorf("output artifact set = %v, want %v", actual, expected)
	}
	return nil
}

func verifyDetachedReceiptBinding(outputDir string) error {
	manifest, err := os.ReadFile(filepath.Join(outputDir, "observation-manifest.json"))
	if err != nil {
		return err
	}
	receipt, err := os.ReadFile(filepath.Join(outputDir, "observer-receipt.json"))
	if err != nil {
		return err
	}
	return verifyDetachedReceiptBindingBytes(manifest, receipt)
}

func verifyDetachedReceiptBindingBytes(manifestBytes, receiptBytes []byte) error {
	var manifest observer.ObservationManifest
	manifestDecoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	manifestDecoder.DisallowUnknownFields()
	if err := manifestDecoder.Decode(&manifest); err != nil {
		return err
	}
	var receipt observer.Report
	receiptDecoder := json.NewDecoder(bytes.NewReader(receiptBytes))
	receiptDecoder.DisallowUnknownFields()
	if err := receiptDecoder.Decode(&receipt); err != nil {
		return err
	}
	if receipt.AuthorityChain.Receipt.Digest != "" {
		return errors.New("REFUTED_RECEIPT_SELF_DIGEST_PRESENT")
	}
	if receipt.SelfBinding.Mode != "DETACHED_MANIFEST" || receipt.SelfBinding.ManifestPath == "" {
		return errors.New("REFUTED_RECEIPT_SELF_BINDING_MISSING")
	}
	if manifest.AuthorityChain.Receipt.Digest == "" {
		return errors.New("REFUTED_RECEIPT_DIGEST_UNBOUND")
	}
	if manifest.AuthorityChain.Receipt.Digest != observer.DigestBytes(receiptBytes) {
		return errors.New("REFUTED_RECEIPT_DIGEST_UNBOUND")
	}
	if manifest.CaseID != receipt.CaseID {
		return errors.New("REFUTED_RECEIPT_CASE_BINDING_MISMATCH")
	}
	return nil
}

func verifyReceiptSubstitutionCounterexample(outputDir string) error {
	targetManifest, err := os.ReadFile(filepath.Join(outputDir, "closed-github-run", "observation-manifest.json"))
	if err != nil {
		return err
	}
	substituteReceipt, err := os.ReadFile(filepath.Join(outputDir, "refuted-contradiction", "observer-receipt.json"))
	if err != nil {
		return err
	}
	err = verifyDetachedReceiptBindingBytes(targetManifest, substituteReceipt)
	if err == nil || err.Error() != "REFUTED_RECEIPT_DIGEST_UNBOUND" {
		return fmt.Errorf("expected REFUTED_RECEIPT_DIGEST_UNBOUND, got %v", err)
	}
	return nil
}

func writeNDJSON(path string, values any) error {
	data := reflectSlice(values)
	var builder strings.Builder
	for _, value := range data {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		builder.Write(raw)
		builder.WriteByte('\n')
	}
	return writeFile(path, []byte(builder.String()))
}

func reflectSlice(values any) []any {
	switch typed := values.(type) {
	case []observer.ClaimRecord:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = typed[index]
		}
		return result
	case []observer.Evidence:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = typed[index]
		}
		return result
	case []observer.Transition:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = typed[index]
		}
		return result
	default:
		return nil
	}
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(raw, '\n'))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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

func expectedDecision(caseName string) (string, bool) {
	decisions := map[string]string{
		"closed-github-run":          observer.DecisionClosed,
		"closed-github-artifact":     observer.DecisionClosed,
		"closed-caller-input":        observer.DecisionClosed,
		"unknown-direct-missing":     observer.DecisionUnknown,
		"unknown-dependency-blocked": observer.DecisionUnknown,
		"unknown-stale":              observer.DecisionUnknown,
		"unknown-ambiguous":          observer.DecisionUnknown,
		"refuted-contradiction":      observer.DecisionRefuted,
		"refuted-self-attestation":   observer.DecisionRefuted,
		"refuted-evaluator-digest":   observer.DecisionRefuted,
		"refuted-evidence-digest":    observer.DecisionRefuted,
		"refuted-source-binding":     observer.DecisionRefuted,
	}
	decision, ok := decisions[caseName]
	return decision, ok
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: gooo-observer <compile|observe|conformance|version>")
}
