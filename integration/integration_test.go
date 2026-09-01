//go:build integration

package integration_test

import (
	"testing"

	"github.com/kimjooyoon/gooo-opentofu-change-contract/internal/contract"
)

func TestFixtureConsumerIntegration(t *testing.T) {
	bundle, err := contract.LoadBundle("../.gooo/change-contract.gooo", "../fixtures/plan.json", "../fixtures/openapi.json", "../fixtures/resource-service-map.json", "../contracts/input-lock-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := contract.Evaluate(bundle, "mapped-update")
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision != contract.Closed || len(evaluation.Impacts) != 3 {
		t.Fatalf("unexpected integration result: decision=%s impacts=%d", evaluation.Decision, len(evaluation.Impacts))
	}
}

