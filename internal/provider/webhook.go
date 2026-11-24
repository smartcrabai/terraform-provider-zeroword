package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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

type webhookResource struct{ apiResource }

type webhookModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	URL           types.String `tfsdk:"url"`
	EventTypes    types.Set    `tfsdk:"event_types"`
	Status        types.String `tfsdk:"status"`
	Secret        types.String `tfsdk:"secret"`
}

func newWebhookResource() resource.Resource { return &webhookResource{} }

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A webhook endpoint of an Environment.",
		Attributes: map[string]schema.Attribute{
			"id":             idAttribute("Webhook ID."),
			"environment_id": immutableString("Environment ID (`env_...`).", true),
			"url":            schema.StringAttribute{Description: "HTTPS endpoint URL.", Required: true},
			"event_types": schema.SetAttribute{
				Description: "Subscribed event types, e.g. `user.created`.",
				ElementType: types.StringType,
				Required:    true,
				Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"status": schema.StringAttribute{
				Description: "`active` or `disabled`. Defaults to `active`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("active"),
				Validators:  []validator.String{stringvalidator.OneOf("active", "disabled")},
			},
			"secret": schema.StringAttribute{
				Description: "Signing secret. Zeroword returns it only at creation, so it is null after import. " +
					"Stored in state; protect the state backend.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (m *webhookModel) set(w *zeroword.Webhook) {
	m.ID = types.StringValue(w.ID)
	m.EnvironmentID = types.StringValue(w.EnvironmentID)
	m.URL = types.StringValue(w.URL)
	m.EventTypes = stringSet(w.EventTypes)
	m.Status = types.StringValue(w.Status)
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	eventTypes := stringSlice(ctx, plan.EventTypes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateWebhook(ctx, zeroword.WebhookCreate{
		EnvironmentID: plan.EnvironmentID.ValueString(),
		URL:           plan.URL.ValueString(),
		EventTypes:    eventTypes,
	})
	if err != nil {
		apiError(&resp.Diagnostics, "create webhook", err)
		return
	}
	desiredStatus := plan.Status.ValueString()
	plan.set(created)
	plan.Secret = types.StringValue(created.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if created.Status == desiredStatus {
		return
	}
	updated, err := r.client.UpdateWebhook(ctx, created.ID, zeroword.WebhookUpdate{
		URL:        created.URL,
		EventTypes: created.EventTypes,
		Status:     desiredStatus,
	})
	if err != nil {
		apiError(&resp.Diagnostics, "set webhook status", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	webhook, err := r.client.GetWebhook(ctx, state.ID.ValueString())
	if zeroword.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "read webhook", err)
		return
	}
	state.set(webhook)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	eventTypes := stringSlice(ctx, plan.EventTypes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateWebhook(ctx, plan.ID.ValueString(), zeroword.WebhookUpdate{
		URL:        plan.URL.ValueString(),
		EventTypes: eventTypes,
		Status:     plan.Status.ValueString(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "update webhook", err)
		return
	}
	plan.set(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWebhook(ctx, state.ID.ValueString()); err != nil && !zeroword.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete webhook", err)
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
