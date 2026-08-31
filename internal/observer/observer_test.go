package observer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUnknownRecordHasSixRequiredFields(t *testing.T) {
	report := Report{Unknowns: []Unknown{{
		Stage:         "EVIDENCE",
		Step:          "LOAD_DEPENDENCY",
		Reason:        "EVIDENCE_DEPENDENCY_BLOCKED",
		UnknownClass:  "DEPENDENCY_BLOCKED",
		NextOperation: "PROVIDE_DEPENDENCY_RECEIPT",
		BlockedBy:     []string{"evidence:blocked"},
	}}}
	if err := report.ValidateUnknowns(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCompilesToExactlyTwelveActivities(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	source, err := os.ReadFile(filepath.Join(root, "examples/semantic-observer/main.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	denominator, _, err := LoadDenominator(filepath.Join(root, "contracts/semantic-observer-denominator-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	ir, err := CompileSource("examples/semantic-observer/main.gooo", source, denominator)
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Nodes) != 12 {
		t.Fatalf("activity count = %d", len(ir.Nodes))
	}
}

func TestImmutableReferenceShape(t *testing.T) {
	if validVersion("main") || validVersion("pull/12") || validVersion("v0.1") {
		t.Fatal("mutable or incomplete version accepted")
	}
	if !validVersion("v0.1.0") || !validDigest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("valid immutable identity rejected")
	}
}

func TestDecisionPrecedence(t *testing.T) {
	if got := resolveDecision(false, 0, true); got != DecisionClosed {
		t.Fatalf("closed decision = %s", got)
	}
	if got := resolveDecision(false, 1, true); got != DecisionUnknown {
		t.Fatalf("unknown decision = %s", got)
	}
	if got := resolveDecision(true, 1, true); got != DecisionRefuted {
		t.Fatalf("refuted decision = %s", got)
	}
}

func TestCallerFixtureSupportsExactTuple(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "closed-caller-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if err := validateEvidence(input.Evidence[0], input.Evaluator); err != nil {
		t.Fatal(err)
	}
	state, transition, unknown := observeClaim(input.Claims[0], input.Evidence)
	if state != StateSupported || transition == nil || unknown != nil {
		t.Fatalf("caller fixture state=%s transition=%#v unknown=%#v", state, transition, unknown)
	}
}

func TestObserveClosedCallerFixture(t *testing.T) {
	root := filepath.Join("..", "..")
	input, err := os.ReadFile(filepath.Join(root, "fixtures", "closed-caller-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) []byte {
		data, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		return data
	}
	source := read("examples/semantic-observer/main.gooo")
	semantic := read("internal/generated/semantic-ir.json")
	generatedGo := read("internal/generated/semantic.gooo.go")
	evaluator := read("internal/observer/evaluate.go")
	contract := read("contracts/semantic-observer-denominator-v1.json")
	denominator, _, err := LoadDenominator(filepath.Join(root, "contracts", "semantic-observer-denominator-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, _, _, _ := Observe(input, Meta{
		SourcePath:        "examples/semantic-observer/main.gooo",
		SourceDigest:      DigestBytes(source),
		SemanticIRPath:    "internal/generated/semantic-ir.json",
		SemanticIRDigest:  DigestBytes(semantic),
		GeneratedGoPath:   "internal/generated/semantic.gooo.go",
		GeneratedGoDigest: DigestBytes(generatedGo),
		EvaluatorPath:     "internal/observer/evaluate.go",
		EvaluatorDigest:   DigestBytes(evaluator),
		ContractPath:      "contracts/semantic-observer-denominator-v1.json",
		ContractDigest:    DigestBytes(contract),
		Denominator:       denominator,
	}, InventoryMetrics{})
	if report.Decision != DecisionClosed {
		t.Fatalf("caller observe decision=%s reason=%s claims=%#v unknowns=%#v transitions=%#v", report.Decision, report.Reason, report.Claims, report.Unknowns, report.Transitions)
	}
}
