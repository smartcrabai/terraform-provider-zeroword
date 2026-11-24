package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type environmentResource struct{ apiResource }

type environmentModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	Type              types.String `tfsdk:"type"`
	Name              types.String `tfsdk:"name"`
	Status            types.String `tfsdk:"status"`
	Issuer            types.String `tfsdk:"issuer"`
	InteractionDomain types.String `tfsdk:"interaction_domain"`
	RPID              types.String `tfsdk:"rp_id"`
}

func newEnvironmentResource() resource.Resource { return &environmentResource{} }

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func computedString(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	typeAttr := immutableString("`test` or `production`. Changing it replaces the Environment.", true)
	typeAttr.Validators = []validator.String{stringvalidator.OneOf("test", "production")}
	resp.Schema = schema.Schema{
		Description: "An Environment is one issuer (tenant) inside a Project. Deleting it is a soft delete in Zeroword.",
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute("Environment ID (`env_...`)."),
			"project_id": immutableString("Project ID (`prj_...`).", true),
			"type":       typeAttr,
			"name":       schema.StringAttribute{Description: "Display name (1-200 characters).", Required: true},
			"status": schema.StringAttribute{
				Description: "`active` or `suspended`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("active"),
				Validators:  []validator.String{stringvalidator.OneOf("active", "suspended")},
			},
			"issuer":             computedString("OIDC issuer URL."),
			"interaction_domain": computedString("Hosted Login host."),
			"rp_id":              computedString("WebAuthn Relying Party ID."),
		},
	}
}

func (m *environmentModel) set(e *zeroword.Environment) {
	m.ID = types.StringValue(e.ID)
	m.ProjectID = types.StringValue(e.ProjectID)
	m.Type = types.StringValue(e.Type)
	m.Name = types.StringValue(e.Name)
	m.Status = types.StringValue(e.Status)
	m.Issuer = types.StringValue(e.Issuer)
	m.InteractionDomain = types.StringValue(e.InteractionDomain)
	m.RPID = types.StringValue(e.RPID)
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateEnvironment(ctx, zeroword.EnvironmentCreate{
		ProjectID: plan.ProjectID.ValueString(),
		Type:      plan.Type.ValueString(),
		Name:      plan.Name.ValueString(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "create environment", err)
		return
	}
	desiredStatus := plan.Status.ValueString()
	plan.set(created)
	// Saved before the follow-up call: if it fails, Terraform keeps the
	// Environment as tainted instead of losing track of it.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if created.Status == desiredStatus {
		return
	}
	updated, err := r.client.UpdateEnvironment(ctx, created.ID, zeroword.EnvironmentUpdate{
		Name:   created.Name,
		Status: desiredStatus,
	})
	if err != nil {
		apiError(&resp.Diagnostics, "set environment status", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	environment, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) || (err == nil && environment.Status == "deleted") {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read environment", err)
		return
	}
	state.set(environment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateEnvironment(ctx, plan.ID.ValueString(), zeroword.EnvironmentUpdate{
		Name:   plan.Name.ValueString(),
		Status: plan.Status.ValueString(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "update environment", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEnvironment(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete environment", err)
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
