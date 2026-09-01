package contract

import "encoding/json"

const (
	Closed  = "CLOSED"
	Unknown = "UNKNOWN"
	Refuted = "REFUTED"
)

var OutputNames = []string{
	"change-contract.json",
	"impact-events.ndjson",
	"resource-service-map.json",
	"unknown-frontier.json",
	"decision-receipt.json",
	"replay-receipt.json",
	"report.md",
}

type OutputSpec struct {
	Name string
	Kind string
}

type ActivitySpec struct {
	ID    string
	Name  string
	Stage string
	Step  string
}

type ActionSpec struct {
	Name      string
	Semantics string
}

type CaseSpec struct {
	ID     string
	State  string
	Kind   string
	Action string
}

type RelationSpec struct {
	From      string
	To        string
	Predicate string
}

type Schema struct {
	Package    string
	Namespace  string
	Scope      map[string]string
	Outputs    []OutputSpec
	Activities []ActivitySpec
	Actions    []ActionSpec
	Cases      []CaseSpec
	Relations  []RelationSpec
}

type PlanPin struct {
	Engine     string `json:"engine"`
	Release    string `json:"release"`
	Scope      string `json:"scope"`
	SourceKind string `json:"source_kind"`
}

type Plan struct {
	FormatVersion    string           `json:"format_version"`
	TerraformVersion string           `json:"terraform_version"`
	GoooPin          PlanPin          `json:"gooo_pin"`
	Configuration    Configuration    `json:"configuration"`
	ResourceChanges  []ResourceChange `json:"resource_changes"`
	Errored          bool             `json:"errored"`
}

type Configuration struct {
	RootModule RootModule `json:"root_module"`
}

type RootModule struct {
	Resources []ConfigResource `json:"resources"`
}

type ConfigResource struct {
	Address   string   `json:"address"`
	DependsOn []string `json:"depends_on"`
}

type ResourceChange struct {
	Address         string `json:"address"`
	PreviousAddress string `json:"previous_address,omitempty"`
	Change          struct {
		Actions []string `json:"actions"`
	} `json:"change"`
}

type MappingFile struct {
	Schema   string    `json:"schema"`
	Version  int       `json:"version"`
	Scope    string    `json:"scope"`
	Mappings []Mapping `json:"mappings"`
}

type Mapping struct {
	ResourceAddress string    `json:"resource_address"`
	Service         string    `json:"service"`
	Contract        string    `json:"contract"`
	Capability      string    `json:"capability"`
	Operation       Operation `json:"operation"`
	UserPath        string    `json:"user_path"`
	Explicit        bool      `json:"explicit"`
}

type Operation struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operation_id"`
}

type OpenAPI struct {
	OpenAPI string                    `json:"openapi"`
	Info    map[string]any            `json:"info"`
	Paths   map[string]map[string]any `json:"paths"`
}

type UnknownClaim struct {
	State         string   `json:"state"`
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type Refutation struct {
	State         string `json:"state"`
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	NextOperation string `json:"next_operation"`
	Evidence      string `json:"evidence"`
}

type ImpactEvent struct {
	Sequence        int       `json:"sequence"`
	ResourceAddress string    `json:"resource_address"`
	Action          string    `json:"action"`
	Semantics       string    `json:"semantics"`
	Service         string    `json:"service"`
	Contract        string    `json:"contract"`
	Capability      string    `json:"capability"`
	Operation       Operation `json:"operation"`
	UserPath        string    `json:"user_path"`
	CausalFrontier  []string  `json:"causal_frontier"`
	State           string    `json:"state"`
	ActivityID      string    `json:"activity_id"`
}

type ActivityReceipt struct {
	Ordinal    int    `json:"ordinal"`
	ID         string `json:"id"`
	Activity   string `json:"activity"`
	Occurrence int    `json:"occurrence"`
	State      string `json:"state"`
}

type InputDigests struct {
	PlanSHA256    string `json:"plan_sha256"`
	OpenAPISHA256 string `json:"openapi_sha256"`
	MappingSHA256 string `json:"mapping_sha256"`
	SchemaSHA256  string `json:"schema_sha256"`
}

type InputLock struct {
	Schema        string `json:"schema"`
	Version       int    `json:"version"`
	PlanSHA256    string `json:"plan_sha256"`
	OpenAPISHA256 string `json:"openapi_sha256"`
	MappingSHA256 string `json:"mapping_sha256"`
	SchemaSHA256  string `json:"schema_sha256"`
}

type Evaluation struct {
	Case                CaseSpec
	Decision            string
	Impacts             []ImpactEvent
	Claims              []UnknownClaim
	Refutations         []Refutation
	Mappings            []Mapping
	Dependencies        map[string][]string
	TargetResource      string
	InputDigests        InputDigests
	ActivityReceipts    []ActivityReceipt
	CanonicalCaseCounts map[string]int
	Plan                Plan
	OpenAPI             OpenAPI
}

func (e Evaluation) MarshalPlan() ([]byte, error) {
	return json.Marshal(e.Plan)
}
