package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-opentofu-change-contract/internal/contract"
)

func main() {
	source := flag.String("source", ".gooo/change-contract.gooo", "Gooo semantic source")
	plan := flag.String("plan", "fixtures/plan.json", "pinned OpenTofu plan JSON fixture")
	openAPI := flag.String("openapi", "fixtures/openapi.json", "pinned OpenAPI fixture")
	mapping := flag.String("mapping", "fixtures/resource-service-map.json", "explicit resource-service mapping fixture")
	lock := flag.String("lock", "contracts/input-lock-v1.json", "input digest lock")
	output := flag.String("output", "output", "caller-owned output directory")
	caseID := flag.String("case", "mapped-additive", "canonical case to evaluate")
	flag.Parse()

	bundle, err := contract.LoadBundle(*source, *plan, *openAPI, *mapping, *lock)
	if err != nil {
		var contradiction *contract.DigestContradictionError
		if !errors.As(err, &contradiction) || *caseID != "digest-contradiction" {
			fail(err)
		}
	}
	evaluation, err := contract.Evaluate(bundle, *caseID)
	if err != nil {
		fail(err)
	}
	outputs, err := contract.Render(evaluation, bundle.Schema)
	if err != nil {
		fail(err)
	}
	if err := contract.WriteOutputs(*output, outputs); err != nil {
		fail(err)
	}
	if evaluation.Decision != contract.Closed && *caseID == "mapped-additive" {
		fail(fmt.Errorf("default mapped-additive case did not close"))
	}
}
func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "gooo-change-contract:", err)
	os.Exit(1)
}
