package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Bundle struct {
	Schema  Schema
	Plan    Plan
	OpenAPI OpenAPI
	Mapping MappingFile
	Lock    InputLock
	Digests InputDigests
}

func DigestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func LoadBundle(sourcePath, planPath, openAPIPath, mappingPath, lockPath string) (Bundle, error) {
	schema, err := ParseSchema(sourcePath)
	if err != nil {
		return Bundle{}, err
	}
	var plan Plan
	if err := ReadJSON(planPath, &plan); err != nil {
		return Bundle{}, err
	}
	var openAPI OpenAPI
	if err := ReadJSON(openAPIPath, &openAPI); err != nil {
		return Bundle{}, err
	}
	var mapping MappingFile
	if err := ReadJSON(mappingPath, &mapping); err != nil {
		return Bundle{}, err
	}
	var lock InputLock
	if err := ReadJSON(lockPath, &lock); err != nil {
		return Bundle{}, err
	}
	if lock.Schema != "gooo/opentofu-change-contract/input-lock/v1" || lock.Version != 1 {
		return Bundle{}, fmt.Errorf("unsupported input lock")
	}
	planDigest, err := DigestFile(planPath)
	if err != nil {
		return Bundle{}, err
	}
	openAPIDigest, err := DigestFile(openAPIPath)
	if err != nil {
		return Bundle{}, err
	}
	mappingDigest, err := DigestFile(mappingPath)
	if err != nil {
		return Bundle{}, err
	}
	schemaDigest, err := DigestFile(sourcePath)
	if err != nil {
		return Bundle{}, err
	}
	digests := InputDigests{PlanSHA256: planDigest, OpenAPISHA256: openAPIDigest, MappingSHA256: mappingDigest, SchemaSHA256: schemaDigest}
	if lock.PlanSHA256 != planDigest || lock.OpenAPISHA256 != openAPIDigest || lock.MappingSHA256 != mappingDigest || lock.SchemaSHA256 != schemaDigest {
		return Bundle{Schema: schema, Plan: plan, OpenAPI: openAPI, Mapping: mapping, Lock: lock, Digests: digests}, &DigestContradictionError{Expected: lock, Observed: digests}
	}
	if mapping.Schema != "gooo/opentofu-change-contract/resource-service-map/v1" || mapping.Version != 1 || mapping.Scope != "FIXTURE_ONLY" {
		return Bundle{}, fmt.Errorf("unsupported explicit mapping")
	}
	return Bundle{Schema: schema, Plan: plan, OpenAPI: openAPI, Mapping: mapping, Lock: lock, Digests: digests}, nil
}

type DigestContradictionError struct {
	Expected InputLock
	Observed InputDigests
}

func (e *DigestContradictionError) Error() string {
	return "input lock contradicts observed fixture digest"
}

func ParseVersionMajor(value string) (int, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("missing format version")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid format version %q", value)
	}
	return major, nil
}

func (b Bundle) ValidatePlan() error {
	major, err := ParseVersionMajor(b.Plan.FormatVersion)
	if err != nil {
		return err
	}
	if major != 1 {
		return fmt.Errorf("unsupported OpenTofu plan JSON major %d", major)
	}
	if b.Plan.GoooPin.Engine != "OPENTOFU" || b.Plan.GoooPin.Release == "" || b.Plan.GoooPin.SourceKind != "PINNED_PLAN_JSON" {
		return fmt.Errorf("explicit OpenTofu plan pin is incomplete")
	}
	if b.Plan.GoooPin.Scope != "FIXTURE_ONLY" {
		return fmt.Errorf("plan scope escalates beyond fixture-only")
	}
	if b.Plan.Errored {
		return fmt.Errorf("pinned plan is errored")
	}
	if len(b.Plan.ResourceChanges) == 0 {
		return fmt.Errorf("pinned plan has no resource changes")
	}
	seen := make(map[string]bool)
	for _, change := range b.Plan.ResourceChanges {
		if change.Address == "" || seen[change.Address] {
			return fmt.Errorf("resource change address is missing or duplicated")
		}
		seen[change.Address] = true
		if len(change.Change.Actions) != 1 {
			return fmt.Errorf("resource change %s has ambiguous action set", change.Address)
		}
	}
	return nil
}

func validateOpenAPI(document OpenAPI) error {
	major, err := ParseVersionMajor(document.OpenAPI)
	if err != nil {
		return err
	}
	if major != 3 || len(document.Paths) == 0 {
		return fmt.Errorf("unsupported or empty OpenAPI document")
	}
	return nil
}

func canonicalCase(schema Schema, caseID string) (CaseSpec, error) {
	for _, item := range schema.Cases {
		if item.ID == caseID {
			return item, nil
		}
	}
	return CaseSpec{}, fmt.Errorf("unknown canonical case %q", caseID)
}

func actionSemantics(schema Schema) map[string]string {
	result := make(map[string]string, len(schema.Actions))
	for _, action := range schema.Actions {
		result[action.Name] = action.Semantics
	}
	return result
}

func mappingIndex(mappings []Mapping) (map[string]Mapping, error) {
	result := make(map[string]Mapping, len(mappings))
	for _, item := range mappings {
		if item.ResourceAddress == "" || result[item.ResourceAddress].ResourceAddress != "" {
			return nil, fmt.Errorf("resource-service mapping address is missing or ambiguous")
		}
		if !item.Explicit || item.Service == "" || item.Contract == "" || item.Capability == "" || item.UserPath == "" {
			return nil, fmt.Errorf("resource-service mapping is not explicit")
		}
		result[item.ResourceAddress] = item
	}
	return result, nil
}

func configurationDependencies(plan Plan) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, resource := range plan.Configuration.RootModule.Resources {
		if resource.Address == "" || result[resource.Address] != nil {
			return nil, fmt.Errorf("configuration resource address is missing or ambiguous")
		}
		deps := append([]string(nil), resource.DependsOn...)
		sort.Strings(deps)
		result[resource.Address] = deps
	}
	return result, nil
}

func operationExists(document OpenAPI, operation Operation) bool {
	path, ok := document.Paths[operation.Path]
	if !ok {
		return false
	}
	value, ok := path[strings.ToLower(operation.Method)]
	if !ok {
		return false
	}
	operationID, _ := value.(map[string]any)["operationId"].(string)
	return operationID == operation.OperationID
}

func activityID(schema Schema, name string) string {
	for _, activity := range schema.Activities {
		if activity.Name == name {
			return activity.ID
		}
	}
	return ""
}

func buildImpacts(bundle Bundle, schema Schema, mappings []Mapping, ignoreDestructiveDependency bool) ([]ImpactEvent, map[string][]string, error) {
	if err := bundle.ValidatePlan(); err != nil {
		return nil, nil, err
	}
	if err := validateOpenAPI(bundle.OpenAPI); err != nil {
		return nil, nil, err
	}
	index, err := mappingIndex(mappings)
	if err != nil {
		return nil, nil, err
	}
	dependencies, err := configurationDependencies(bundle.Plan)
	if err != nil {
		return nil, nil, err
	}
	semantics := actionSemantics(schema)
	actions := make(map[string]string)
	for _, resource := range bundle.Plan.ResourceChanges {
		action := resource.Change.Actions[0]
		semantic, ok := semantics[action]
		if !ok {
			return nil, nil, fmt.Errorf("unmodeled OpenTofu action %q", action)
		}
		if _, ok := index[resource.Address]; !ok {
			return nil, nil, fmt.Errorf("missing explicit mapping for %s", resource.Address)
		}
		actions[resource.Address] = action
		_ = semantic
	}
	for address := range index {
		if _, ok := actions[address]; !ok {
			return nil, nil, fmt.Errorf("mapping has no plan resource %s", address)
		}
	}

	if ignoreDestructiveDependency {
		for dependent, deps := range dependencies {
			for _, dependency := range deps {
				if actions[dependency] == "delete" || actions[dependency] == "replace" {
					return nil, dependencies, fmt.Errorf("destructive dependency %s was ignored by propagation for %s", dependency, dependent)
				}
			}
		}
	}

	addresses := make([]string, 0, len(actions))
	for address := range actions {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	impacts := make([]ImpactEvent, 0, len(addresses))
	for sequence, address := range addresses {
		resource := index[address]
		frontier := make([]string, 0)
		for _, dependency := range dependencies[address] {
			if _, changed := actions[dependency]; changed {
				frontier = append(frontier, dependency)
			}
		}
		impact := ImpactEvent{
			Sequence:        sequence + 1,
			ResourceAddress: address,
			Action:          actions[address],
			Semantics:       semantics[actions[address]],
			Service:         resource.Service,
			Contract:        resource.Contract,
			Capability:      resource.Capability,
			Operation:       resource.Operation,
			UserPath:        resource.UserPath,
			CausalFrontier:  frontier,
			State:           Closed,
			ActivityID:      activityID(schema, "PropagateImpactAcrossDependencies"),
		}
		if !operationExists(bundle.OpenAPI, resource.Operation) {
			return nil, dependencies, fmt.Errorf("mapping operation is not present in pinned OpenAPI: %s", address)
		}
		impacts = append(impacts, impact)
	}
	return impacts, dependencies, nil
}
func unknownClaim(schema Schema, stage, step, reason, class, next string, blocked []string) UnknownClaim {
	return UnknownClaim{State: Unknown, Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func refutation(stage, step, reason, next, evidence string) Refutation {
	return Refutation{State: Refuted, Stage: stage, Step: step, Reason: reason, NextOperation: next, Evidence: evidence}
}

func caseCounts(schema Schema) map[string]int {
	counts := map[string]int{Closed: 0, Unknown: 0, Refuted: 0}
	for _, item := range schema.Cases {
		counts[item.State]++
	}
	return counts
}

func activityReceipts(schema Schema, state string) []ActivityReceipt {
	result := make([]ActivityReceipt, 0, len(schema.Activities))
	for ordinal, activity := range schema.Activities {
		result = append(result, ActivityReceipt{Ordinal: ordinal + 1, ID: activity.ID, Activity: activity.Name, Occurrence: 1, State: state})
	}
	return result
}

func cloneBundle(bundle Bundle) Bundle {
	planBytes, _ := json.Marshal(bundle.Plan)
	var plan Plan
	_ = json.Unmarshal(planBytes, &plan)
	mappingBytes, _ := json.Marshal(bundle.Mapping)
	var mapping MappingFile
	_ = json.Unmarshal(mappingBytes, &mapping)
	bundle.Plan = plan
	bundle.Mapping = mapping
	return bundle
}

func Evaluate(bundle Bundle, caseID string) (Evaluation, error) {
	item, err := canonicalCase(bundle.Schema, caseID)
	if err != nil {
		return Evaluation{}, err
	}
	evaluation := Evaluation{Case: item, Mappings: append([]Mapping(nil), bundle.Mapping.Mappings...), InputDigests: bundle.Digests, Plan: bundle.Plan, OpenAPI: bundle.OpenAPI, CanonicalCaseCounts: caseCounts(bundle.Schema)}
	evaluation.ActivityReceipts = activityReceipts(bundle.Schema, item.State)

	switch item.Kind {
	case "STALE_INPUT":
		evaluation.Decision = Unknown
		evaluation.Claims = []UnknownClaim{unknownClaim(bundle.Schema, "INPUT", "VERIFY_PINNED_PLAN_SCHEMA", "STALE_PLAN_OR_SCHEMA_DIGEST", "STALE_INPUT", "PIN_CURRENT_PLAN_AND_OPENAPI_DIGESTS", []string{activityID(bundle.Schema, "ReadPinnedOpenTofuPlan")})}
		return evaluation, nil
	case "DIGEST_CONTRADICTION":
		evaluation.Decision = Refuted
		evaluation.Refutations = []Refutation{refutation("INPUT", "COMPARE_PINNED_INPUT_DIGESTS", "DIGEST_CONTRADICTION", "REPIN_ALL_INPUT_DIGESTS", "lock digest differs from observed fixture digest")}
		return evaluation, nil
	case "SCOPE_ESCALATION":
		evaluation.Decision = Refuted
		evaluation.Refutations = []Refutation{refutation("AUTHORITY", "ENFORCE_FIXTURE_ONLY_SCOPE", "SCOPE_ESCALATION", "REMOVE_APPLY_CLOUD_OR_NETWORK_SCOPE", "requested scope is outside FIXTURE_ONLY")}
		return evaluation, nil
	case "AMBIGUOUS_ADDRESS":
		mappings := append([]Mapping(nil), bundle.Mapping.Mappings...)
		mappings = append(mappings, mappings[0])
		_, _, buildErr := buildImpacts(bundle, bundle.Schema, mappings, false)
		if buildErr == nil {
			return Evaluation{}, fmt.Errorf("ambiguous mapping fixture did not contradict")
		}
		evaluation.Decision = Unknown
		evaluation.Claims = []UnknownClaim{unknownClaim(bundle.Schema, "MAPPING", "RESOLVE_RESOURCE_ADDRESS", "AMBIGUOUS_RESOURCE_ADDRESS_MAPPING", "AMBIGUOUS", "DISAMBIGUATE_RESOURCE_SERVICE_MAPPING", []string{})}
		return evaluation, nil
	case "MISSING_MAPPING":
		mappings := append([]Mapping(nil), bundle.Mapping.Mappings[:len(bundle.Mapping.Mappings)-1]...)
		_, _, buildErr := buildImpacts(bundle, bundle.Schema, mappings, false)
		if buildErr == nil {
			return Evaluation{}, fmt.Errorf("missing mapping fixture did not contradict")
		}
		evaluation.Decision = Unknown
		evaluation.Claims = []UnknownClaim{unknownClaim(bundle.Schema, "MAPPING", "RESOLVE_RESOURCE_SERVICE_MAPPING", "EXPLICIT_RESOURCE_SERVICE_MAPPING_MISSING", "DIRECT_MISSING", "PROVIDE_EXPLICIT_RESOURCE_SERVICE_MAPPING", []string{})}
		return evaluation, nil
	case "IGNORED_DESTRUCTIVE_DEPENDENCY":
		mutated := cloneBundle(bundle)
		mutated.Plan.Configuration.RootModule.Resources[1].DependsOn = append(mutated.Plan.Configuration.RootModule.Resources[1].DependsOn, "aws_s3_bucket.receipts")
		impacts, dependencies, buildErr := buildImpacts(mutated, bundle.Schema, mutated.Mapping.Mappings, true)
		_ = impacts
		_ = dependencies
		if buildErr == nil {
			return Evaluation{}, fmt.Errorf("ignored destructive dependency fixture did not contradict")
		}
		evaluation.Decision = Refuted
		evaluation.Refutations = []Refutation{refutation("PROPAGATION", "PROPAGATE_DESTRUCTIVE_DEPENDENCY", "IGNORED_DESTRUCTIVE_DEPENDENCY", "EMIT_DEPENDENT_SERVICE_IMPACT", buildErr.Error())}
		return evaluation, nil
	}

	impacts, dependencies, err := buildImpacts(bundle, bundle.Schema, bundle.Mapping.Mappings, false)
	if err != nil {
		return Evaluation{}, err
	}
	evaluation.Decision = Closed
	evaluation.Impacts = impacts
	evaluation.Dependencies = dependencies
	for _, impact := range impacts {
		if impact.Action == item.Action {
			evaluation.TargetResource = impact.ResourceAddress
			break
		}
	}
	if evaluation.TargetResource == "" {
		return Evaluation{}, fmt.Errorf("canonical case %s has no matching action", item.ID)
	}
	return evaluation, nil
}
