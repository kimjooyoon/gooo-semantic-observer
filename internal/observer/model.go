package observer

import "encoding/json"

const (
	Schema          = "gooo/semantic-observer/receipt/v1"
	InputSchema     = "gooo/semantic-observer/input/v1"
	IRSchema        = "gooo/semantic-observer/ir/v1"
	ContractSchema  = "gooo/semantic-observer/denominator/v1"
	StateUnverified = "UNVERIFIED"
	StateSupported  = "SUPPORTED"
	StateRefuted    = "REFUTED"
	DecisionClosed  = "CLOSED"
	DecisionUnknown = "UNKNOWN"
	DecisionRefuted = "REFUTED"
)

var Precedence = []string{DecisionRefuted, DecisionUnknown, DecisionClosed}

var ArtifactKinds = []string{
	"observation-manifest.json",
	"claims.ndjson",
	"evidence.ndjson",
	"transitions.ndjson",
	"observer-receipt.json",
	"observer-report.md",
}

type Input struct {
	Schema    string             `json:"schema"`
	CaseID    string             `json:"case_id"`
	Evaluator EvaluatorReference `json:"evaluator"`
	Claims    []Claim            `json:"claims"`
	Evidence  []Evidence         `json:"evidence"`
	Authority AuthorityInput      `json:"authority"`
}

type EvaluatorReference struct {
	ID                    string `json:"id"`
	Version               string `json:"version"`
	ReleaseTag            string `json:"release_tag"`
	ReleaseDigest         string `json:"release_digest"`
	ObservedReleaseDigest string `json:"observed_release_digest"`
}

type Claim struct {
	ID              string           `json:"id"`
	SubjectID       string           `json:"subject_id"`
	Title           string           `json:"title"`
	AssertionDigest string           `json:"assertion_digest"`
	ExactTuple      []string         `json:"exact_tuple"`
	EvaluatorID     string           `json:"evaluator_id"`
	EvaluatorDigest string           `json:"evaluator_digest"`
	SelfAttestation *SelfAttestation `json:"self_attestation"`
}

type SelfAttestation struct {
	EvaluatorID   string `json:"evaluator_id"`
	ReleaseDigest string `json:"release_digest"`
	Decision      string `json:"decision"`
}

type Evidence struct {
	ID              string   `json:"id"`
	ClaimID         string   `json:"claim_id"`
	SourceKind      string   `json:"source_kind"`
	SourceRef       string   `json:"source_ref"`
	EvaluatorDigest string   `json:"evaluator_digest"`
	Digest          string   `json:"digest"`
	ObservedDigest  string   `json:"observed_digest"`
	Observation     string   `json:"observation"`
	Availability    string   `json:"availability"`
	Freshness       string   `json:"freshness"`
	ExactTuple      []string `json:"exact_tuple"`
}

type AuthorityInput struct {
	RepositoryWrites          int `json:"repository_writes"`
	LocalTestExecutions       int `json:"local_test_executions"`
	CrossProjectRequiredGates int `json:"cross_project_required_gates"`
}

type AuthorityReport struct {
	RepositoryWrites                   int  `json:"repository_writes"`
	LocalTestExecutions                int  `json:"local_test_executions"`
	CrossProjectRequiredGates          int  `json:"cross_project_required_gates"`
	RequestedRepositoryWrites          int  `json:"requested_repository_writes"`
	RequestedLocalTestExecutions       int  `json:"requested_local_test_executions"`
	RequestedCrossProjectRequiredGates int  `json:"requested_cross_project_required_gates"`
	ReadOnly                           bool `json:"read_only"`
}

type SemanticIR struct {
	Schema       string       `json:"schema"`
	SourcePath   string       `json:"source_path"`
	SourceDigest string       `json:"source_digest"`
	Nodes        []ActivityIR `json:"nodes"`
}

type ActivityIR struct {
	ID             string `json:"id"`
	Activity       string `json:"activity"`
	Name           string `json:"name"`
	ProofChoice    string `json:"proof_choice"`
	IndicatorClass string `json:"indicator_class"`
	MetricID       string `json:"metric_id"`
	MetricPath     string `json:"metric_path"`
	Artifact       string `json:"artifact"`
	Evaluator      string `json:"evaluator"`
	SourceLine     int    `json:"source_line"`
}

type Denominator struct {
	Schema           string            `json:"schema"`
	DenominatorID    string            `json:"denominator_id"`
	CandidateID      string            `json:"candidate_id"`
	Total            int               `json:"total"`
	Proofs           []Balance         `json:"proofs"`
	IndicatorClasses []Balance         `json:"indicator_classes"`
	Cells            []DenominatorCell `json:"cells"`
}

type Balance struct {
	Choice string `json:"choice,omitempty"`
	Class  string `json:"class,omitempty"`
	Total  int    `json:"total"`
}

type DenominatorCell struct {
	Ordinal        int    `json:"ordinal"`
	ID             string `json:"id"`
	Activity       string `json:"activity"`
	ProofChoice    string `json:"proof_choice"`
	IndicatorClass string `json:"indicator_class"`
	MetricID       string `json:"metric_id"`
	MetricPath     string `json:"metric_path"`
	Artifact       string `json:"artifact"`
	Evaluator      string `json:"evaluator"`
}

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type ClaimRecord struct {
	RecordType      string   `json:"record_type"`
	Sequence        int      `json:"sequence"`
	ClaimID         string   `json:"claim_id"`
	SubjectID       string   `json:"subject_id"`
	AssertionDigest string   `json:"assertion_digest"`
	ExactTuple      []string `json:"exact_tuple"`
	State           string   `json:"state"`
}

type Transition struct {
	Sequence    int      `json:"sequence"`
	ClaimID     string   `json:"claim_id"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	EvidenceIDs []string `json:"evidence_ids"`
	Reason      string   `json:"reason"`
}

type ClaimState struct {
	ClaimID         string `json:"claim_id"`
	OriginalState   string `json:"original_state"`
	CurrentState    string `json:"current_state"`
	TransitionCount int    `json:"transition_count"`
}

type StateCounts struct {
	Unverified int `json:"unverified"`
	Supported  int `json:"supported"`
	Refuted    int `json:"refuted"`
}

type RuntimeMetrics struct {
	PeakRSSKiB          int `json:"peak_rss_kib"`
	WallMS              int `json:"wall_ms"`
	OutputArtifactFiles int `json:"output_artifact_files"`
}

type InventoryMetrics struct {
	InputDescendantDirs  int      `json:"input_descendant_dirs"`
	InputDescendantFiles int      `json:"input_descendant_files"`
	InputPhysicalLines   int      `json:"input_physical_lines"`
	InputGoLines         int      `json:"input_go_lines"`
	InputGoooLines       int      `json:"input_gooo_lines"`
	RootReadmeExcluded   bool     `json:"root_readme_excluded"`
	Violations           []string `json:"violations"`
}

type ArtifactLink struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type AuthorityChain struct {
	Source      ArtifactLink `json:"source"`
	SemanticIR  ArtifactLink `json:"semantic_ir"`
	GeneratedGo ArtifactLink `json:"generated_go"`
	Evaluator   ArtifactLink `json:"evaluator"`
	Receipt     ArtifactLink `json:"observer_receipt"`
	HumanReport ArtifactLink `json:"human_report"`
}

type AppendOnly struct {
	OriginalClaimPreserved bool `json:"original_claim_preserved"`
	EvidencePreserved      bool `json:"evidence_preserved"`
	TransitionsAppended    bool `json:"transitions_appended"`
}

type ObservationManifest struct {
	Schema          string         `json:"schema"`
	CaseID          string         `json:"case_id"`
	InputDigest     string         `json:"input_digest"`
	ArtifactKinds   []string       `json:"artifact_kinds"`
	ClaimRecords    int            `json:"claim_records"`
	EvidenceRecords int            `json:"evidence_records"`
	Transitions     int            `json:"transitions"`
	AuthorityChain  AuthorityChain `json:"authority_chain"`
}

type Meta struct {
	SourcePath        string
	SourceDigest      string
	SemanticIRPath    string
	SemanticIRDigest  string
	GeneratedGoPath   string
	GeneratedGoDigest string
	EvaluatorPath     string
	EvaluatorDigest   string
	ContractPath      string
	ContractDigest    string
	ReceiptPath       string
	HumanReportPath   string
	Denominator       Denominator
}

type Report struct {
	Schema         string           `json:"schema"`
	CaseID         string           `json:"case_id"`
	InputDigest    string           `json:"input_digest"`
	Decision       string           `json:"decision"`
	Reason         string           `json:"reason"`
	Precedence     []string         `json:"precedence"`
	Claims         []ClaimState     `json:"claims"`
	StateCounts    StateCounts      `json:"state_counts"`
	Unknowns       []Unknown        `json:"unknowns"`
	Transitions    []Transition     `json:"transitions"`
	ArtifactKinds  []string         `json:"artifact_kinds"`
	Runtime        RuntimeMetrics   `json:"runtime"`
	Inventory      InventoryMetrics `json:"inventory"`
	Authority      AuthorityReport  `json:"authority"`
	AuthorityChain AuthorityChain   `json:"authority_chain"`
	AppendOnly     AppendOnly       `json:"append_only"`
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
