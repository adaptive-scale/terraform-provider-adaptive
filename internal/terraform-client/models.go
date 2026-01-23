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
}

type CreateSessionResponse struct {
	ID string `json:"id"`
}

type UpdateSessionRequest = CreateSessionRequest

// type UpdateSessionRequest struct {
// 	SessionName       string `json:"sessionName"`
// 	ClusterName       string `json:"clusterName"`
// 	AuthorizationName string `json:"authorizationName"`
// 	SessionTTL        string `json:"sessionTTL"`
// }

// type UpdateSessionRequest struct {
// 	*CreateSessionRequest
// }

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

type ErrorResponse struct {
	Error string
	Msg   string `json:",omitempty"`
}

type DefaultResponse struct {
	Status string
}
