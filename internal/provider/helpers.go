package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/smartcrabai/terraform-provider-zeroword/internal/zeroword"
)

// apiResource carries the configured API client into every resource.
type apiResource struct {
	client *zeroword.Client
}

func (r *apiResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*zeroword.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *zeroword.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func apiError(diags *diag.Diagnostics, action string, err error) {
	diags.AddError("Zeroword API error", action+": "+err.Error())
}

// stringSlice converts a set or list of strings; null and unknown become an
// empty (non-nil) slice so the API receives `[]`.
func stringSlice(ctx context.Context, value interface {
	IsNull() bool
	IsUnknown() bool
	ElementsAs(context.Context, any, bool) diag.Diagnostics
}, diags *diag.Diagnostics) []string {
	out := []string{}
	if value.IsNull() || value.IsUnknown() {
		return out
	}
	diags.Append(value.ElementsAs(ctx, &out, false)...)
	return out
}

func stringSet(values []string) types.Set {
	elements := make([]types.String, 0, len(values))
	for _, v := range values {
		elements = append(elements, types.StringValue(v))
	}
	set, _ := types.SetValueFrom(context.Background(), types.StringType, elements)
	return set
}

func stringList(values []string) types.List {
	elements := make([]types.String, 0, len(values))
	for _, v := range values {
		elements = append(elements, types.StringValue(v))
	}
	list, _ := types.ListValueFrom(context.Background(), types.StringType, elements)
	return list
}

func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueString()
	return &v
}

func idAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func immutableString(description string, required bool) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Required:      required,
		Optional:      !required,
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
}
