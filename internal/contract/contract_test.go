package contract

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSchemaOwnsFixedDenominator(t *testing.T) {
	schema, err := ParseSchema("../../.gooo/change-contract.gooo")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Activities) != 10 || len(schema.Cases) != 9 || len(schema.Outputs) != 7 {
		t.Fatalf("unexpected .gooo denominator: activities=%d cases=%d outputs=%d", len(schema.Activities), len(schema.Cases), len(schema.Outputs))
	}
}
func testBundle(t *testing.T) Bundle {
	t.Helper()
	bundle, err := LoadBundle("../../.gooo/change-contract.gooo", "../../fixtures/plan.json", "../../fixtures/openapi.json", "../../fixtures/resource-service-map.json", "../../contracts/input-lock-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestCanonicalCasesResolveToFixedStates(t *testing.T) {
	bundle := testBundle(t)
	want := map[string]string{
		"mapped-additive": Closed,
		"mapped-update": Closed,
		"mapped-delete": Closed,
		"missing-mapping": Unknown,
		"stale-plan-schema": Unknown,
		"ambiguous-address": Unknown,
		"digest-contradiction": Refuted,
		"ignored-destructive-dependency": Refuted,
		"scope-escalation": Refuted,
	}
	for caseID, expected := range want {
		item, err := Evaluate(bundle, caseID)
		if err != nil {
			t.Fatalf("case %s: %v", caseID, err)
		}
		if item.Decision != expected {
			t.Errorf("case %s: got %s want %s", caseID, item.Decision, expected)
		}
	}
}

func TestUnknownClaimHasExactSixCoordinates(t *testing.T) {
	bundle := testBundle(t)
	evaluation, err := Evaluate(bundle, "missing-mapping")
	if err != nil {
		t.Fatal(err)
	}
	if len(evaluation.Claims) != 1 || len(evaluation.Claims[0].BlockedBy) != 0 {
		t.Fatalf("unexpected unknown claim: %#v", evaluation.Claims)
	}
	data, err := json.Marshal(evaluation.Claims[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Fatalf("unknown claim has %d fields, want state plus six coordinates", len(fields))
	}
	for _, field := range []string{"state", "stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("unknown claim missing %s", field)
		}
	}
}

func TestRenderIsExactlySevenOutputsAndReplayStable(t *testing.T) {
	bundle := testBundle(t)
	evaluation, err := Evaluate(bundle, "mapped-additive")
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := Render(evaluation, bundle.Schema)
	if err != nil {
		t.Fatal(err)
	}
	if !firstHasExactOutputs(outputs) {
		t.Fatalf("output names are not exact: %#v", outputs)
	}
	dir := t.TempDir()
	if err := WriteOutputs(dir, outputs); err != nil {
		t.Fatal(err)
	}
	for _, name := range OutputNames {
		if _, err := os.Stat(dir + "/" + name); err != nil {
			t.Errorf("missing output %s: %v", name, err)
		}
	}
}
