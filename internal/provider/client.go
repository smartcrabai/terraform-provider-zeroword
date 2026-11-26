package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type clientResource struct{ apiResource }

type clientModel struct {
	ID                      types.String `tfsdk:"id"`
	EnvironmentID           types.String `tfsdk:"environment_id"`
	ClientType              types.String `tfsdk:"client_type"`
	Name                    types.String `tfsdk:"name"`
	RedirectURIs            types.Set    `tfsdk:"redirect_uris"`
	PostLogoutRedirectURIs  types.Set    `tfsdk:"post_logout_redirect_uris"`
	AllowedOrigins          types.Set    `tfsdk:"allowed_origins"`
	AllowedScopes           types.Set    `tfsdk:"allowed_scopes"`
	AllowedResources        types.Set    `tfsdk:"allowed_resources"`
	TokenEndpointAuthMethod types.String `tfsdk:"token_endpoint_auth_method"`
	RequirePKCE             types.Bool   `tfsdk:"require_pkce"`
	RequireNonce            types.Bool   `tfsdk:"require_nonce"`
	Status                  types.String `tfsdk:"status"`
	MobileApp               types.Object `tfsdk:"mobile_app"`
	ClientSecret            types.String `tfsdk:"client_secret"`
}

type mobileAppModel struct {
	AppScheme                 types.String `tfsdk:"app_scheme"`
	IOSBundleID               types.String `tfsdk:"ios_bundle_id"`
	IOSTeamID                 types.String `tfsdk:"ios_team_id"`
	AndroidPackageName        types.String `tfsdk:"android_package_name"`
	AndroidSHA256Fingerprints types.Set    `tfsdk:"android_sha256_fingerprints"`
	UniversalLinkHost         types.String `tfsdk:"universal_link_host"`
}

var mobileAppAttrTypes = map[string]attr.Type{
	"app_scheme":                  types.StringType,
	"ios_bundle_id":               types.StringType,
	"ios_team_id":                 types.StringType,
	"android_package_name":        types.StringType,
	"android_sha256_fingerprints": types.SetType{ElemType: types.StringType},
	"universal_link_host":         types.StringType,
}

func newClientResource() resource.Resource { return &clientResource{} }

func (r *clientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client"
}

func stringSetAttribute(description string) schema.SetAttribute {
	return schema.SetAttribute{
		Description: description + " Defaults to empty.",
		ElementType: types.StringType,
		Optional:    true,
		Computed:    true,
		Default:     setdefault.StaticValue(stringSet(nil)),
	}
}

func optionalString(description string) schema.StringAttribute {
	return schema.StringAttribute{Description: description, Optional: true}
}

func (r *clientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	clientType := immutableString("`web_bff`, `web_spa`, or `react_native`.", true)
	clientType.Validators = []validator.String{stringvalidator.OneOf("web_bff", "web_spa", "react_native")}
	resp.Schema = schema.Schema{
		Description: "An OAuth/OIDC client of an Environment.",
		Attributes: map[string]schema.Attribute{
			"id":                        idAttribute("Client ID."),
			"environment_id":            immutableString("Environment ID (`env_...`).", true),
			"client_type":               clientType,
			"name":                      schema.StringAttribute{Description: "Display name (1-200 characters).", Required: true},
			"redirect_uris":             stringSetAttribute("Allowed redirect URIs (max 50)."),
			"post_logout_redirect_uris": stringSetAttribute("Allowed post-logout redirect URIs (max 50)."),
			"allowed_origins":           stringSetAttribute("Allowed CORS origins (max 50)."),
			"allowed_scopes":            stringSetAttribute("Scopes the client may request (max 50)."),
			"allowed_resources":         stringSetAttribute("HTTPS resource indicators (audiences) the client may request (max 20)."),
			"token_endpoint_auth_method": schema.StringAttribute{
				Description: "`none` (public client), `client_secret_basic`, or `client_secret_post`.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("none", "client_secret_basic", "client_secret_post")},
			},
			"require_pkce": schema.BoolAttribute{
				Description: "Kept for compatibility; Zeroword requires PKCE S256 for every client. Defaults to `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"require_nonce": schema.BoolAttribute{
				Description: "Whether an `openid` authorization request must carry a `nonce`. Set `false` only for public clients that cannot send one (for example MCP clients); PKCE S256, `state` and the registered redirect URI stay required. Requires Zeroword 0.7.0 or later. Defaults to `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				Description: "`active` or `disabled`. Defaults to `active`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("active"),
				Validators:  []validator.String{stringvalidator.OneOf("active", "disabled")},
			},
			"mobile_app": schema.SingleNestedAttribute{
				Description: "Native app settings of a `react_native` client.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"app_scheme":           schema.StringAttribute{Description: "Custom URL scheme.", Required: true},
					"ios_bundle_id":        optionalString("iOS bundle ID."),
					"ios_team_id":          optionalString("Apple Team ID."),
					"android_package_name": optionalString("Android package name."),
					"android_sha256_fingerprints": schema.SetAttribute{
						Description: "SHA-256 signing certificate fingerprints of the Android app, colon-separated uppercase hex " +
							"(Zeroword stores the uppercase form, so lowercase input shows a diff on every plan).",
						ElementType: types.StringType,
						Optional:    true,
					},
					"universal_link_host": optionalString("Host for iOS Universal Links / Android App Links."),
				},
			},
			"client_secret": schema.StringAttribute{
				Description: "Client secret of a confidential client. Zeroword returns it only at creation, " +
					"so it is null after import and for public clients.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (m *clientModel) settings(ctx context.Context, diags *diag.Diagnostics) zeroword.OAuthClientSettings {
	return zeroword.OAuthClientSettings{
		Name:                    m.Name.ValueString(),
		RedirectURIs:            stringSlice(ctx, m.RedirectURIs, diags),
		PostLogoutRedirectURIs:  stringSlice(ctx, m.PostLogoutRedirectURIs, diags),
		AllowedOrigins:          stringSlice(ctx, m.AllowedOrigins, diags),
		AllowedScopes:           stringSlice(ctx, m.AllowedScopes, diags),
		AllowedResources:        stringSlice(ctx, m.AllowedResources, diags),
		TokenEndpointAuthMethod: m.TokenEndpointAuthMethod.ValueString(),
		RequirePKCE:             m.RequirePKCE.ValueBool(),
		RequireNonce:            m.RequireNonce.ValueBool(),
	}
}

func (m *clientModel) mobileApp(ctx context.Context, diags *diag.Diagnostics) *zeroword.MobileApp {
	if m.MobileApp.IsNull() || m.MobileApp.IsUnknown() {
		return nil
	}
	var app mobileAppModel
	diags.Append(m.MobileApp.As(ctx, &app, basetypes.ObjectAsOptions{})...)
	return &zeroword.MobileApp{
		AppScheme:                 app.AppScheme.ValueString(),
		IOSBundleID:               stringPointer(app.IOSBundleID),
		IOSTeamID:                 stringPointer(app.IOSTeamID),
		AndroidPackageName:        stringPointer(app.AndroidPackageName),
		AndroidSHA256Fingerprints: stringSlice(ctx, app.AndroidSHA256Fingerprints, diags),
		UniversalLinkHost:         stringPointer(app.UniversalLinkHost),
	}
}

func (m *clientModel) set(c *zeroword.OAuthClient, diags *diag.Diagnostics) {
	m.ID = types.StringValue(c.ID)
	m.EnvironmentID = types.StringValue(c.EnvironmentID)
	m.ClientType = types.StringValue(c.ClientType)
	m.Name = types.StringValue(c.Name)
	m.RedirectURIs = stringSet(c.RedirectURIs)
	m.PostLogoutRedirectURIs = stringSet(c.PostLogoutRedirectURIs)
	m.AllowedOrigins = stringSet(c.AllowedOrigins)
	m.AllowedScopes = stringSet(c.AllowedScopes)
	m.AllowedResources = stringSet(c.AllowedResources)
	m.TokenEndpointAuthMethod = types.StringValue(c.TokenEndpointAuthMethod)
	m.RequirePKCE = types.BoolValue(c.RequirePKCE)
	m.RequireNonce = types.BoolValue(c.RequireNonce)
	m.Status = types.StringValue(c.Status)
	if c.MobileApp == nil {
		m.MobileApp = types.ObjectNull(mobileAppAttrTypes)
		return
	}
	// An empty fingerprint list from the API maps back to an omitted attribute.
	fingerprints := types.SetNull(types.StringType)
	if len(c.MobileApp.AndroidSHA256Fingerprints) > 0 {
		fingerprints = stringSet(c.MobileApp.AndroidSHA256Fingerprints)
	}
	object, d := types.ObjectValue(mobileAppAttrTypes, map[string]attr.Value{
		"app_scheme":                  types.StringValue(c.MobileApp.AppScheme),
		"ios_bundle_id":               types.StringPointerValue(c.MobileApp.IOSBundleID),
		"ios_team_id":                 types.StringPointerValue(c.MobileApp.IOSTeamID),
		"android_package_name":        types.StringPointerValue(c.MobileApp.AndroidPackageName),
		"android_sha256_fingerprints": fingerprints,
		"universal_link_host":         types.StringPointerValue(c.MobileApp.UniversalLinkHost),
	})
	diags.Append(d...)
	m.MobileApp = object
}

func (r *clientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := zeroword.OAuthClientCreate{
		OAuthClientSettings: plan.settings(ctx, &resp.Diagnostics),
		EnvironmentID:       plan.EnvironmentID.ValueString(),
		ClientType:          plan.ClientType.ValueString(),
		MobileApp:           plan.mobileApp(ctx, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateClient(ctx, body)
	if err != nil {
		apiError(&resp.Diagnostics, "create client", err)
		return
	}
	desiredStatus := plan.Status.ValueString()
	plan.set(created, &resp.Diagnostics)
	plan.ClientSecret = types.StringNull()
	if created.ClientSecret != "" {
		plan.ClientSecret = types.StringValue(created.ClientSecret)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if created.Status == desiredStatus || resp.Diagnostics.HasError() {
		return
	}
	// The create body has no status; disable in a second call.
	updated, err := r.client.UpdateClient(ctx, created.ID, zeroword.OAuthClientUpdate{
		OAuthClientSettings: created.OAuthClientSettings,
		Status:              desiredStatus,
		MobileApp:           created.MobileApp,
	})
	if err != nil {
		apiError(&resp.Diagnostics, "set client status", err)
		return
	}
	plan.set(updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client, err := r.client.GetClient(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read client", err)
		return
	}
	state.set(client, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan clientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := zeroword.OAuthClientUpdate{
		OAuthClientSettings: plan.settings(ctx, &resp.Diagnostics),
		Status:              plan.Status.ValueString(),
		MobileApp:           plan.mobileApp(ctx, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateClient(ctx, plan.ID.ValueString(), body)
	if err != nil {
		apiError(&resp.Diagnostics, "update client", err)
		return
	}
	plan.set(updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteClient(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete client", err)
	}
}

func (r *clientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
