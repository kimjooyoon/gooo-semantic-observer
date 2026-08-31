# Gooo semantic observer

`gooo-semantic-observer` is a self-contained Go 1.27 observer for the
observe → claim-state transition → counterexample-preserving loop. It reads
immutable GitHub run, artifact, and commit fixtures or caller-owned input. It
does not manufacture evidence from prose, titles, scores, percentages, or
LLM inference.

The authority chain is executable and digest-bound:

```text
examples/semantic-observer/main.gooo
  -> internal/generated/semantic-ir.json
  -> internal/generated/semantic.gooo.go
  -> internal/observer/evaluate.go
  -> six caller-owned output artifacts
```

The evaluator uses only exact IDs, `sha256:` digests, enum values, and ordered
exact tuples. An unsupported claim remains `UNVERIFIED`. A matching exact
evidence tuple appends `SUPPORTED`; a matching contradiction appends
`REFUTED`. The original claim is never removed. The precedence is fixed:
`REFUTED > UNKNOWN > CLOSED`.

## Fixed contract

- Exactly 12 denominator cells and exactly 12 released `.gooo` activities.
- Every cell is bound one-to-one to an activity, metric, path, artifact, and
  evaluator in the generated semantic IR and generated Go.
- Exactly six output artifact kinds:
  `observation-manifest.json`, `claims.ndjson`, `evidence.ndjson`,
  `transitions.ndjson`, `observer-receipt.json`, and `observer-report.md`.
- Every UNKNOWN record carries `stage`, `step`, `reason`, `unknown_class`,
  `next_operation`, and `blocked_by`.
- The runtime boundary records
  `repository_writes=0`, `local_test_executions=0`, and
  `cross_project_required_gates=0`.
- Reports include integer `peak_rss_kib`, `wall_ms`, input descendant
  directories/files/physical lines/Go lines/Gooo lines, and output artifact
  file count. A project-root README is excluded from inventory violations.

## Canonical executable cases

There are exactly 12 fixture cases: CLOSED=3, UNKNOWN=4, and REFUTED=5.
The UNKNOWN cases cover `DIRECT_MISSING`, `DEPENDENCY_BLOCKED`, `STALE`, and
`AMBIGUOUS`. Refuted cases cover explicit contradiction, self-attestation,
evaluator digest mismatch, evidence digest mismatch, and mutable source
binding.

## Usage and verification

`compile` emits semantic IR and generated Go. `observe` writes only to a fresh
caller-owned output directory. `conformance` evaluates all twelve cases and
emits a human CI summary.

Go build, test, formatting, vet, and conformance are intentionally run only in
GitHub Actions. The repository does not require another project branch, pull
request, or mutable ref as a gate.
