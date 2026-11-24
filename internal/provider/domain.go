package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

type domainResource struct{ apiResource }

type domainModel struct {
	ID                      types.String `tfsdk:"id"`
	EnvironmentID           types.String `tfsdk:"environment_id"`
	Hostname                types.String `tfsdk:"hostname"`
	RPEnabled               types.Bool   `tfsdk:"rp_enabled"`
	Status                  types.String `tfsdk:"status"`
	OwnershipVerified       types.Bool   `tfsdk:"ownership_verified"`
	VerificationRecordName  types.String `tfsdk:"verification_record_name"`
	VerificationRecordValue types.String `tfsdk:"verification_record_value"`
}

func newDomainResource() resource.Resource { return &domainResource{} }

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom Hosted Login domain of an Environment. Publish the TXT record " +
			"`verification_record_name` = `verification_record_value` (for example with a DNS provider resource); " +
			"Zeroword activates the domain after it sees the record.",
		Attributes: map[string]schema.Attribute{
			"id":             idAttribute("Domain ID."),
			"environment_id": immutableString("Environment ID (`env_...`).", true),
			"hostname":       immutableString("Custom hostname, e.g. `login.example.com`.", true),
			"rp_enabled": schema.BoolAttribute{
				Description: "Also serve this host as the WebAuthn RP ID. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"status":                    schema.StringAttribute{Description: "Provisioning status.", Computed: true},
			"ownership_verified":        schema.BoolAttribute{Description: "Whether the TXT record was found.", Computed: true},
			"verification_record_name":  computedString("TXT record name to publish."),
			"verification_record_value": computedString("TXT record value to publish."),
		},
	}
}

func (m *domainModel) set(d *zeroword.Domain) {
	m.ID = types.StringValue(d.ID)
	m.EnvironmentID = types.StringValue(d.EnvironmentID)
	m.Hostname = types.StringValue(d.Hostname)
	m.RPEnabled = types.BoolValue(d.RPEnabled)
	m.Status = types.StringValue(d.Status)
	m.OwnershipVerified = types.BoolValue(d.OwnershipVerified)
	m.VerificationRecordName = types.StringPointerValue(d.VerificationRecordName)
	m.VerificationRecordValue = types.StringPointerValue(d.VerificationRecordValue)
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateDomain(ctx, zeroword.DomainCreate{
		EnvironmentID: plan.EnvironmentID.ValueString(),
		Hostname:      plan.Hostname.ValueString(),
		RPEnabled:     plan.RPEnabled.ValueBool(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "create domain", err)
		return
	}
	plan.set(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.GetDomain(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read domain", err)
		return
	}
	state.set(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateDomain(ctx, plan.ID.ValueString(), plan.RPEnabled.ValueBool())
	if err != nil {
		apiError(&resp.Diagnostics, "update domain", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDomain(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete domain", err)
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
