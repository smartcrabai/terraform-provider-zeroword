package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type projectResource struct{ apiResource }

type projectModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
}

func newProjectResource() resource.Resource { return &projectResource{} }

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Project groups Environments inside a Workspace. Deleting a Project fails while it still has Environments.",
		Attributes: map[string]schema.Attribute{
			"id": idAttribute("Project ID (`prj_...`)."),
			"workspace_id": schema.StringAttribute{
				Description:   "Workspace ID (`wsp_...`). Must be the API key's Workspace.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{Description: "Display name (1-200 characters).", Required: true},
			"slug": schema.StringAttribute{
				Description:   "URL slug. Generated from the name when omitted.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (m *projectModel) set(p *zeroword.Project) {
	m.ID = types.StringValue(p.ID)
	m.WorkspaceID = types.StringValue(p.WorkspaceID)
	m.Name = types.StringValue(p.Name)
	m.Slug = types.StringValue(p.Slug)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateProject(ctx, zeroword.ProjectCreate{
		WorkspaceID: plan.WorkspaceID.ValueString(),
		Name:        plan.Name.ValueString(),
		Slug:        plan.Slug.ValueString(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "create project", err)
		return
	}
	plan.set(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	project, err := r.client.GetProject(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) || (err == nil && project.DeletedAt != nil) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read project", err)
		return
	}
	state.set(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateProject(ctx, plan.ID.ValueString(), zeroword.ProjectUpdate{
		Name: plan.Name.ValueString(),
		Slug: plan.Slug.ValueString(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "update project", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete project", err)
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
