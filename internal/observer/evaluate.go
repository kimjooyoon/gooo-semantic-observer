package observer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-semantic-observer/internal/generated"
)

func Observe(raw []byte, meta Meta, inventory InventoryMetrics) (Report, []ClaimRecord, []Evidence, []Transition) {
	started := time.Now()
	report := baseReport(meta, DigestBytes(raw), inventory)
	var input Input
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return finishReport(report, started, "MALFORMED_INPUT", false), nil, nil, nil
	}
	report.CaseID = input.CaseID
	report.Authority = authorityFor(input.Authority)

	hasRefuted := false
	refutedReason := ""
	addRefutation := func(reason string) {
		if !hasRefuted {
			refutedReason = reason
		}
		hasRefuted = true
	}
	if input.Schema != InputSchema || input.CaseID == "" {
		addRefutation("INVALID_INPUT_CONTRACT")
	}
	if err := validateMeta(meta); err != nil {
		addRefutation(err.Error())
	}
	if input.Authority.RepositoryWrites != 0 || input.Authority.LocalTestExecutions != 0 || input.Authority.CrossProjectRequiredGates != 0 {
		addRefutation("AUTHORITY_ESCALATION_REFUTED")
	}
	if err := validateEvaluator(input.Evaluator); err != nil {
		addRefutation(err.Error())
	}

	claims := make([]ClaimRecord, 0, len(input.Claims))
	evidenceRecords := append([]Evidence(nil), input.Evidence...)
	claimByID := map[string]Claim{}
	for index, claim := range input.Claims {
		claims = append(claims, ClaimRecord{
			RecordType:      "CLAIM_ORIGINAL",
			Sequence:        index + 1,
			ClaimID:         claim.ID,
			SubjectID:       claim.SubjectID,
			AssertionDigest: claim.AssertionDigest,
			ExactTuple:      append([]string(nil), claim.ExactTuple...),
			State:           StateUnverified,
		})
		if claim.ID == "" || claimByID[claim.ID].ID != "" {
			addRefutation("INVALID_OR_DUPLICATE_CLAIM")
			continue
		}
		claimByID[claim.ID] = claim
		if err := validateClaim(claim, input.Evaluator); err != nil {
			addRefutation(err.Error())
		}
	}
	if len(input.Claims) == 0 {
		addRefutation("CLAIMS_REQUIRED")
	}

	evidenceByClaim := map[string][]Evidence{}
	for _, evidence := range input.Evidence {
		if err := validateEvidence(evidence, input.Evaluator); err != nil {
			addRefutation(err.Error())
		}
		if evidence.ClaimID == "" || claimByID[evidence.ClaimID].ID == "" {
			addRefutation("EVIDENCE_CLAIM_BINDING_MISMATCH")
			continue
		}
		evidenceByClaim[evidence.ClaimID] = append(evidenceByClaim[evidence.ClaimID], evidence)
	}

	authorityRefuted := hasRefuted
	authorityReason := refutedReason
	transitions := []Transition{}
	report.Claims = make([]ClaimState, 0, len(input.Claims))
	unknowns := []Unknown{}
	for _, claim := range input.Claims {
		state := StateUnverified
		transitionCount := 0
		claimEvidence := evidenceByClaim[claim.ID]
		if claim.SelfAttestation != nil && (claim.SelfAttestation.EvaluatorID == claim.EvaluatorID || claim.SelfAttestation.ReleaseDigest == claim.EvaluatorDigest) {
			state = StateRefuted
			transitionCount++
			transitions = append(transitions, Transition{Sequence: len(transitions) + 1, ClaimID: claim.ID, From: StateUnverified, To: StateRefuted, Reason: "SELF_ATTESTATION_REFUTED"})
			addRefutation("SELF_ATTESTATION_REFUTED")
		} else if hasGlobalRefutation(authorityRefuted, authorityReason) {
			state = StateRefuted
			transitionCount++
			transitions = append(transitions, Transition{Sequence: len(transitions) + 1, ClaimID: claim.ID, From: StateUnverified, To: StateRefuted, Reason: authorityReason})
		} else {
			state, transition, unknown := observeClaim(claim, claimEvidence)
			if transition != nil {
				transition.Sequence = len(transitions) + 1
				transitions = append(transitions, *transition)
				transitionCount++
			}
			if state == StateRefuted && transition != nil {
				addRefutation(transition.Reason)
			}
			if unknown != nil {
				addUnknown(&unknowns, *unknown)
			}
		}
		report.Claims = append(report.Claims, ClaimState{ClaimID: claim.ID, OriginalState: StateUnverified, CurrentState: state, TransitionCount: transitionCount})
	}

	report.Decision = resolveDecision(hasRefuted, len(unknowns), allSupported(report.Claims))
	if report.Decision == DecisionRefuted {
		report.Reason = refutedReason
	} else if report.Decision == DecisionUnknown {
		if len(unknowns) > 0 {
			report.Reason = unknowns[0].Reason
		} else {
			report.Reason = "CLAIMS_REMAIN_UNVERIFIED"
		}
	} else {
		report.Reason = "ALL_CLAIMS_SUPPORTED"
	}
	report.Unknowns = unknowns
	report.Transitions = transitions
	report.StateCounts = countStates(report.Claims)
	report.AppendOnly = AppendOnly{OriginalClaimPreserved: len(claims) == len(input.Claims), EvidencePreserved: len(evidenceRecords) == len(input.Evidence), TransitionsAppended: true}
	return finishReport(report, started, report.Reason, hasRefuted), claims, evidenceRecords, transitions
}

func resolveDecision(hasRefuted bool, unknownCount int, allClaimsSupported bool) string {
	if hasRefuted {
		return DecisionRefuted
	}
	if unknownCount > 0 || !allClaimsSupported {
		return DecisionUnknown
	}
	return DecisionClosed
}

func baseReport(meta Meta, inputDigest string, inventory InventoryMetrics) Report {
	return Report{
		Schema:        Schema,
		InputDigest:   inputDigest,
		Decision:      DecisionRefuted,
		Precedence:    append([]string(nil), Precedence...),
		Unknowns:      []Unknown{},
		Transitions:   []Transition{},
		ArtifactKinds: append([]string(nil), ArtifactKinds...),
		Inventory:     inventory,
		Runtime:       RuntimeMetrics{OutputArtifactFiles: len(ArtifactKinds)},
		AuthorityChain: AuthorityChain{
			Source:      ArtifactLink{Path: meta.SourcePath, Digest: meta.SourceDigest},
			SemanticIR:  ArtifactLink{Path: meta.SemanticIRPath, Digest: meta.SemanticIRDigest},
			GeneratedGo: ArtifactLink{Path: meta.GeneratedGoPath, Digest: meta.GeneratedGoDigest},
			Evaluator:   ArtifactLink{Path: meta.EvaluatorPath, Digest: meta.EvaluatorDigest},
			Receipt:     ArtifactLink{Path: meta.ReceiptPath},
			HumanReport: ArtifactLink{Path: meta.HumanReportPath},
		},
	}
}

func finishReport(report Report, started time.Time, reason string, hasRefuted bool) Report {
	if report.Reason == "" {
		report.Reason = reason
	}
	if hasRefuted {
		report.Decision = DecisionRefuted
	}
	report.Runtime.WallMS = int(time.Since(started).Milliseconds())
	if report.Runtime.WallMS < 1 {
		report.Runtime.WallMS = 1
	}
	report.Runtime.PeakRSSKiB = peakRSSKiB()
	return report
}

func authorityFor(input AuthorityInput) AuthorityReport {
	return AuthorityReport{
		RepositoryWrites:                   0,
		LocalTestExecutions:                0,
		CrossProjectRequiredGates:          0,
		RequestedRepositoryWrites:          input.RepositoryWrites,
		RequestedLocalTestExecutions:       input.LocalTestExecutions,
		RequestedCrossProjectRequiredGates: input.CrossProjectRequiredGates,
		ReadOnly:                           input.RepositoryWrites == 0 && input.LocalTestExecutions == 0 && input.CrossProjectRequiredGates == 0,
	}
}

func validateMeta(meta Meta) error {
	if meta.SourcePath == "" || !validDigest(meta.SourceDigest) || meta.SemanticIRPath == "" || !validDigest(meta.SemanticIRDigest) || meta.GeneratedGoPath == "" || !validDigest(meta.GeneratedGoDigest) || meta.EvaluatorPath == "" || !validDigest(meta.EvaluatorDigest) || meta.ContractPath == "" || !validDigest(meta.ContractDigest) {
		return errors.New("INCOMPLETE_AUTHORITY_CHAIN")
	}
	if generated.SourcePath != meta.SourcePath || generated.SourceDigest != meta.SourceDigest || generated.SemanticIRPath != meta.SemanticIRPath || generated.SemanticIRDigest != meta.SemanticIRDigest || generated.ContractPath != meta.ContractPath || generated.ContractDigest != meta.ContractDigest || generated.ActivityCount != 12 || len(generated.Activities) != 12 {
		return errors.New("GENERATED_AUTHORITY_CHAIN_MISMATCH")
	}
	for index, cell := range meta.Denominator.Cells {
		activity := generated.Activities[index]
		if activity.ID != activityIDForCell(cell.ID) || activity.Activity != cell.Activity || activity.MetricID != cell.MetricID || activity.MetricPath != cell.MetricPath || activity.ProofChoice != cell.ProofChoice || activity.IndicatorClass != cell.IndicatorClass || activity.Artifact != cell.Artifact || activity.Evaluator != cell.Evaluator {
			return errors.New("GENERATED_ACTIVITY_BINDING_MISMATCH")
		}
	}
	return nil
}

func validateEvaluator(reference EvaluatorReference) error {
	if reference.ID == "" || !validVersion(reference.Version) || reference.ReleaseTag != reference.Version || !validDigest(reference.ReleaseDigest) || !validDigest(reference.ObservedReleaseDigest) {
		return errors.New("INVALID_IMMUTABLE_EVALUATOR_REFERENCE")
	}
	if reference.ReleaseDigest != reference.ObservedReleaseDigest {
		return errors.New("EVALUATOR_DIGEST_MISMATCH")
	}
	return nil
}

func validateClaim(claim Claim, evaluator EvaluatorReference) error {
	if claim.SubjectID == "" || !validDigest(claim.AssertionDigest) || len(claim.ExactTuple) == 0 || claim.EvaluatorID == "" || !validDigest(claim.EvaluatorDigest) {
		return errors.New("INVALID_CLAIM_EXACT_BINDING")
	}
	if claim.EvaluatorID == evaluator.ID || claim.EvaluatorDigest == evaluator.ReleaseDigest {
		return errors.New("CLAIM_EVALUATOR_IDENTITY_COLLISION")
	}
	if claim.SelfAttestation != nil && claim.SelfAttestation.Decision == "" {
		return errors.New("INVALID_SELF_ATTESTATION")
	}
	return nil
}

func validateEvidence(evidence Evidence, evaluator EvaluatorReference) error {
	if evidence.ID == "" || evidence.ClaimID == "" || !validSource(evidence.SourceKind, evidence.SourceRef) || !validDigest(evidence.EvaluatorDigest) || !validDigest(evidence.Digest) || !validDigest(evidence.ObservedDigest) || evidence.Availability == "" || evidence.Freshness == "" || evidence.Observation == "" {
		return errors.New("INVALID_EVIDENCE_EXACT_BINDING")
	}
	if evidence.EvaluatorDigest != evaluator.ReleaseDigest {
		return errors.New("EVALUATOR_EVIDENCE_DIGEST_MISMATCH")
	}
	if evidence.Digest != evidence.ObservedDigest {
		return errors.New("EVIDENCE_DIGEST_MISMATCH")
	}
	if evidence.Availability != "AVAILABLE" && evidence.Availability != "MISSING" && evidence.Availability != "BLOCKED" {
		return errors.New("INVALID_EVIDENCE_AVAILABILITY")
	}
	if evidence.Freshness != "CURRENT" && evidence.Freshness != "STALE" && evidence.Freshness != "UNKNOWN" {
		return errors.New("INVALID_EVIDENCE_FRESHNESS")
	}
	if evidence.Observation != "SUPPORTS" && evidence.Observation != "CONTRADICTS" && evidence.Observation != "UNKNOWN" {
		return errors.New("INVALID_EVIDENCE_OBSERVATION")
	}
	return nil
}

func observeClaim(claim Claim, evidence []Evidence) (string, *Transition, *Unknown) {
	if len(evidence) == 0 {
		return StateUnverified, nil, unknownRecord("OBSERVATION", "LOAD_CLAIM_EVIDENCE", "CLAIM_EVIDENCE_MISSING", "DIRECT_MISSING", "PROVIDE_IMMUTABLE_EVIDENCE", "claim:"+claim.ID)
	}
	var blockedEvidence, staleEvidence, ambiguousEvidence *Evidence
	for _, item := range evidence {
		if item.Availability == "BLOCKED" {
			if blockedEvidence == nil {
				candidate := item
				blockedEvidence = &candidate
			}
			continue
		}
		if item.Freshness == "STALE" {
			if staleEvidence == nil {
				candidate := item
				staleEvidence = &candidate
			}
			continue
		}
		if item.Observation == "UNKNOWN" || item.Availability == "MISSING" || item.Freshness == "UNKNOWN" {
			if ambiguousEvidence == nil {
				candidate := item
				ambiguousEvidence = &candidate
			}
			continue
		}
		if !equalTuple(item.ExactTuple, claim.ExactTuple) {
			continue
		}
		if item.Observation == "CONTRADICTS" {
			return StateRefuted, &Transition{ClaimID: claim.ID, From: StateUnverified, To: StateRefuted, EvidenceIDs: []string{item.ID}, Reason: "EXPLICIT_CONTRADICTION"}, nil
		}
		if item.Observation == "SUPPORTS" {
			return StateSupported, &Transition{ClaimID: claim.ID, From: StateUnverified, To: StateSupported, EvidenceIDs: []string{item.ID}, Reason: "EXACT_EVIDENCE_MATCH"}, nil
		}
	}
	if blockedEvidence != nil {
		return StateUnverified, nil, unknownRecord("EVIDENCE", "LOAD_DEPENDENCY", "EVIDENCE_DEPENDENCY_BLOCKED", "DEPENDENCY_BLOCKED", "PROVIDE_DEPENDENCY_RECEIPT", "evidence:"+blockedEvidence.ID)
	}
	if staleEvidence != nil {
		return StateUnverified, nil, unknownRecord("EVIDENCE", "VERIFY_FRESHNESS", "EVIDENCE_STALE", "STALE", "OBSERVE_CURRENT_EVIDENCE", "evidence:"+staleEvidence.ID)
	}
	if ambiguousEvidence != nil {
		return StateUnverified, nil, unknownRecord("EVIDENCE", "RESOLVE_OBSERVATION", "EVIDENCE_OBSERVATION_UNKNOWN", "AMBIGUOUS", "PROVIDE_EXACT_OBSERVATION", "evidence:"+ambiguousEvidence.ID)
	}
	return StateUnverified, nil, unknownRecord("EVIDENCE", "COMPARE_EXACT_TUPLE", "EVIDENCE_TUPLE_AMBIGUOUS", "AMBIGUOUS", "PROVIDE_MATCHING_EXACT_TUPLE", "claim:"+claim.ID)
}

func unknownRecord(stage, step, reason, class, next, blocked string) *Unknown {
	return &Unknown{Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: []string{blocked}}
}

func addUnknown(records *[]Unknown, unknown Unknown) {
	for _, existing := range *records {
		if existing.Reason == unknown.Reason && existing.BlockedBy[0] == unknown.BlockedBy[0] {
			return
		}
	}
	*records = append(*records, unknown)
}

func allSupported(claims []ClaimState) bool {
	if len(claims) == 0 {
		return false
	}
	for _, claim := range claims {
		if claim.CurrentState != StateSupported {
			return false
		}
	}
	return true
}

func countStates(claims []ClaimState) StateCounts {
	counts := StateCounts{}
	for _, claim := range claims {
		switch claim.CurrentState {
		case StateUnverified:
			counts.Unverified++
		case StateSupported:
			counts.Supported++
		case StateRefuted:
			counts.Refuted++
		}
	}
	return counts
}

func hasGlobalRefutation(hasRefuted bool, reason string) bool {
	return hasRefuted && reason != ""
}

func equalTuple(left, right []string) bool {
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

func validSource(kind, reference string) bool {
	switch kind {
	case "GITHUB_RUN":
		return numericReference(reference, "run:")
	case "GITHUB_ARTIFACT":
		return numericReference(reference, "artifact:")
	case "GIT_COMMIT":
		return len(reference) == len("commit:")+40 && strings.HasPrefix(reference, "commit:") && isHex(reference[len("commit:"):])
	case "CALLER_INPUT":
		return strings.HasPrefix(reference, "caller:") && validDigest(strings.TrimPrefix(reference, "caller:"))
	default:
		return false
	}
}

func numericReference(reference, prefix string) bool {
	value := strings.TrimPrefix(reference, prefix)
	if value == reference || value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func validVersion(value string) bool {
	if len(value) < 6 || value[0] != 'v' {
		return false
	}
	parts := strings.Split(value[1:], ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || !numeric(part) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	return len(value) == len("sha256:")+64 && strings.HasPrefix(value, "sha256:") && isHex(strings.TrimPrefix(value, "sha256:"))
}

func numeric(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != ""
}

func isHex(value string) bool {
	if value == "" {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func peakRSSKiB() int {
	status, err := os.ReadFile("/proc/self/status")
	if err == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "VmHWM:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					value, parseErr := strconv.Atoi(fields[1])
					if parseErr == nil {
						return value
					}
				}
			}
		}
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return int(memory.Sys / 1024)
}

func (r Report) ValidateUnknowns() error {
	for _, unknown := range r.Unknowns {
		if unknown.Stage == "" || unknown.Step == "" || unknown.Reason == "" || unknown.UnknownClass == "" || unknown.NextOperation == "" || len(unknown.BlockedBy) == 0 {
			return errors.New("UNKNOWN_RECORD_MISSING_REQUIRED_FIELD")
		}
	}
	return nil
}

func (r Report) ExactStateSummary() string {
	return fmt.Sprintf("unverified=%d supported=%d refuted=%d", r.StateCounts.Unverified, r.StateCounts.Supported, r.StateCounts.Refuted)
}
