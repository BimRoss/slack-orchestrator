package inbound

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
)

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
	raw, _, _ := defaultContractCached()
	return append(json.RawMessage(nil), raw...)
}

// DefaultCapabilityContractRevision returns the revision tag for the default capability contract.
func DefaultCapabilityContractRevision() string {
	_, revision, _ := defaultContractCached()
	return revision
}

// DefaultCapabilityContractDigest returns a stable short hash of the default contract JSON bytes.
func DefaultCapabilityContractDigest() string {
	_, _, digest := defaultContractCached()
	return digest
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
			{ID: "anna", Label: "Anna", Description: "Head of Creative specializing in image concepts and generation workflows."},
		},
		Skills: []CapabilitySkillV1{
			{
				ID: "create-email", Label: "Create Email", Description: "Design and send email, to one or a hundred. Bulk concurrency handled, HTML supported natively. Requires confirmation before send.",
				RuntimeTool: "joanne-create-email", RequiredParams: []string{"intent", "to", "subject"},
				OptionalParams: []string{"button", "link"},
				ParamDefaults: map[string]string{
					"to":     "Message author (Slack profile; makeacompany slack→email index when configured)",
					"button": "none",
					"link":   "none",
				},
			},
			{
				ID: "create-email-welcome", Label: "Create Email (Welcome)", Description: "Send the standard MakeACompany welcome email. Wraps create-email with a fixed subject, one short welcoming paragraph, and a Join our Company button. Requires confirmation before send.",
				RuntimeTool: "joanne-create-email-welcome", RequiredParams: []string{"name", "email"},
				OptionalParams: []string{},
				Requires:       []string{"google_oauth"},
			},
			{
				ID: "create-doc", Label: "Create Doc", Description: "Create Google documents, outlines, and game plans. Pair with search skills to produce research documents in seconds.",
				RuntimeTool: "joanne-create-doc", RequiredParams: []string{"intent", "title", "editors"},
				OptionalParams: []string{"commenters", "viewers", "type", "length"},
				ParamDefaults: map[string]string{
					"title":      "Derived from intent when omitted; runtime infers a working title before draft",
					"editors":    "Message author email (implicit default); append @mentions or explicit editor emails",
					"type":       "outline",
					"length":     "Defaults to one page when omitted",
					"commenters": "none",
					"viewers":    "none",
				},
			},
			{
				ID: "create-image", Label: "Create Image", Description: "Generate an original image from a text prompt using Anna's creative workflow.",
				RuntimeTool: "anna-create-image", RequiredParams: []string{"intent"}, OptionalParams: []string{"style", "ratio", "size"},
			},
			{
				ID: "create-company", Label: "Create Company", Description: "Start a private company channel from a name (slug); founders default to you plus @mentioned cofounders.",
				RuntimeTool: "joanne-create-company", RequiredParams: []string{"name"}, OptionalParams: []string{"founders"},
				ParamDefaults: map[string]string{
					"name":     "Company / channel slug (gathered in-thread when not in the first message)",
					"founders": "Optional; when omitted defaults to the message author plus any @mentioned cofounders",
				},
			},
			{
				ID: "create-connect", Label: "Create Connect", Description: "Generate a Slack Connect setup link for the current company channel to invite others, or have your MakeACompany channel alongside other's in your own workspace.",
				RuntimeTool:    "joanne-create-connect",
				RequiredParams: []string{},
				OptionalParams: []string{"emails"},
				ParamDefaults: map[string]string{
					"emails": "Optional. When omitted, the runtime uses the message author's Slack email, any email addresses in the message text, and emails resolved from @mentioned users (same behavior as if you only @mention people or type addresses inline).",
				},
			},
			{
				ID: "create-issue", Label: "Create Issue", Description: "Create a company issue with a title and body from the Slack thread. New issues land in Backlog on the team board. Requires confirmation before publish.",
				RuntimeTool:    "ross-create-issue",
				RequiredParams: []string{"body", "title"},
				OptionalParams: []string{},
			},
			{
				ID: "read-issue", Label: "Read Issue", Description: "List GitHub project issue cards by workflow lane (Backlog, In Progress, Done).",
				RuntimeTool: "ross-read-issue", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-backend", Label: "Read Backend", Description: "Read the makeacompany backend admin JSON surfaces (health, catalog, users, company channels, and related diagnostics).",
				RuntimeTool: "ross-read-backend", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "update-issue", Label: "Update Issue", Description: "Update a company issue, including the title, body, or status.  Requires confirmation before write.",
				RuntimeTool: "ross-update-issue", RequiredParams: []string{"number"}, OptionalParams: []string{"body", "title", "status"},
				ParamDefaults: map[string]string{
					"status": "Match an existing Project Status option (for example Backlog, In Progress, Done)",
				},
			},
			{
				ID: "delete-company", Label: "Delete Company", Description: "Removes a company and sends it to the archive. Requires confirmation.",
				RuntimeTool: "joanne-delete-company", RequiredParams: []string{"name"}, OptionalParams: []string{},
				ParamDefaults: map[string]string{
					"channel": "The Slack channel where the command runs (implicit default; operators do not pass this at runtime)",
				},
			},
			{
				ID: "update-company", Label: "Update Company", Description: "Rename the current company Slack channel (the channel where the message is sent) and sync the new slug/display name in app registry. Required field name is the new channel slug. Requires explicit confirmation before rename.",
				RuntimeTool: "joanne-update-company", RequiredParams: []string{"name"}, OptionalParams: []string{},
				ParamDefaults: map[string]string{
					"name": "New Slack channel name (slug) for the current channel; gathered in-thread when not in the first message",
				},
			},
			{
				ID: "read-company", Label: "Read Company", Description: "Summarize the latest activity within the company.",
				RuntimeTool: "joanne-read-company", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-web", Label: "Read Web", Description: "Search the public web (internet) for current events and external references.",
				RuntimeTool: "joanne-read-web", RequiredParams: []string{"query"}, OptionalParams: []string{"count"},
			},
			{
				ID: "read-skills", Label: "Read Skills", Description: "Display the skills of the team",
				RuntimeTool: "joanne-read-skills", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-user", Label: "Read User", Description: "Display a user's company card.",
				RuntimeTool: "joanne-read-user", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-twitter", Label: "Read Twitter", Description: "Search twitter for high-impression tweets on any topic",
				RuntimeTool: "garth-read-twitter", RequiredParams: []string{"query"}, OptionalParams: []string{"count"},
			},
			{
				ID: "read-trends", Label: "Read Trends", Description: "Show the latest trends",
				RuntimeTool: "garth-read-trends", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "update-terms", Label: "Update Terms", Description: "Show platform terms of use for users to agree (or not). Automatically recorded",
				RuntimeTool: "joanne-update-terms", RequiredParams: []string{}, OptionalParams: []string{},
			},
		},
		EmployeeSkillIDs: map[string][]string{
			"alex":   {"read-web"},
			"tim":    {"read-web"},
			"ross":   {"read-web", "create-issue", "read-issue", "read-backend", "update-issue"},
			"garth":  {"read-twitter", "read-trends", "read-web"},
			"joanne": {"read-company", "read-web", "read-skills", "read-user", "create-company", "create-connect", "delete-company", "update-company", "create-email", "create-email-welcome", "create-doc", "update-terms"},
			"anna":   {"create-image", "read-web"},
		},
	}
}

var (
	defaultContractOnce     sync.Once
	defaultContractJSON     json.RawMessage
	defaultContractRevision string
	defaultContractDigest   string
)

func defaultContractCached() (json.RawMessage, string, string) {
	defaultContractOnce.Do(func() {
		c := DefaultCapabilityContractV1()
		defaultContractRevision = strings.TrimSpace(c.Revision)
		raw, err := json.Marshal(c)
		if err == nil {
			defaultContractJSON = raw
			sum := sha256.Sum256(raw)
			defaultContractDigest = hex.EncodeToString(sum[:8])
		}
	})
	return defaultContractJSON, defaultContractRevision, defaultContractDigest
}
