package zeroword

import (
	"context"
	"net/http"
)

type Workspace struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

func (c *Client) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	return get[Workspace](ctx, c, id("/workspaces", workspaceID))
}

type Project struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	DeletedAt   *string `json:"deleted_at"`
}

type ProjectCreate struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug,omitempty"`
}

type ProjectUpdate struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

func (c *Client) CreateProject(ctx context.Context, in ProjectCreate) (*Project, error) {
	return send[Project](ctx, c, http.MethodPost, "/projects", in)
}

func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	return get[Project](ctx, c, id("/projects", projectID))
}

func (c *Client) UpdateProject(ctx context.Context, projectID string, in ProjectUpdate) (*Project, error) {
	return send[Project](ctx, c, http.MethodPatch, id("/projects", projectID), in)
}

func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	return c.do(ctx, http.MethodDelete, id("/projects", projectID), nil, nil)
}

type Environment struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	Type              string `json:"type"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	Issuer            string `json:"issuer"`
	InteractionDomain string `json:"interaction_domain"`
	RPID              string `json:"rp_id"`
}

type EnvironmentCreate struct {
	ProjectID string `json:"project_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
}

type EnvironmentUpdate struct {
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

func (c *Client) CreateEnvironment(ctx context.Context, in EnvironmentCreate) (*Environment, error) {
	return send[Environment](ctx, c, http.MethodPost, "/environments", in)
}

func (c *Client) GetEnvironment(ctx context.Context, environmentID string) (*Environment, error) {
	return get[Environment](ctx, c, id("/environments", environmentID))
}

func (c *Client) UpdateEnvironment(ctx context.Context, environmentID string, in EnvironmentUpdate) (*Environment, error) {
	return send[Environment](ctx, c, http.MethodPatch, id("/environments", environmentID), in)
}

func (c *Client) DeleteEnvironment(ctx context.Context, environmentID string) error {
	return c.do(ctx, http.MethodDelete, id("/environments", environmentID), nil, nil)
}

type EnvironmentSettings struct {
	EnvironmentID        string `json:"environment_id"`
	ImpersonationEnabled bool   `json:"impersonation_enabled"`
}

func (c *Client) GetEnvironmentSettings(ctx context.Context, environmentID string) (*EnvironmentSettings, error) {
	return get[EnvironmentSettings](ctx, c, id("/environments", environmentID)+"/settings")
}

func (c *Client) UpdateEnvironmentSettings(ctx context.Context, environmentID string, impersonationEnabled bool) (*EnvironmentSettings, error) {
	body := map[string]bool{"impersonation_enabled": impersonationEnabled}
	return send[EnvironmentSettings](ctx, c, http.MethodPatch, id("/environments", environmentID)+"/settings", body)
}

// EnvironmentTheme carries the theme document as raw JSON so the provider
// never drops a field it does not model (specs/theme.schema.json).
type EnvironmentTheme struct {
	EnvironmentID string         `json:"environment_id"`
	Version       int64          `json:"version"`
	Theme         map[string]any `json:"theme"`
}

func (c *Client) GetEnvironmentTheme(ctx context.Context, environmentID string) (*EnvironmentTheme, error) {
	return get[EnvironmentTheme](ctx, c, id("/environments", environmentID)+"/theme")
}

func (c *Client) PutEnvironmentTheme(ctx context.Context, environmentID string, theme map[string]any) (*EnvironmentTheme, error) {
	return send[EnvironmentTheme](ctx, c, http.MethodPut, id("/environments", environmentID)+"/theme", theme)
}

type MobileApp struct {
	AppScheme                 string   `json:"app_scheme"`
	IOSBundleID               *string  `json:"ios_bundle_id"`
	IOSTeamID                 *string  `json:"ios_team_id"`
	AndroidPackageName        *string  `json:"android_package_name"`
	AndroidSHA256Fingerprints []string `json:"android_sha256_fingerprints"`
	UniversalLinkHost         *string  `json:"universal_link_host"`
}

// OAuthClientSettings are the fields shared by the create and update bodies.
type OAuthClientSettings struct {
	Name                    string   `json:"name"`
	RedirectURIs            []string `json:"redirect_uris"`
	PostLogoutRedirectURIs  []string `json:"post_logout_redirect_uris"`
	AllowedOrigins          []string `json:"allowed_origins"`
	AllowedScopes           []string `json:"allowed_scopes"`
	AllowedResources        []string `json:"allowed_resources"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	RequirePKCE             bool     `json:"require_pkce"`
	RequireNonce            bool     `json:"require_nonce"`
}

type OAuthClient struct {
	OAuthClientSettings
	ID            string     `json:"id"`
	EnvironmentID string     `json:"environment_id"`
	ClientType    string     `json:"client_type"`
	Status        string     `json:"status"`
	MobileApp     *MobileApp `json:"mobile_app"`
	// ClientSecret is only present in the create response of a confidential client.
	ClientSecret string `json:"client_secret,omitempty"`
}

type OAuthClientCreate struct {
	OAuthClientSettings
	EnvironmentID string     `json:"environment_id"`
	ClientType    string     `json:"client_type"`
	MobileApp     *MobileApp `json:"mobile_app,omitempty"`
}

// OAuthClientUpdate sends mobile_app even when nil: an explicit null clears it.
type OAuthClientUpdate struct {
	OAuthClientSettings
	Status    string     `json:"status,omitempty"`
	MobileApp *MobileApp `json:"mobile_app"`
}

func (c *Client) CreateClient(ctx context.Context, in OAuthClientCreate) (*OAuthClient, error) {
	return send[OAuthClient](ctx, c, http.MethodPost, "/clients", in)
}

func (c *Client) GetClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	return get[OAuthClient](ctx, c, id("/clients", clientID))
}

func (c *Client) UpdateClient(ctx context.Context, clientID string, in OAuthClientUpdate) (*OAuthClient, error) {
	return send[OAuthClient](ctx, c, http.MethodPatch, id("/clients", clientID), in)
}

func (c *Client) DeleteClient(ctx context.Context, clientID string) error {
	return c.do(ctx, http.MethodDelete, id("/clients", clientID), nil, nil)
}

type Connection struct {
	ID               string            `json:"id"`
	EnvironmentID    string            `json:"environment_id"`
	Provider         string            `json:"provider"`
	Mode             string            `json:"mode"`
	ProviderKey      string            `json:"provider_key"`
	Issuer           *string           `json:"issuer"`
	ProviderClientID string            `json:"provider_client_id"`
	Scopes           []string          `json:"scopes"`
	ProfileMapping   map[string]string `json:"profile_mapping"`
	Status           string            `json:"status"`
	OwnerConfirmed   bool              `json:"owner_confirmed"`
}

type ConnectionCreate struct {
	EnvironmentID        string            `json:"environment_id"`
	Provider             string            `json:"provider"`
	Mode                 string            `json:"mode"`
	ProviderKey          string            `json:"provider_key"`
	Issuer               *string           `json:"issuer,omitempty"`
	ProviderClientID     string            `json:"provider_client_id"`
	ProviderClientSecret string            `json:"provider_client_secret"`
	Scopes               []string          `json:"scopes"`
	ProfileMapping       map[string]string `json:"profile_mapping"`
}

type ConnectionUpdate struct {
	Issuer               *string           `json:"issuer,omitempty"`
	ProviderClientID     string            `json:"provider_client_id"`
	ProviderClientSecret string            `json:"provider_client_secret,omitempty"`
	Scopes               []string          `json:"scopes"`
	ProfileMapping       map[string]string `json:"profile_mapping"`
	Status               string            `json:"status,omitempty"`
	OwnerConfirmed       *bool             `json:"owner_confirmed,omitempty"`
}

func (c *Client) CreateConnection(ctx context.Context, in ConnectionCreate) (*Connection, error) {
	return send[Connection](ctx, c, http.MethodPost, "/connections", in)
}

func (c *Client) GetConnection(ctx context.Context, connectionID string) (*Connection, error) {
	return get[Connection](ctx, c, id("/connections", connectionID))
}

func (c *Client) UpdateConnection(ctx context.Context, connectionID string, in ConnectionUpdate) (*Connection, error) {
	return send[Connection](ctx, c, http.MethodPatch, id("/connections", connectionID), in)
}

func (c *Client) DeleteConnection(ctx context.Context, connectionID string) error {
	return c.do(ctx, http.MethodDelete, id("/connections", connectionID), nil, nil)
}

type APIKey struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspace_id"`
	EnvironmentID *string  `json:"environment_id"`
	Name          string   `json:"name"`
	Prefix        string   `json:"prefix"`
	Scopes        []string `json:"scopes"`
	ExpiresAt     *string  `json:"expires_at"`
	RevokedAt     *string  `json:"revoked_at"`
	// Secret is only present in the create response.
	Secret string `json:"secret,omitempty"`
}

type APIKeyCreate struct {
	WorkspaceID   string   `json:"workspace_id"`
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	EnvironmentID *string  `json:"environment_id,omitempty"`
	ExpiresAt     *string  `json:"expires_at,omitempty"`
}

func (c *Client) CreateAPIKey(ctx context.Context, in APIKeyCreate) (*APIKey, error) {
	return send[APIKey](ctx, c, http.MethodPost, "/api-keys", in)
}

func (c *Client) GetAPIKey(ctx context.Context, apiKeyID string) (*APIKey, error) {
	return get[APIKey](ctx, c, id("/api-keys", apiKeyID))
}

func (c *Client) RevokeAPIKey(ctx context.Context, apiKeyID string) error {
	return c.do(ctx, http.MethodPost, id("/api-keys", apiKeyID)+"/revoke", nil, nil)
}

type Domain struct {
	ID                      string  `json:"id"`
	EnvironmentID           string  `json:"environment_id"`
	Hostname                string  `json:"hostname"`
	Status                  string  `json:"status"`
	OwnershipVerified       bool    `json:"ownership_verified"`
	VerificationRecordName  *string `json:"verification_record_name"`
	VerificationRecordValue *string `json:"verification_record_value"`
	RPEnabled               bool    `json:"rp_enabled"`
}

type DomainCreate struct {
	EnvironmentID string `json:"environment_id"`
	Hostname      string `json:"hostname"`
	RPEnabled     bool   `json:"rp_enabled"`
}

func (c *Client) CreateDomain(ctx context.Context, in DomainCreate) (*Domain, error) {
	return send[Domain](ctx, c, http.MethodPost, "/domains", in)
}

func (c *Client) GetDomain(ctx context.Context, domainID string) (*Domain, error) {
	return get[Domain](ctx, c, id("/domains", domainID))
}

func (c *Client) UpdateDomain(ctx context.Context, domainID string, rpEnabled bool) (*Domain, error) {
	return send[Domain](ctx, c, http.MethodPatch, id("/domains", domainID), map[string]bool{"rp_enabled": rpEnabled})
}

func (c *Client) DeleteDomain(ctx context.Context, domainID string) error {
	return c.do(ctx, http.MethodDelete, id("/domains", domainID), nil, nil)
}

type Webhook struct {
	ID            string   `json:"id"`
	EnvironmentID string   `json:"environment_id"`
	URL           string   `json:"url"`
	EventTypes    []string `json:"event_types"`
	Status        string   `json:"status"`
	// Secret is only present in the create response.
	Secret string `json:"secret,omitempty"`
}

type WebhookCreate struct {
	EnvironmentID string   `json:"environment_id"`
	URL           string   `json:"url"`
	EventTypes    []string `json:"event_types"`
}

type WebhookUpdate struct {
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
	Status     string   `json:"status,omitempty"`
}

func (c *Client) CreateWebhook(ctx context.Context, in WebhookCreate) (*Webhook, error) {
	return send[Webhook](ctx, c, http.MethodPost, "/webhooks", in)
}

func (c *Client) GetWebhook(ctx context.Context, webhookID string) (*Webhook, error) {
	return get[Webhook](ctx, c, id("/webhooks", webhookID))
}

func (c *Client) UpdateWebhook(ctx context.Context, webhookID string, in WebhookUpdate) (*Webhook, error) {
	return send[Webhook](ctx, c, http.MethodPatch, id("/webhooks", webhookID), in)
}

func (c *Client) DeleteWebhook(ctx context.Context, webhookID string) error {
	return c.do(ctx, http.MethodDelete, id("/webhooks", webhookID), nil, nil)
}
