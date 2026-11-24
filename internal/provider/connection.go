package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type connectionResource struct{ apiResource }

type connectionModel struct {
	ID                            types.String `tfsdk:"id"`
	EnvironmentID                 types.String `tfsdk:"environment_id"`
	Provider                      types.String `tfsdk:"social_provider"`
	Mode                          types.String `tfsdk:"mode"`
	ProviderKey                   types.String `tfsdk:"provider_key"`
	Issuer                        types.String `tfsdk:"issuer"`
	ProviderClientID              types.String `tfsdk:"provider_client_id"`
	ProviderClientSecretWO        types.String `tfsdk:"provider_client_secret_wo"`
	ProviderClientSecretWOVersion types.Int64  `tfsdk:"provider_client_secret_wo_version"`
	Scopes                        types.List   `tfsdk:"scopes"`
	ProfileMapping                types.Map    `tfsdk:"profile_mapping"`
	Status                        types.String `tfsdk:"status"`
	OwnerConfirmed                types.Bool   `tfsdk:"owner_confirmed"`
}

func newConnectionResource() resource.Resource { return &connectionResource{} }

func (r *connectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (r *connectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	provider := immutableString("`google`, `apple`, `github`, `microsoft`, or `generic_oidc`.", true)
	provider.Validators = []validator.String{stringvalidator.OneOf("google", "apple", "github", "microsoft", "generic_oidc")}
	mode := immutableString("`managed` (Zeroword's shared OAuth app) or `byo` (your own provider app).", true)
	mode.Validators = []validator.String{stringvalidator.OneOf("managed", "byo")}
	emptyMap, _ := types.MapValue(types.StringType, nil)
	resp.Schema = schema.Schema{
		Description: "A social login connection of an Environment. Requires Terraform 1.11 or later " +
			"for the write-only client secret.",
		Attributes: map[string]schema.Attribute{
			"id":              idAttribute("Connection ID."),
			"environment_id":  immutableString("Environment ID (`env_...`).", true),
			"social_provider": provider,
			"mode":            mode,
			"provider_key":    immutableString("Key unique per Environment, used in callback URLs.", true),
			"issuer":          optionalString("Issuer URL (HTTPS). Required for `generic_oidc`."),
			"provider_client_id": schema.StringAttribute{
				Description: "Client ID at the social provider.",
				Required:    true,
			},
			"provider_client_secret_wo": schema.StringAttribute{
				Description: "Client secret at the social provider. Write-only: never stored in state and " +
					"never returned by Zeroword. Sent on create, and on update only when " +
					"`provider_client_secret_wo_version` changes.",
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
			},
			"provider_client_secret_wo_version": schema.Int64Attribute{
				Description: "Change this number to send `provider_client_secret_wo` again (secret rotation).",
				Optional:    true,
			},
			"scopes": schema.ListAttribute{
				Description: "Scopes requested from the provider (max 20). Defaults to empty.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(stringList(nil)),
			},
			"profile_mapping": schema.MapAttribute{
				Description: "Mapping from Zeroword profile fields to provider claims. Defaults to empty.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     mapdefault.StaticValue(emptyMap),
			},
			"status": schema.StringAttribute{
				Description: "`active` or `disabled`. Defaults to `active`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("active"),
				Validators:  []validator.String{stringvalidator.OneOf("active", "disabled")},
			},
			"owner_confirmed": schema.BoolAttribute{
				Description: "Owner confirmation of a `generic_oidc` issuer. Set `true` to allow sign-in; " +
					"changing `issuer` clears it unless this is set in the same apply. Always `true` for other providers.",
				Optional: true,
				Computed: true,
			},
		},
	}
}

func (m *connectionModel) set(c *zeroword.Connection) {
	m.ID = types.StringValue(c.ID)
	m.EnvironmentID = types.StringValue(c.EnvironmentID)
	m.Provider = types.StringValue(c.Provider)
	m.Mode = types.StringValue(c.Mode)
	m.ProviderKey = types.StringValue(c.ProviderKey)
	m.Issuer = types.StringPointerValue(c.Issuer)
	m.ProviderClientID = types.StringValue(c.ProviderClientID)
	m.Scopes = stringList(c.Scopes)
	mapping, _ := types.MapValueFrom(context.Background(), types.StringType, c.ProfileMapping)
	m.ProfileMapping = mapping
	m.Status = types.StringValue(c.Status)
	m.OwnerConfirmed = types.BoolValue(c.OwnerConfirmed)
	// Write-only attributes are always null in state.
	m.ProviderClientSecretWO = types.StringNull()
}

// update builds the PATCH body. owner_confirmed comes from the configuration,
// not the plan, so an unset value never re-approves a changed issuer.
func (m *connectionModel) update(ctx context.Context, secret string, configured types.Bool, diags *diag.Diagnostics) zeroword.ConnectionUpdate {
	mapping := map[string]string{}
	diags.Append(m.ProfileMapping.ElementsAs(ctx, &mapping, false)...)
	var ownerConfirmed *bool
	if !configured.IsNull() && !configured.IsUnknown() {
		v := configured.ValueBool()
		ownerConfirmed = &v
	}
	return zeroword.ConnectionUpdate{
		Issuer:               stringPointer(m.Issuer),
		ProviderClientID:     m.ProviderClientID.ValueString(),
		ProviderClientSecret: secret,
		Scopes:               stringSlice(ctx, m.Scopes, diags),
		ProfileMapping:       mapping,
		Status:               m.Status.ValueString(),
		OwnerConfirmed:       ownerConfirmed,
	}
}

func (r *connectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config connectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mapping := map[string]string{}
	resp.Diagnostics.Append(plan.ProfileMapping.ElementsAs(ctx, &mapping, false)...)
	body := zeroword.ConnectionCreate{
		EnvironmentID:        plan.EnvironmentID.ValueString(),
		Provider:             plan.Provider.ValueString(),
		Mode:                 plan.Mode.ValueString(),
		ProviderKey:          plan.ProviderKey.ValueString(),
		Issuer:               stringPointer(plan.Issuer),
		ProviderClientID:     plan.ProviderClientID.ValueString(),
		ProviderClientSecret: config.ProviderClientSecretWO.ValueString(),
		Scopes:               stringSlice(ctx, plan.Scopes, &resp.Diagnostics),
		ProfileMapping:       mapping,
	}
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateConnection(ctx, body)
	if err != nil {
		apiError(&resp.Diagnostics, "create connection", err)
		return
	}
	desired := plan
	plan.set(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	ownerMismatch := !config.OwnerConfirmed.IsNull() && config.OwnerConfirmed.ValueBool() != created.OwnerConfirmed
	if (created.Status == desired.Status.ValueString() && !ownerMismatch) || resp.Diagnostics.HasError() {
		return
	}
	// status and owner_confirmed are not create fields; apply them in a second call.
	updated, err := r.client.UpdateConnection(ctx, created.ID, desired.update(ctx, "", config.OwnerConfirmed, &resp.Diagnostics))
	if err != nil {
		apiError(&resp.Diagnostics, "set connection status", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	connection, err := r.client.GetConnection(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read connection", err)
		return
	}
	state.set(connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior, config connectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secret := ""
	if !plan.ProviderClientSecretWOVersion.Equal(prior.ProviderClientSecretWOVersion) {
		secret = config.ProviderClientSecretWO.ValueString()
	}
	body := plan.update(ctx, secret, config.OwnerConfirmed, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateConnection(ctx, plan.ID.ValueString(), body)
	if err != nil {
		apiError(&resp.Diagnostics, "update connection", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteConnection(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete connection", err)
	}
}

func (r *connectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
