#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work=${1:-"${RUNNER_TEMP:-/tmp}/gooo-opentofu-change-contract-conformance"}
rm -rf "$work"
mkdir -p "$work"
cd "$root"

before=$(mktemp "$work/repository-before.XXXXXX")
after=$(mktemp "$work/repository-after.XXXXXX")
git status --porcelain=v1 --untracked-files=all > "$before"

base="$work/base"
go run ./cmd/gooo-change-contract --output "$base"
test "$(find "$base" -type f -maxdepth 1 | wc -l | tr -d ' ')" = 7
test "$(find "$base" -type f -maxdepth 1 -exec basename {} \; | sort | paste -sd, -)" = "change-contract.json,decision-receipt.json,impact-events.ndjson,report.md,replay-receipt.json,resource-service-map.json,unknown-frontier.json"
jq -e '
  .schema == "gooo/opentofu-change-contract/change-contract/v1" and
  .decision == "CLOSED" and
  (.resource_changes|length) == 3 and
  ([.resource_changes[].action]|sort) == ["create","delete","update"] and
  .authority.repository_writes == 0 and
  .authority.opentofu_invocations == 0 and
  (.canonical_cases|length) == 9
' "$base/change-contract.json" >/dev/null
jq -e '
  .schema == "gooo/opentofu-change-contract/decision-receipt/v1" and
  .decision == "CLOSED" and
  .canonical_case_counts == {CLOSED:3,UNKNOWN:3,REFUTED:3} and
  (.activity_receipts|length) == 10 and all(.activity_receipts[]; .occurrence == 1) and
  .tests == {total:9,selected:1,executed:1,reused:0,failed:0,unknown:0} and
  .output_names == ["change-contract.json","impact-events.ndjson","resource-service-map.json","unknown-frontier.json","decision-receipt.json","replay-receipt.json","report.md"]
' "$base/decision-receipt.json" >/dev/null
jq -e '
  .schema == "gooo/opentofu-change-contract/replay-receipt/v1" and
  .replay_runs == 2 and .replay_equal == true and (.compared_outputs|length) == 6 and .repository_writes == 0
' "$base/replay-receipt.json" >/dev/null

replay="$work/replay"
go run ./cmd/gooo-change-contract --output "$replay"
for name in change-contract.json impact-events.ndjson resource-service-map.json unknown-frontier.json decision-receipt.json replay-receipt.json report.md; do
  cmp -s "$base/$name" "$replay/$name"
done

declare -A expected=(
  [mapped-additive]=CLOSED
  [mapped-update]=CLOSED
  [mapped-delete]=CLOSED
  [missing-mapping]=UNKNOWN
  [stale-plan-schema]=UNKNOWN
  [ambiguous-address]=UNKNOWN
  [digest-contradiction]=REFUTED
  [ignored-destructive-dependency]=REFUTED
  [scope-escalation]=REFUTED
)
for case_id in "${!expected[@]}"; do
  case_dir="$work/cases/$case_id"
  go run ./cmd/gooo-change-contract --case "$case_id" --output "$case_dir"
  jq -e --arg want "${expected[$case_id]}" --arg case_id "$case_id" '.decision == $want and .case_id == $case_id' "$case_dir/decision-receipt.json" >/dev/null
done

for case_id in missing-mapping stale-plan-schema ambiguous-address; do
  jq -e '
    .decision == "UNKNOWN" and (.active_claims|length) == 1 and
    ([.active_claims[0]|keys_unsorted[]] | sort) == ["blocked_by","next_operation","reason","stage","state","step","unknown_class"] and
    (.active_claims[0].blocked_by|type) == "array"
  ' "$work/cases/$case_id/unknown-frontier.json" >/dev/null
done
for case_id in digest-contradiction ignored-destructive-dependency scope-escalation; do
  jq -e '.decision == "REFUTED" and (.refutations|length) == 1' "$work/cases/$case_id/decision-receipt.json" >/dev/null
done

git status --porcelain=v1 --untracked-files=all > "$after"
cmp -s "$before" "$after"

echo "conformance: CLOSED=3 UNKNOWN=3 REFUTED=3; outputs=7; replay=equal; repository_writes=0"

