package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ resource.Resource = (*nameserversResource)(nil)
var _ resource.ResourceWithConfigure = (*nameserversResource)(nil)
var _ resource.ResourceWithImportState = (*nameserversResource)(nil)

type nameserversResource struct {
	client *gandi.Client
}

func newNameserversResource() resource.Resource { return &nameserversResource{} }

type nameserversModel struct {
	Domain      types.String `tfsdk:"domain"`
	ID          types.String `tfsdk:"id"`
	Nameservers types.List   `tfsdk:"nameservers"`
}

func (r *nameserversResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nameservers"
}

func (r *nameserversResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the registry-level nameservers of a Gandi domain. " +
			"Deleting this resource stops Terraform managing the nameservers but does not reset them at the registry.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) whose nameservers are managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN); same as `domain`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "Ordered list of nameserver hostnames. Minimum one.",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
}

func (r *nameserversResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*gandi.Client)
}

func (r *nameserversResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nameserversModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(plan.Domain.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nameserversResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nameserversModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, err := r.client.GetNameservers(ctx, state.Domain.ValueString())
	if err != nil {
		if gandi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read nameservers", err.Error())
		return
	}

	list, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Nameservers = list
	state.ID = types.StringValue(state.Domain.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nameserversResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nameserversModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(plan.Domain.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is intentionally a no-op at the registry: a domain must always have
// nameservers, so we only drop it from Terraform state.
func (r *nameserversResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *nameserversResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain"), req, resp)
}

func (r *nameserversResource) apply(ctx context.Context, plan nameserversModel, diags *diag.Diagnostics) {
	var ns []string
	d := plan.Nameservers.ElementsAs(ctx, &ns, false)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	if err := r.client.SetNameservers(ctx, plan.Domain.ValueString(), ns); err != nil {
		diags.AddError("Unable to set nameservers", err.Error())
	}
}
