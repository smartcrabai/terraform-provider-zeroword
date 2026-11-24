package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type environmentThemeResource struct{ apiResource }

type environmentThemeModel struct {
	EnvironmentID types.String         `tfsdk:"environment_id"`
	Theme         jsontypes.Normalized `tfsdk:"theme"`
	Version       types.Int64          `tfsdk:"version"`
}

func newEnvironmentThemeResource() resource.Resource { return &environmentThemeResource{} }

func (r *environmentThemeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_theme"
}

func (r *environmentThemeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Hosted Login theme of one Environment (`PUT /environments/{id}/theme`). The API has no " +
			"delete, so destroying this resource only removes it from state and leaves the last theme in place. " +
			"Activating a social connection appends its provider to `methods` when it is missing; list every " +
			"active provider in `methods` to avoid a diff after that.",
		Attributes: map[string]schema.Attribute{
			"environment_id": immutableString("Environment ID (`env_...`).", true),
			"theme": schema.StringAttribute{
				Description: "Theme document as JSON (use `jsonencode`). It replaces the whole theme; see " +
					"https://docs.zeroword.smartcrab.ai/hosted-login for the allowed keys.",
				Required:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"version": schema.Int64Attribute{Description: "Theme version, incremented on every save.", Computed: true},
		},
	}
}

func (m *environmentThemeModel) set(t *zeroword.EnvironmentTheme) error {
	encoded, err := json.Marshal(t.Theme)
	if err != nil {
		return err
	}
	m.EnvironmentID = types.StringValue(t.EnvironmentID)
	m.Theme = jsontypes.NewNormalizedValue(string(encoded))
	m.Version = types.Int64Value(t.Version)
	return nil
}

func (r *environmentThemeResource) put(ctx context.Context, plan *environmentThemeModel) error {
	var theme map[string]any
	if err := json.Unmarshal([]byte(plan.Theme.ValueString()), &theme); err != nil {
		return err
	}
	saved, err := r.client.PutEnvironmentTheme(ctx, plan.EnvironmentID.ValueString(), theme)
	if err != nil {
		return err
	}
	// Keep the configured JSON text: the saved theme is semantically equal,
	// and Terraform requires the planned value back byte for byte.
	plan.Version = types.Int64Value(saved.Version)
	return nil
}

func (r *environmentThemeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentThemeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		apiError(&resp.Diagnostics, "put environment theme", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentThemeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentThemeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	theme, err := r.client.GetEnvironmentTheme(ctx, state.EnvironmentID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read environment theme", err)
		return
	}
	if err := state.set(theme); err != nil {
		apiError(&resp.Diagnostics, "encode environment theme", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentThemeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentThemeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		apiError(&resp.Diagnostics, "put environment theme", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentThemeResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *environmentThemeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}
