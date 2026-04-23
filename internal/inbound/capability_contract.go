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
	ID             string            `json:"id"`
	Label          string            `json:"label"`
	Description    string            `json:"description"`
	RuntimeTool    string            `json:"runtimeTool"`
	RequiredParams []string          `json:"requiredParams"`
	OptionalParams []string          `json:"optionalParams"`
	ParamDefaults  map[string]string `json:"paramDefaults,omitempty"`
	Requires       []string          `json:"requires,omitempty"`
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
				ID: "create-email", Label: "Create Email", Description: "Draft, send, and triage email communication. Requires confirmation before send.",
				RuntimeTool: "joanne-create-email", RequiredParams: []string{"intent", "to"},
				OptionalParams: []string{"button", "link"},
				ParamDefaults: map[string]string{
					"to":     "Message author (Slack profile; makeacompany slack→email index when configured)",
					"button": "none",
					"link":   "none",
				},
			},
			{
				ID: "create-doc", Label: "Create Doc", Description: "Create, edit, and organize working docs. Requires confirmation before publish.",
				RuntimeTool: "joanne-create-doc", RequiredParams: []string{"intent", "title", "editors"},
				OptionalParams: []string{"commenters", "viewers", "type"},
				ParamDefaults: map[string]string{
					"title":      "Derived from intent when omitted; runtime infers a working title before draft",
					"editors":    "Message author email (implicit default); append @mentions or explicit editor emails",
					"type":       "outline",
					"commenters": "none",
					"viewers":    "none",
				},
			},
			{
				ID: "create-company", Label: "Create Company", Description: "Provision a company channel, run onboarding, create channels, and invite members. Requires confirmation before writes.",
				RuntimeTool: "joanne-create-company", RequiredParams: []string{"name", "founders"}, OptionalParams: []string{},
				ParamDefaults: map[string]string{
					"name":     "Company / channel slug (gathered in-thread when not in the first message)",
					"founders": "Message author (implicit default); the skill appends @mentioned cofounders",
				},
			},
			{
				ID: "delete-company", Label: "Delete Company", Description: "Permanently delete a company Slack channel and remove app-owned Redis data for that workspace (frees the channel name). Requires explicit Confirm/Cancel before any write.",
				RuntimeTool: "joanne-delete-company", RequiredParams: []string{"channel"}, OptionalParams: []string{},
				ParamDefaults: map[string]string{
					"channel": "The Slack channel where the command runs (implicit default; operators do not pass this at runtime)",
				},
			},
			{
				ID: "read-company", Label: "Read Company", Description: "Summarize this channel from cached Slack history (Redis digest). Runs immediately (no confirmation).",
				RuntimeTool: "joanne-read-company", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-skills", Label: "Read Skills", Description: "List team skills from the orchestrator capability catalog (who has which skills). Runs immediately (no confirmation).",
				RuntimeTool: "joanne-read-skills", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-user", Label: "Read User", Description: "Show the message author's Stripe customer id (makeacompany Redis profile), Slack user id, and Slack workspace team id. Runs immediately (no confirmation).",
				RuntimeTool: "joanne-read-user", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-twitter", Label: "Read Twitter", Description: "Search Twitter by keyword and fetch high-impression tweets (not the platform trend list).",
				RuntimeTool: "garth-read-twitter", RequiredParams: []string{"query"}, OptionalParams: []string{"count"},
			},
			{
				ID: "read-trends", Label: "Read Trends", Description: "Fetch the current Twitter/X trend list (not keyword search).",
				RuntimeTool: "garth-read-trends", RequiredParams: []string{}, OptionalParams: []string{},
			},
		},
		EmployeeSkillIDs: map[string][]string{
			"alex":   {},
			"tim":    {},
			"ross":   {},
			"garth":  {"read-twitter", "read-trends"},
			"joanne": {"read-company", "read-skills", "read-user", "create-company", "delete-company", "create-email", "create-doc"},
		},
	}
}
