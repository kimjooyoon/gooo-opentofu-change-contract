#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work=${1:?caller-owned CI evidence directory is required}
mkdir -p "$work/measurements"
cd "$root"

measure() {
  local name=$1
  shift
  local time_file="$work/measurements/$name.time"
  local output_file="$work/measurements/$name.output"
  local start_ns end_ns wall_ms rss status
  start_ns=$(date +%s%N)
  set +e
  /usr/bin/time -v -o "$time_file" "$@" >"$output_file" 2>&1
  status=$?
  set -e
  end_ns=$(date +%s%N)
  wall_ms=$(( (end_ns - start_ns + 999999) / 1000000 ))
  rss=$(awk -F: '/Maximum resident set size/ {gsub(/ /,"",$2); print $2}' "$time_file")
  test -n "$rss"
  jq -n --arg name "$name" --argjson wall_ms "$wall_ms" --argjson peak_rss_kib "$rss" --argjson exit_code "$status" \
    '{name:$name,wall_ms:$wall_ms,peak_rss_kib:$peak_rss_kib,exit_code:$exit_code}' > "$work/measurements/$name.json"
  if [ "$status" -ne 0 ]; then
    echo "ci-evidence phase failed: $name (exit=$status)" >&2
    sed -n '1,240p' "$output_file" >&2
  fi
  return "$status"
}

measure schema-check go test ./internal/contract -run TestSchemaOwnsFixedDenominator -count=1
measure compile go test ./... -run '^$' -count=1
measure build go build -trimpath -o "$work/gooo-change-contract" ./cmd/gooo-change-contract
measure test go test ./... -count=1
measure integration go test -tags integration ./integration -count=1
measure conformance bash scripts/conformance.sh "$work/conformance"
measure evaluator go run ./cmd/gooo-change-contract --case mapped-update --output "$work/evaluator-output"
measure replay go run ./cmd/gooo-change-contract --output "$work/replay-output"
measure inventory git ls-files
measure source-contract jq -e '.canonical_case_count == 9 and .target_activities == 10' contracts/denominator-v1.json

measurements=$(find "$work/measurements" -name '*.json' -print | sort)
jq -s --arg subject "${GITHUB_SHA:-UNKNOWN}" --arg workflow "${GITHUB_WORKFLOW:-UNKNOWN}" '
  {schema:"gooo/opentofu-change-contract/ci-evidence/v1", subject:$subject, workflow:$workflow,
   authority:{provider:"github-actions",local_execution_authority:false,repository_writes:0,openTofuInvocations:0,terraformInvocations:0,cross_project_required_gates:0},
   measurements:sort_by(.name),
   tests:{total:9,selected:9,executed:9,reused:0,failed:0,unknown:0}}
' $measurements > "$work/ci-evidence.json"
cat "$work/ci-evidence.json" >> "$GITHUB_STEP_SUMMARY"
