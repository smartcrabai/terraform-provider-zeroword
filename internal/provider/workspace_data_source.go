package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type workspaceDataSource struct {
	client *zeroword.Client
}

type workspaceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Slug   types.String `tfsdk:"slug"`
	Status types.String `tfsdk:"status"`
}

func newWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Workspace. Workspaces are created in the Dashboard; an API key cannot create them.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Description: "Workspace ID (`wsp_...`).", Required: true},
			"name":   schema.StringAttribute{Description: "Display name.", Computed: true},
			"slug":   schema.StringAttribute{Description: "URL slug.", Computed: true},
			"status": schema.StringAttribute{Description: "Workspace status.", Computed: true},
		},
	}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*zeroword.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *zeroword.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspaceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	workspace, err := d.client.GetWorkspace(ctx, config.ID.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "read workspace", err)
		return
	}
	state := workspaceModel{
		ID:     types.StringValue(workspace.ID),
		Name:   types.StringValue(workspace.Name),
		Slug:   types.StringValue(workspace.Slug),
		Status: types.StringValue(workspace.Status),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
