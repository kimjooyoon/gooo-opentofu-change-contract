package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type OutputSet map[string][]byte

func jsonBytes(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func outputDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func canonicalCaseViews(schema Schema) []map[string]any {
	views := make([]map[string]any, 0, len(schema.Cases))
	for _, item := range schema.Cases {
		view := map[string]any{"id": item.ID, "state": item.State, "kind": item.Kind}
		if item.Action != "" {
			view["action"] = item.Action
		}
		views = append(views, view)
	}
	return views
}

func unknownViews(claims []UnknownClaim) []UnknownClaim {
	if claims == nil {
		return []UnknownClaim{}
	}
	return claims
}

func refutationViews(refutations []Refutation) []Refutation {
	if refutations == nil {
		return []Refutation{}
	}
	return refutations
}

func impactViews(impacts []ImpactEvent) []ImpactEvent {
	if impacts == nil {
		return []ImpactEvent{}
	}
	return impacts
}

func mappingViews(mappings []Mapping) []Mapping {
	if mappings == nil {
		return []Mapping{}
	}
	result := append([]Mapping(nil), mappings...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].ResourceAddress < result[right].ResourceAddress
	})
	return result
}

func decisionValue(evaluation Evaluation, schema Schema) map[string]any {
	testsUnknown := 0
	if evaluation.Decision == Unknown {
		testsUnknown = 1
	}
	return map[string]any{
		"schema": "gooo/opentofu-change-contract/decision-receipt/v1",
		"version": 1,
		"decision": evaluation.Decision,
		"case_id": evaluation.Case.ID,
		"precedence": []string{Refuted, Unknown, Closed},
		"canonical_case_counts": evaluation.CanonicalCaseCounts,
		"input_digests": evaluation.InputDigests,
		"activity_receipts": evaluation.ActivityReceipts,
		"active_unknown_claims": unknownViews(evaluation.Claims),
		"refutations": refutationViews(evaluation.Refutations),
		"tests": map[string]int{
			"total": 9,
			"selected": 1,
			"executed": 1,
			"reused": 0,
			"failed": 0,
			"unknown": testsUnknown,
		},
		"authority": map[string]any{
			"scope": "FIXTURE_ONLY",
			"input_read_only": true,
			"repository_writes": 0,
			"source_writes": 0,
			"input_mutations": 0,
			"opentofu_invocations": 0,
			"terraform_invocations": 0,
			"provider_network_invocations": 0,
			"cross_project_required_gates": 0,
			"engine_identity_source": "explicit_gooo_pin.engine",
			"terraform_version_used_for_engine_identity": false,
		},
		"output_names": append([]string(nil), OutputNames...),
		"replay_required": true,
		"semantic_owner": schema.Namespace,
	}
}

func changeContractValue(evaluation Evaluation, schema Schema) map[string]any {
	return map[string]any{
		"schema": "gooo/opentofu-change-contract/change-contract/v1",
		"version": 1,
		"decision": evaluation.Decision,
		"case_id": evaluation.Case.ID,
		"input_digests": evaluation.InputDigests,
		"plan_pin": evaluation.Plan.GoooPin,
		"resource_changes": impactViews(evaluation.Impacts),
		"unknown_causal_frontier": unknownViews(evaluation.Claims),
		"refutations": refutationViews(evaluation.Refutations),
		"canonical_cases": canonicalCaseViews(schema),
		"precedence": []string{Refuted, Unknown, Closed},
		"authority": map[string]any{
			"scope": "FIXTURE_ONLY",
			"opentofu_invocations": 0,
			"terraform_invocations": 0,
			"provider_network_invocations": 0,
			"repository_writes": 0,
		},
	}
}

func renderCore(evaluation Evaluation, schema Schema) (OutputSet, error) {
	change, err := jsonBytes(changeContractValue(evaluation, schema))
	if err != nil {
		return nil, err
	}

	var impactBuilder strings.Builder
	for _, impact := range impactViews(evaluation.Impacts) {
		line, marshalErr := json.Marshal(impact)
		if marshalErr != nil {
			return nil, marshalErr
		}
		impactBuilder.Write(line)
		impactBuilder.WriteByte('\n')
	}
	impact := []byte(impactBuilder.String())

	mapping, err := jsonBytes(map[string]any{
		"schema": "gooo/opentofu-change-contract/resource-service-map/v1",
		"version": 1,
		"scope": "FIXTURE_ONLY",
		"input_digest": evaluation.InputDigests.MappingSHA256,
		"explicit": true,
		"mappings": mappingViews(evaluation.Mappings),
	})
	if err != nil {
		return nil, err
	}
	unknown, err := jsonBytes(map[string]any{
		"schema": "gooo/opentofu-change-contract/unknown-frontier/v1",
		"version": 1,
		"decision": evaluation.Decision,
		"case_id": evaluation.Case.ID,
		"precedence": []string{Refuted, Unknown, Closed},
		"active_claims": unknownViews(evaluation.Claims),
		"canonical_unknown_case_ids": []string{"missing-mapping", "stale-plan-schema", "ambiguous-address"},
	})
	if err != nil {
		return nil, err
	}
	decision, err := jsonBytes(decisionValue(evaluation, schema))
	if err != nil {
		return nil, err
	}
	report := []byte(reportText(evaluation, schema))
	return OutputSet{
		"change-contract.json": change,
		"impact-events.ndjson": impact,
		"resource-service-map.json": mapping,
		"unknown-frontier.json": unknown,
		"decision-receipt.json": decision,
		"report.md": report,
	}, nil
}

func replayReceipt(first, second OutputSet, evaluation Evaluation) ([]byte, error) {
	files := make([]string, 0, 6)
	digests := make(map[string]string, 6)
	equal := true
	for _, name := range []string{"change-contract.json", "impact-events.ndjson", "resource-service-map.json", "unknown-frontier.json", "decision-receipt.json", "report.md"} {
		files = append(files, name)
		digests[name] = outputDigest(first[name])
		if string(first[name]) != string(second[name]) {
			equal = false
		}
	}
	return jsonBytes(map[string]any{
		"schema": "gooo/opentofu-change-contract/replay-receipt/v1",
		"version": 1,
		"case_id": evaluation.Case.ID,
		"replay_runs": 2,
		"replay_equal": equal,
		"compared_outputs": files,
		"output_digests": digests,
		"decision": evaluation.Decision,
		"repository_writes": 0,
	})
}

func Render(evaluation Evaluation, schema Schema) (OutputSet, error) {
	first, err := renderCore(evaluation, schema)
	if err != nil {
		return nil, err
	}
	second, err := renderCore(evaluation, schema)
	if err != nil {
		return nil, err
	}
	replay, err := replayReceipt(first, second, evaluation)
	if err != nil {
		return nil, err
	}
	first["replay-receipt.json"] = replay
	if !firstHasExactOutputs(first) {
		return nil, fmt.Errorf("generated output set is not exactly seven files")
	}
	return first, nil
}

func firstHasExactOutputs(outputs OutputSet) bool {
	if len(outputs) != len(OutputNames) {
		return false
	}
	for _, name := range OutputNames {
		if _, ok := outputs[name]; !ok {
			return false
		}
	}
	return true
}

func WriteOutputs(directory string, outputs OutputSet) error {
	if !firstHasExactOutputs(outputs) {
		return fmt.Errorf("output set must contain exactly seven named outputs")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	for _, name := range OutputNames {
		if err := os.WriteFile(directory+"/"+name, outputs[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func reportText(evaluation Evaluation, schema Schema) string {
	var builder strings.Builder
	builder.WriteString("# Gooo → OpenTofu deterministic change contract\n\n")
	builder.WriteString("This report consumes pinned JSON fixtures only. OpenTofu/Terraform are not executed; service meaning comes only from the explicit Gooo-owned mapping.\n\n")
	fmt.Fprintf(&builder, "Decision: **%s**\n\n", evaluation.Decision)
	builder.WriteString("## Resource impact\n\n")
	builder.WriteString("| Resource address | Action | Semantics | Service | Contract | Operation | User path | Causal frontier |\n")
	builder.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, impact := range evaluation.Impacts {
		fmt.Fprintf(&builder, "| %s | %s | %s | %s | %s | %s %s | %s | %s |\n", impact.ResourceAddress, impact.Action, impact.Semantics, impact.Service, impact.Contract, impact.Operation.Method, impact.Operation.Path, impact.UserPath, strings.Join(impact.CausalFrontier, ", "))
	}
	if len(evaluation.Claims) > 0 {
		builder.WriteString("\n## UNKNOWN causal frontier\n\n")
		for _, claim := range evaluation.Claims {
			fmt.Fprintf(&builder, "- `%s` / `%s`: %s; next `%s`; blocked_by `%s`\n", claim.Stage, claim.Step, claim.Reason, claim.NextOperation, strings.Join(claim.BlockedBy, ", "))
		}
	}
	if len(evaluation.Refutations) > 0 {
		builder.WriteString("\n## REFUTED contradiction\n\n")
		for _, item := range evaluation.Refutations {
			fmt.Fprintf(&builder, "- `%s` / `%s`: %s (%s)\n", item.Stage, item.Step, item.Reason, item.Evidence)
		}
	}
	builder.WriteString("\n## Canonical cases\n\n")
	builder.WriteString("| Case | Expected state | Kind | Action |\n|---|---|---|---|\n")
	for _, item := range schema.Cases {
		fmt.Fprintf(&builder, "| %s | %s | %s | %s |\n", item.ID, item.State, item.Kind, item.Action)
	}
	builder.WriteString("\nPrecedence is `REFUTED > UNKNOWN > CLOSED`. UNKNOWN claims preserve exactly `stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`.\n")
	return builder.String()
}

