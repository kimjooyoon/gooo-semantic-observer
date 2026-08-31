#!/usr/bin/env bash
set -euo pipefail

jq -e '
  .schema == "gooo/semantic-observer/development-history/v1" and
  .writer_policy == "single-writer" and
  (.duplicate_writers | length) == 0 and
  .local_go_commands == 0 and
  .cross_project_required_gates == 0
' provenance/development-history-v1.json >/dev/null

jq -e '
  .schema == "gooo/semantic-observer/release-history/v1" and
  .releases[0].tag == "v0.1.0" and
  .releases[0].status == "REFUTED_RECEIPT_DIGEST_UNBOUND" and
  .releases[0].target_commit == "091764a433400dbd0c31a2f48e257ac300a99d4f" and
  .releases[0].tag_object == "8fbce29f46e13b9e489a2ba49012046eddecf25e" and
  .releases[0].asset.digest == "sha256:34fb06f127bb6bffd64f07dc59916205963bffbf051f1e504cb64ea886731034" and
  .releases[0].preservation.delete == false and
  .releases[0].preservation.recreate == false
' provenance/release-history-v1.json >/dev/null

jq -e '
  .schema == "gooo/semantic-observer/denominator/v1" and
  .total == 12 and (.cells | length) == 12
' contracts/semantic-observer-denominator-v1.json >/dev/null

go test ./...

go_files=$(git ls-files '*.go')
gofmt_output=$(gofmt -d ${go_files} || true)
if [[ -n "${gofmt_output}" ]]; then
  printf '%s\n' "${gofmt_output}"
  exit 1
fi

go vet ./...
go build -o "${RUNNER_TEMP:-/tmp}/gooo-observer" ./cmd/gooo-observer
go run ./cmd/gooo-observer compile \
  -source examples/semantic-observer/main.gooo \
  -contract contracts/semantic-observer-denominator-v1.json \
  -output-ir internal/generated/semantic-ir.json \
  -output-go internal/generated/semantic.gooo.go
git diff --exit-code -- internal/generated/semantic-ir.json internal/generated/semantic.gooo.go

go run ./cmd/gooo-observer conformance \
  -fixtures fixtures \
  -output-dir artifacts/conformance

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  cat artifacts/conformance/ci-summary.md >> "${GITHUB_STEP_SUMMARY}"
fi
