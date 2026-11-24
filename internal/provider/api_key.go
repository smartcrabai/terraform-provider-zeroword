package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type apiKeyResource struct{ apiResource }

type apiKeyModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`
	Scopes        types.Set    `tfsdk:"scopes"`
	ExpiresAt     types.String `tfsdk:"expires_at"`
	Prefix        types.String `tfsdk:"prefix"`
	Secret        types.String `tfsdk:"secret"`
}

func newAPIKeyResource() resource.Resource { return &apiKeyResource{} }

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Management API key. Keys cannot be edited: every change revokes the key and issues a new one. " +
			"The provider's own key must hold every scope granted here (and `api_keys:write`). " +
			"A key revoked or expired outside Terraform is issued again on the next apply.",
		Attributes: map[string]schema.Attribute{
			"id":             idAttribute("API key ID (`mky_...`)."),
			"workspace_id":   immutableString("Workspace ID (`wsp_...`). Must be the provider key's Workspace.", true),
			"environment_id": immutableString("Pin the key to this Environment. Omit for a Workspace-scoped key.", false),
			"name":           immutableString("Display name (1-200 characters).", true),
			"scopes": schema.SetAttribute{
				Description:   "Granted scopes, e.g. `users:read`.",
				ElementType:   types.StringType,
				Required:      true,
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
			},
			"expires_at": immutableString("Expiry as RFC 3339 timestamp. Omit for a key without expiry.", false),
			"prefix":     computedString("Non-secret display prefix of the key."),
			"secret": schema.StringAttribute{
				Description: "The key (`sk_live_...`). Zeroword returns it only at creation, so it is null after import. " +
					"Stored in state; protect the state backend.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (m *apiKeyModel) set(k *zeroword.APIKey) {
	m.ID = types.StringValue(k.ID)
	m.WorkspaceID = types.StringValue(k.WorkspaceID)
	m.EnvironmentID = types.StringPointerValue(k.EnvironmentID)
	m.Name = types.StringValue(k.Name)
	m.Scopes = stringSet(k.Scopes)
	m.Prefix = types.StringValue(k.Prefix)
	if k.ExpiresAt == nil {
		m.ExpiresAt = types.StringNull()
		return
	}
	// Keep the configured spelling when it denotes the same instant.
	configured, errConfigured := time.Parse(time.RFC3339Nano, m.ExpiresAt.ValueString())
	returned, errReturned := time.Parse(time.RFC3339Nano, *k.ExpiresAt)
	if errConfigured != nil || errReturned != nil || !configured.Equal(returned) {
		m.ExpiresAt = types.StringValue(*k.ExpiresAt)
	}
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := zeroword.APIKeyCreate{
		WorkspaceID:   plan.WorkspaceID.ValueString(),
		Name:          plan.Name.ValueString(),
		Scopes:        stringSlice(ctx, plan.Scopes, &resp.Diagnostics),
		EnvironmentID: stringPointer(plan.EnvironmentID),
		ExpiresAt:     stringPointer(plan.ExpiresAt),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateAPIKey(ctx, body)
	if err != nil {
		apiError(&resp.Diagnostics, "create API key", err)
		return
	}
	plan.set(created)
	plan.Secret = types.StringValue(created.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) || (err == nil && !usable(key, time.Now())) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read API key", err)
		return
	}
	state.set(key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// usable reports whether the key still authenticates; a revoked or expired
// key is gone as far as Terraform is concerned.
func usable(k *zeroword.APIKey, now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt == nil {
		return true
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, *k.ExpiresAt)
	return err != nil || now.Before(expiresAt)
}

// Update is never called: every attribute requires replacement.
func (r *apiKeyResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RevokeAPIKey(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "revoke API key", err)
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
