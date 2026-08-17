package client

type CreateResourceRequest struct {
	IntegrationType string   `json:"integrationType"`
	Name            string   `json:"name"`
	Configuration   string   `json:"config"`
	UserTags        []string `json:"userTags"`
	DefaultCluster  string   `json:"defaultCluster,omitempty"`
}

type UpdateResourceRequest struct {
	IntegrationType string   `json:"integrationType"`
	Configuration   string   `json:"config"`
	UserTags        []string `json:"userTags"`
	DefaultCluster  string   `json:"defaultCluster,omitempty"`
}

type CreateResourceResponse struct {
	ID string `json:"id"`
}

type UpdateResourceResponse struct {
	ID string `json:"id"`
}

const (
	PostgresIntegrationType = "postgres"
)

// sessions
type CreateSessionRequest struct {
	SessionName       string `json:"sessionName"`
	ResourceName      string `json:"resourceName"`
	ClusterName       string `json:"clusterName,omitempty"`
	AuthorizationName string `json:"authorizationName,omitempty"`
	SessionTTL        string `json:"sessionTTL,omitempty"`
	SessionType       string `json:"sessionType"`
	// List of user emails to add to the endpoint
	SessionUsers []string `json:"sessionUsers,omitempty"`
	// endpoint JIT access mode
	IsJITEnabled    bool     `json:"is_jit_enabled"`
	AccessApprovers []string `json:"access_approvers"`

	Memory    string   `json:"memory"`
	CPU       string   `json:"cpu"`
	UsersTags []string `json:"usertags"`

	// pause endpoint timeout
	PauseTimeout string   `json:"pause_timeout,omitempty"`
	Groups       []string `json:"groups,omitempty"`
	IdleTimeout  string   `json:"idle_timeout,omitempty"`

	// script only access
	ScriptOnlyAccess bool `json:"script_only_access"`

	// session recording / UI surface toggles
	DisableOutputCapture bool `json:"disable_output_capture"`
	DisableDataStudio    bool `json:"disable_data_studio"`
	DisableWebCLI        bool `json:"disable_web_cli"`

	// JIT approval tuning. These are pointers so an unset input is omitted and
	// the server keeps its own default rather than being reset to the zero value.
	JITMode           string `json:"jit_mode,omitempty"`
	AutoApproval      *bool  `json:"auto_approval,omitempty"`
	JITMultiApprover  *bool  `json:"jit_multi_approver,omitempty"`
	JITTotalApprovers *int   `json:"jit_total_approvers,omitempty"`
}

type CreateSessionResponse struct {
	ID string `json:"id"`
}

type UpdateSessionRequest = CreateSessionRequest

type UpdateSessionResponse struct {
	ID string `json:"id"`
}

type UpdateAuthorizationRequest struct {
	AuthorizationName        string `json:"name"`
	AuthorizationDescription string `json:"description"`
	ResourceType             string `json:"resourceType"`
	Permissions              string `json:"permissions"`
}

type UpdateAuthorizationResponse struct {
	ID string `json:"id"`
}

// ScriptRequest is the payload consumed by the script create/update routes. The
// backend binds these with PascalCase keys.
type ScriptRequest struct {
	Name                  string            `json:"Name"`
	Command               string            `json:"Command"`
	Endpoint              string            `json:"Endpoint"`
	Description           string            `json:"Description,omitempty"`
	ParameterDescriptions map[string]string `json:"ParameterDescriptions,omitempty"`
}

// DataProtectionRequest mirrors the server's TerraformDataProtectionRequest.
type DataProtectionRequest struct {
	ResourceName string               `json:"resourceName"`
	Scoped       *bool                `json:"scoped,omitempty"` // nil means true (server default)
	Masks        []DataProtectionMask `json:"masks"`
}

type DataProtectionMask struct {
	DatabaseName string                `json:"databaseName"`
	Tables       []DataProtectionTable `json:"tables"`
}

type DataProtectionTable struct {
	TableName     string                     `json:"tableName"`
	Schema        string                     `json:"schema,omitempty"`
	Masked        *bool                      `json:"masked,omitempty"`
	MaskedColumns []DataProtectionColumnMask `json:"maskedColumns"`
}

type DataProtectionColumnMask struct {
	ColumnName  string `json:"columnName"`
	MaskingType string `json:"maskingType"`
}

type ErrorResponse struct {
	Error string
	Msg   string `json:",omitempty"`
}

type DefaultResponse struct {
	Status string
}

// ---------------------------------------------------------------------------
// Read models — the canonical camelCase keys of the /terraform/<type>/read/:id
// DTOs. Decoding into these instead of map[string]interface{} keeps every read
// path free of unchecked type assertions.
// ---------------------------------------------------------------------------

type ResourceReadResponse struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	IntegrationType string         `json:"integrationType"`
	UserTags        []string       `json:"userTags"`
	DefaultCluster  string         `json:"defaultCluster"`
	Configuration   map[string]any `json:"configuration"` // secret values are stripped server-side
	RedactedKeys    []string       `json:"redactedKeys"`
	// RedactedDigests carries an opaque server-side fingerprint per redacted key
	// so secret drift can be detected without the value ever crossing the wire.
	// Absent on servers that predate the feature.
	RedactedDigests map[string]string `json:"redactedDigests"`
	Status          string            `json:"status"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
}

type SessionReadResponse struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Resource             string   `json:"resource"` // integration name, not id
	Cluster              string   `json:"cluster"`
	Authorization        string   `json:"authorization"`
	Status               string   `json:"status"`
	SessionType          string   `json:"sessionType"` // cli | client | services
	TTL                  string   `json:"ttl"`
	IdleTimeout          string   `json:"idleTimeout"`
	PauseTimeout         string   `json:"pauseTimeout"`
	Memory               string   `json:"memory"`
	CPU                  string   `json:"cpu"`
	Storage              string   `json:"storage"`
	IsJITEnabled         bool     `json:"isJitEnabled"`
	JITMode              string   `json:"jitMode"`
	JITMultiApprover     bool     `json:"jitMultiApprover"`
	JITTotalApprovers    int      `json:"jitTotalApprovers"`
	AutoApproval         bool     `json:"autoApproval"`
	SessionUsers         []string `json:"sessionUsers"`
	AccessApprovers      []string `json:"accessApprovers"`
	Groups               []string `json:"groups"`
	UserTags             []string `json:"userTags"`
	ScriptOnlyAccess     bool     `json:"scriptOnlyAccess"`
	DisableOutputCapture bool     `json:"disableOutputCapture"`
	DisableDataStudio    bool     `json:"disableDataStudio"`
	DisableWebCLI        bool     `json:"disableWebCli"`
	Public               bool     `json:"public"`
	Exposed              bool     `json:"exposed"`
	ExposeType           string   `json:"exposeType"`
	ExposeStatus         string   `json:"exposeStatus"`
}

type AuthorizationReadResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ResourceType string `json:"resourceType"`
	Permissions  string `json:"permissions"`
	Status       string `json:"status"`
	Type         string `json:"type"`
}

type TeamReadResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Members        []string `json:"members"`   // emails
	Endpoints      []string `json:"endpoints"` // endpoint names
	SlackChannelID string   `json:"slackChannelId"`
	CreatedBy      string   `json:"createdBy"`
	CreatedAt      string   `json:"createdAt"`
}

type ScriptReadResponse struct {
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	Endpoint              string            `json:"endpoint"` // endpoint name (resolved)
	Description           string            `json:"description"`
	ParameterDescriptions map[string]string `json:"parameterDescriptions"`
	IsAutoGen             bool              `json:"isAutoGen"`
	CommandOmitted        bool              `json:"commandOmitted"` // command is write-only
	// CommandDigest is an opaque server-side fingerprint of the script body,
	// used to detect out-of-band command changes. Absent on older servers.
	CommandDigest string `json:"commandDigest"`
	CreatedBy     string `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
}

type DataProtectionReadResponse struct {
	ID                string               `json:"id"` // the resource (integration) id
	ResourceName      string               `json:"resourceName"`
	AuthorizationName string               `json:"authorizationName"` // "masked_<resourceName>"
	Scoped            *bool                `json:"scoped"`
	Masks             []DataProtectionMask `json:"masks"`
	Status            string               `json:"status"`
	LastAppliedAt     string               `json:"lastAppliedAt"`
}
