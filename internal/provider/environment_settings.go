package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type environmentSettingsResource struct{ apiResource }

type environmentSettingsModel struct {
	EnvironmentID        types.String `tfsdk:"environment_id"`
	ImpersonationEnabled types.Bool   `tfsdk:"impersonation_enabled"`
}

func newEnvironmentSettingsResource() resource.Resource { return &environmentSettingsResource{} }

func (r *environmentSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_settings"
}

func (r *environmentSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Management settings of one Environment. Every Environment always has settings, so " +
			"destroying this resource resets them to the defaults (`impersonation_enabled = false`).",
		Attributes: map[string]schema.Attribute{
			"environment_id": immutableString("Environment ID (`env_...`).", true),
			"impersonation_enabled": schema.BoolAttribute{
				Description: "Whether Owner/Admin Dashboard sessions may impersonate users of this Environment.",
				Required:    true,
			},
		},
	}
}

func (m *environmentSettingsModel) set(s *zeroword.EnvironmentSettings) {
	m.EnvironmentID = types.StringValue(s.EnvironmentID)
	m.ImpersonationEnabled = types.BoolValue(s.ImpersonationEnabled)
}

func (r *environmentSettingsResource) write(ctx context.Context, plan environmentSettingsModel) (environmentSettingsModel, error) {
	saved, err := r.client.UpdateEnvironmentSettings(ctx, plan.EnvironmentID.ValueString(), plan.ImpersonationEnabled.ValueBool())
	if err != nil {
		return plan, err
	}
	plan.set(saved)
	return plan, nil
}

func (r *environmentSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.write(ctx, plan)
	if err != nil {
		apiError(&resp.Diagnostics, "update environment settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	settings, err := r.client.GetEnvironmentSettings(ctx, state.EnvironmentID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read environment settings", err)
		return
	}
	state.set(settings)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.write(ctx, plan)
	if err != nil {
		apiError(&resp.Diagnostics, "update environment settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ImpersonationEnabled = types.BoolValue(false)
	if _, err := r.write(ctx, state); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "reset environment settings", err)
	}
}

func (r *environmentSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}
