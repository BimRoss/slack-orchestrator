package inbound

import "encoding/json"

// CapabilityContractV1 is the JSON shape of the runtime capability catalog. The concrete default
// (DefaultCapabilityContractV1) is authored in this repo as the product source of truth for dispatch;
// workers receive it on every JetStream message and do not fetch policy over HTTP for those turns.
type CapabilityContractV1 struct {
	Revision         string                 `json:"revision,omitempty"`
	CoreEmployees    []CapabilityEmployeeV1 `json:"coreEmployees"`
	Skills           []CapabilitySkillV1    `json:"skills"`
	EmployeeSkillIDs map[string][]string    `json:"employeeSkillIds"`
}

// CapabilityEmployeeV1 is a squad member row in the contract.
type CapabilityEmployeeV1 struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// CapabilitySkillV1 is a skill definition with runtime tool binding.
type CapabilitySkillV1 struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	RuntimeTool    string   `json:"runtimeTool"`
	RequiredParams []string `json:"requiredParams"`
	OptionalParams []string `json:"optionalParams"`
	Requires       []string `json:"requires,omitempty"`
}

// DefaultCapabilityContractJSON returns JSON bytes for the default BimRoss squad contract (canonical here).
func DefaultCapabilityContractJSON() json.RawMessage {
	c := DefaultCapabilityContractV1()
	raw, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	return raw
}

// DefaultCapabilityContractV1 returns the hardcoded squad + skill matrix (revision "default").
func DefaultCapabilityContractV1() *CapabilityContractV1 {
	return &CapabilityContractV1{
		Revision: "default",
		CoreEmployees: []CapabilityEmployeeV1{
			{ID: "alex", Label: "Alex", Description: "Head of Sales frameworks, pricing, and offer design."},
			{ID: "tim", Label: "Tim", Description: "Head of Simplifying focused on leverage and decision quality."},
			{ID: "ross", Label: "Ross", Description: "Head of Automation owning technical execution and shipping."},
			{ID: "garth", Label: "Garth", Description: "Head of Interns supporting research and implementation follow-through."},
			{ID: "joanne", Label: "Joanne", Description: "Head of Executive Operations for coordination and executive support."},
		},
		Skills: []CapabilitySkillV1{
			{
				ID: "write-email", Label: "Write Email", Description: "Draft, send, and triage email communication.",
				RuntimeTool: "joanne-write-email", RequiredParams: []string{"intent", "subject"},
				OptionalParams: []string{"to", "button", "commenters", "editors", "link", "viewers"},
			},
			{
				ID: "write-doc", Label: "Write Doc", Description: "Create, edit, and organize working docs.",
				RuntimeTool: "joanne-write-doc", RequiredParams: []string{"intent", "title", "type"},
				OptionalParams: []string{"commenters", "editors", "viewers"},
			},
			{
				ID: "write-company", Label: "Write Company", Description: "Provision a company channel, run onboarding, create channels, and invite members.",
				RuntimeTool: "joanne-write-company", RequiredParams: []string{"action", "intent"},
				OptionalParams: []string{"channel", "channel_name", "is_private", "reason"},
			},
			{
				ID: "read-company", Label: "Read Company", Description: "Summarize this channel from cached Slack history (Redis digest). Runs immediately (no confirmation).",
				RuntimeTool: "joanne-read-company", RequiredParams: []string{"intent"}, OptionalParams: []string{},
			},
			{
				ID: "read-twitter", Label: "Read Twitter", Description: "Search Twitter by keyword and fetch high-impression tweets (not the platform trend list).",
				RuntimeTool: "garth-read-twitter", RequiredParams: []string{"intent", "query"}, OptionalParams: []string{"count"},
			},
			{
				ID: "read-trends", Label: "Read Trends", Description: "Fetch the current Twitter/X trend list (not keyword search).",
				RuntimeTool: "garth-read-trends", RequiredParams: []string{"intent"}, OptionalParams: []string{"count"},
			},
		},
		EmployeeSkillIDs: map[string][]string{
			"alex":   {},
			"tim":    {},
			"ross":   {},
			"garth":  {"read-twitter", "read-trends"},
			"joanne": {"read-company", "write-company", "write-email", "write-doc"},
		},
	}
}
