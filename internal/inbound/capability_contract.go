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
				ID: "create-company", Label: "Create Company", Description: "Start a private company channel from a name (slug); founders default to you plus @mentioned cofounders.",
				RuntimeTool: "joanne-create-company", RequiredParams: []string{"name"}, OptionalParams: []string{"founders"},
				ParamDefaults: map[string]string{
					"name":     "Company / channel slug (gathered in-thread when not in the first message)",
					"founders": "Optional; when omitted defaults to the message author plus any @mentioned cofounders",
				},
			},
			{
				ID: "create-connect", Label: "Create Connect", Description: "Generate a Slack Connect setup link for the current company channel. No confirm/cancel flow.",
				RuntimeTool: "joanne-create-connect", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "delete-company", Label: "Delete Company", Description: "Removes a company and sends it to the archive. Requires confirmation.",
				RuntimeTool: "joanne-delete-company", RequiredParams: []string{"name"}, OptionalParams: []string{},
				ParamDefaults: map[string]string{
					"channel": "The Slack channel where the command runs (implicit default; operators do not pass this at runtime)",
				},
			},
			{
				ID: "read-company", Label: "Read Company", Description: "Summarize the latest activity within the company.",
				RuntimeTool: "joanne-read-company", RequiredParams: []string{}, OptionalParams: []string{},
			},
			{
				ID: "read-internet", Label: "Read Internet", Description: "Search the public web (internet) for current events and external references.",
				RuntimeTool: "joanne-read-internet", RequiredParams: []string{"query"}, OptionalParams: []string{"count"},
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
				ID: "update-terms", Label: "Update Terms", Description: "Show platform terms of use; record I Agree / I Do Not Agree on the operator profile (same confirm control as #humans onboarding).",
				RuntimeTool: "joanne-update-terms", RequiredParams: []string{}, OptionalParams: []string{},
			},
		},
		EmployeeSkillIDs: map[string][]string{
			"alex":   {"read-internet"},
			"tim":    {"read-internet"},
			"ross":   {"read-internet"},
			"garth":  {"read-twitter", "read-trends", "read-internet"},
			"joanne": {"read-company", "read-internet", "read-skills", "read-user", "create-company", "create-connect", "delete-company", "create-email", "create-doc", "update-terms"},
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
