// Package provider implements the Zeroword Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type zerowordProvider struct {
	version string
}

type providerModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &zerowordProvider{version: version} }
}

func (p *zerowordProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "zeroword"
	resp.Version = p.version
}

func (p *zerowordProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Zeroword resources through the Management API. " +
			"The API key fixes the Workspace: every resource is created in the key's Workspace.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "Workspace-scoped Management API key (`sk_live_...`). Defaults to the `ZEROWORD_API_KEY` environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Management API base URL including `/v1`. Defaults to the `ZEROWORD_BASE_URL` environment variable, then `" + zeroword.DefaultBaseURL + "`. Staging: `https://api.staging.zeroword.smartcrab.ai/v1`.",
				Optional:    true,
			},
		},
	}
}

func (p *zerowordProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.APIKey.IsUnknown() || config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddError("Unknown provider configuration",
			"api_key and base_url must be known at plan time.")
		return
	}
	apiKey := firstNonEmpty(config.APIKey.ValueString(), os.Getenv("ZEROWORD_API_KEY"))
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing Management API key",
			"Set api_key or the ZEROWORD_API_KEY environment variable.")
		return
	}
	baseURL := firstNonEmpty(config.BaseURL.ValueString(), os.Getenv("ZEROWORD_BASE_URL"), zeroword.DefaultBaseURL)
	client := zeroword.NewClient(baseURL, apiKey, "terraform-provider-zeroword/"+p.version)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *zerowordProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newProjectResource,
		newEnvironmentResource,
		newEnvironmentSettingsResource,
		newEnvironmentThemeResource,
		newClientResource,
		newConnectionResource,
		newAPIKeyResource,
		newDomainResource,
		newWebhookResource,
	}
}

func (p *zerowordProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newWorkspaceDataSource}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
