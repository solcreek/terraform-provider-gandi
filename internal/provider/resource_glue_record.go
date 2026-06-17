package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ resource.Resource = (*glueRecordResource)(nil)
var _ resource.ResourceWithConfigure = (*glueRecordResource)(nil)
var _ resource.ResourceWithImportState = (*glueRecordResource)(nil)

type glueRecordResource struct {
	client *gandi.Client
}

func newGlueRecordResource() resource.Resource { return &glueRecordResource{} }

type glueRecordModel struct {
	Domain types.String `tfsdk:"domain"`
	Name   types.String `tfsdk:"name"`
	ID     types.String `tfsdk:"id"`
	IPs    types.Set    `tfsdk:"ips"`
}

func (r *glueRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_glue_record"
}

func (r *glueRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a glue record (host) for a Gandi domain, mapping a nameserver name to one or more IPs.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) the glue record belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Short host name, e.g. `ns1` for `ns1.example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "`<domain>/<name>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ips": schema.SetAttribute{
				MarkdownDescription: "IPv4/IPv6 addresses for the host.",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
}

func (r *glueRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*gandi.Client)
}

func (r *glueRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan glueRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ips []string
	resp.Diagnostics.Append(plan.IPs.ElementsAs(ctx, &ips, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.CreateHost(ctx, plan.Domain.ValueString(), plan.Name.ValueString(), ips); err != nil {
		resp.Diagnostics.AddError("Unable to create glue record", err.Error())
		return
	}
	// Creation is asynchronous; wait until the registry reflects it.
	if err := r.client.WaitForHostIPs(ctx, plan.Domain.ValueString(), plan.Name.ValueString(), ips); err != nil {
		resp.Diagnostics.AddError("Glue record did not become consistent", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.Domain.ValueString() + "/" + plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *glueRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state glueRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host, err := r.client.GetHost(ctx, state.Domain.ValueString(), state.Name.ValueString())
	if err != nil {
		if gandi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read glue record", err.Error())
		return
	}

	ips, diags := types.SetValueFrom(ctx, types.StringType, host.IPs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.IPs = ips
	state.ID = types.StringValue(state.Domain.ValueString() + "/" + state.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *glueRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan glueRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ips []string
	resp.Diagnostics.Append(plan.IPs.ElementsAs(ctx, &ips, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateHost(ctx, plan.Domain.ValueString(), plan.Name.ValueString(), ips); err != nil {
		resp.Diagnostics.AddError("Unable to update glue record", err.Error())
		return
	}
	// Updates are asynchronous; wait until the new IPs are reflected.
	if err := r.client.WaitForHostIPs(ctx, plan.Domain.ValueString(), plan.Name.ValueString(), ips); err != nil {
		resp.Diagnostics.AddError("Glue record did not become consistent", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.Domain.ValueString() + "/" + plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *glueRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state glueRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteHost(ctx, state.Domain.ValueString(), state.Name.ValueString()); err != nil {
		if !gandi.IsNotFound(err) {
			resp.Diagnostics.AddError("Unable to delete glue record", err.Error())
		}
		return
	}
	// Deletion is asynchronous; wait until it is gone so re-creates are clean.
	if err := r.client.WaitForHostGone(ctx, state.Domain.ValueString(), state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Glue record was not deleted in time", err.Error())
	}
}

func (r *glueRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, name, ok := strings.Cut(req.ID, "/")
	if !ok || domain == "" || name == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `<domain>/<name>`, got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
