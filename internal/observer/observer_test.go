package observer

import (
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
