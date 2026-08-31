# Semantic observer v1

## State machine

Each input claim is first written to `claims.ndjson` as an original
`UNVERIFIED` record. Evidence is copied to `evidence.ndjson`. The evaluator
then appends transitions to `transitions.ndjson`:

```text
UNVERIFIED -- exact matching evidence --> SUPPORTED
UNVERIFIED -- exact contradiction      --> REFUTED
UNVERIFIED -- no resolvable evidence    --> UNVERIFIED + UNKNOWN record
```

Evidence never deletes or rewrites the original claim. A report can therefore
replay the entire claim history from the three append-only streams.

## Evidence authority

The accepted source kinds are `GITHUB_RUN`, `GITHUB_ARTIFACT`, `GIT_COMMIT`,
and `CALLER_INPUT`. Each source uses an immutable ID/reference shape. Evidence
must carry a valid evaluator digest, evidence digest, observed digest, enum
state, and ordered exact tuple. Digest mismatch and self-attestation are
`REFUTED`. Titles are descriptive only and never participate in resolution.

## Unknowns and precedence

Missing direct evidence is `DIRECT_MISSING`; unavailable dependencies are
`DEPENDENCY_BLOCKED`; stale evidence is `STALE`; an unresolved observation or
tuple is `AMBIGUOUS`. Each is a six-coordinate UNKNOWN record. The final
decision order is `REFUTED > UNKNOWN > CLOSED`.

## Provenance and boundary

The observer receipt binds `.gooo` source, semantic IR, generated Go, evaluator,
and human report by digest. The output directory must be empty before the
observer writes, so the observer never deletes caller data. Repository writes,
local test executions, and cross-project required gates are all reported as
zero. Inventory excludes only a missing root README.
