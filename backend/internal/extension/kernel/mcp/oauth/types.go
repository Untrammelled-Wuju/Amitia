package oauth

import "time"

type MCPProtectedResourceMetadata struct {
	Resource                     string   `json:"resource"`
	AuthorizationServers         []string `json:"authorization_servers"`
	ScopesSupported              []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported       []string `json:"bearer_methods_supported,omitempty"`
	ResourceSigningAlgsSupported []string `json:"resource_signing_alg_values_supported,omitempty"`
	ResourceDocumentation        string   `json:"resource_documentation,omitempty"`
}

type MCPAuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ClientIDMetadataDocumentSupported bool     `json:"client_id_metadata_document_supported,omitempty"`
	AuthorizationResponseISSSupported bool     `json:"authorization_response_iss_parameter_supported,omitempty"`
}

type MCPOAuthCredentialKey struct {
	ServerID string
	Resource string
	Issuer   string
}

type MCPOAuthClientRegistration struct {
	RegistrationMethod      string    `json:"registration_method"`
	Issuer                  string    `json:"issuer"`
	ClientID                string    `json:"client_id"`
	ClientSecretRef         string    `json:"client_secret_ref"`
	RedirectURIs            []string  `json:"redirect_uris"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method"`
	CreatedAt               time.Time `json:"created_at"`
	MetadataFingerprint     string    `json:"metadata_fingerprint"`
}

type MCPOAuthCredentialMetadata struct {
	ServerID            string    `json:"server_id"`
	Resource            string    `json:"resource"`
	Issuer              string    `json:"issuer"`
	SecretRef           string    `json:"secret_ref"`
	Scopes              []string  `json:"scopes"`
	ExpiresAt           time.Time `json:"expires_at"`
	RegistrationMethod  string    `json:"registration_method"`
	ClientIDFingerprint string    `json:"client_id_fingerprint"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type MCPOAuthOperationState string

const (
	MCPOAuthIdle                 MCPOAuthOperationState = "idle"
	MCPOAuthDiscovering          MCPOAuthOperationState = "discovering"
	MCPOAuthRegistrationRequired MCPOAuthOperationState = "registration_required"
	MCPOAuthAuthorizing          MCPOAuthOperationState = "authorizing"
	MCPOAuthCallbackReceived     MCPOAuthOperationState = "callback_received"
	MCPOAuthExchanging           MCPOAuthOperationState = "exchanging"
	MCPOAuthAuthorized           MCPOAuthOperationState = "authorized"
	MCPOAuthScopeUpgradeRequired MCPOAuthOperationState = "scope_upgrade_required"
	MCPOAuthFailed               MCPOAuthOperationState = "failed"
	MCPOAuthCancelled            MCPOAuthOperationState = "cancelled"
	MCPOAuthExpired              MCPOAuthOperationState = "expired"
)

type MCPAuthorizationState string

const (
	MCPAuthNotRequired     MCPAuthorizationState = "not_required"
	MCPAuthRequired        MCPAuthorizationState = "authorization_required"
	MCPAuthAuthorizing     MCPAuthorizationState = "authorizing"
	MCPAuthAuthorized      MCPAuthorizationState = "authorized"
	MCPAuthRefreshDue      MCPAuthorizationState = "refresh_due"
	MCPAuthScopeUpgradeReq MCPAuthorizationState = "scope_upgrade_required"
	MCPAuthRevoked         MCPAuthorizationState = "revoked"
	MCPAuthExpired         MCPAuthorizationState = "expired"
	MCPAuthError           MCPAuthorizationState = "error"
)
