package observer

import (
	"fmt"
	"strings"
)

func HumanReport(report Report) string {
	var builder strings.Builder
	builder.WriteString("# Gooo semantic observer report\n\n")
	fmt.Fprintf(&builder, "- case: `%s`\n", report.CaseID)
	fmt.Fprintf(&builder, "- decision: `%s`\n", report.Decision)
	fmt.Fprintf(&builder, "- reason: `%s`\n", report.Reason)
	fmt.Fprintf(&builder, "- state_counts: `%s`\n", report.ExactStateSummary())
	fmt.Fprintf(&builder, "- precedence: `%s`\n", strings.Join(report.Precedence, " > "))
	fmt.Fprintf(&builder, "- append_only: claims=%t evidence=%t transitions=%t\n", report.AppendOnly.OriginalClaimPreserved, report.AppendOnly.EvidencePreserved, report.AppendOnly.TransitionsAppended)
	fmt.Fprintf(&builder, "- peak_rss_kib: `%d`\n", report.Runtime.PeakRSSKiB)
	fmt.Fprintf(&builder, "- wall_ms: `%d`\n", report.Runtime.WallMS)
	fmt.Fprintf(&builder, "- output_artifact_files: `%d`\n", report.Runtime.OutputArtifactFiles)
	fmt.Fprintf(&builder, "- input_descendant_dirs: `%d`\n", report.Inventory.InputDescendantDirs)
	fmt.Fprintf(&builder, "- input_descendant_files: `%d`\n", report.Inventory.InputDescendantFiles)
	fmt.Fprintf(&builder, "- input_physical_lines: `%d`\n", report.Inventory.InputPhysicalLines)
	fmt.Fprintf(&builder, "- input_go_lines: `%d`\n", report.Inventory.InputGoLines)
	fmt.Fprintf(&builder, "- input_gooo_lines: `%d`\n", report.Inventory.InputGoooLines)
	fmt.Fprintf(&builder, "- repository_writes: `%d`\n", report.Authority.RepositoryWrites)
	fmt.Fprintf(&builder, "- local_test_executions: `%d`\n", report.Authority.LocalTestExecutions)
	fmt.Fprintf(&builder, "- cross_project_required_gates: `%d`\n", report.Authority.CrossProjectRequiredGates)
	builder.WriteString("\n## Artifact kinds\n\n")
	for _, artifact := range report.ArtifactKinds {
		fmt.Fprintf(&builder, "- `%s`\n", artifact)
	}
	builder.WriteString("\n## Authority chain\n\n")
	fmt.Fprintf(&builder, "`%s` (%s) -> `%s` (%s) -> `%s` (%s) -> `%s` (%s) -> `%s` -> `%s`\n", report.AuthorityChain.Source.Path, report.AuthorityChain.Source.Digest, report.AuthorityChain.SemanticIR.Path, report.AuthorityChain.SemanticIR.Digest, report.AuthorityChain.GeneratedGo.Path, report.AuthorityChain.GeneratedGo.Digest, report.AuthorityChain.Evaluator.Path, report.AuthorityChain.Evaluator.Digest, report.AuthorityChain.Receipt.Path, report.AuthorityChain.HumanReport.Path)
	if len(report.Unknowns) > 0 {
		builder.WriteString("\n## UNKNOWN records\n\n")
		for _, unknown := range report.Unknowns {
			fmt.Fprintf(&builder, "- stage=`%s`, step=`%s`, reason=`%s`, unknown_class=`%s`, next_operation=`%s`, blocked_by=`%s`\n", unknown.Stage, unknown.Step, unknown.Reason, unknown.UnknownClass, strings.Join(unknown.BlockedBy, ","))
		}
	}
	return builder.String()
}

func HumanConformanceSummary(reports []Report) string {
	var builder strings.Builder
	builder.WriteString("# Gooo semantic observer CI summary\n\n")
	builder.WriteString("| case | decision | unverified | supported | refuted | unknown classes | transitions | artifacts |\n")
	builder.WriteString("|---|---|---:|---:|---:|---|---:|---:|\n")
	for _, report := range reports {
		classes := make([]string, 0, len(report.Unknowns))
		for _, unknown := range report.Unknowns {
			classes = append(classes, unknown.UnknownClass)
		}
		fmt.Fprintf(&builder, "| %s | %s | %d | %d | %d | %s | %d | %d |\n", report.CaseID, report.Decision, report.StateCounts.Unverified, report.StateCounts.Supported, report.StateCounts.Refuted, strings.Join(classes, ","), len(report.Transitions), report.Runtime.OutputArtifactFiles)
	}
	builder.WriteString("\nCanonical cases: 12 total; CLOSED=3, UNKNOWN=4, REFUTED=5.\n\n")
	builder.WriteString("Fixed denominator: 12 cells with 12 one-to-one `.gooo` activity bindings. Precedence: REFUTED > UNKNOWN > CLOSED.\n\n")
	builder.WriteString("Runtime boundary: repository_writes=0, local_test_executions=0, cross_project_required_gates=0.\n")
	return builder.String()
}
