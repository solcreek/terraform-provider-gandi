package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ resource.Resource = (*livednsRecordResource)(nil)
var _ resource.ResourceWithConfigure = (*livednsRecordResource)(nil)
var _ resource.ResourceWithImportState = (*livednsRecordResource)(nil)

type livednsRecordResource struct {
	client *gandi.Client
}

func newLiveDNSRecordResource() resource.Resource { return &livednsRecordResource{} }

type livednsRecordModel struct {
	Domain types.String `tfsdk:"domain"`
	Name   types.String `tfsdk:"name"`
	Type   types.String `tfsdk:"type"`
	TTL    types.Int64  `tfsdk:"ttl"`
	Values types.Set    `tfsdk:"values"`
	ID     types.String `tfsdk:"id"`
}

func (r *livednsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_livedns_record"
}

func (r *livednsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single LiveDNS rrset (record) on a Gandi domain. " +
			"The domain must use Gandi LiveDNS nameservers for records to resolve.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) the record belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Record name relative to the domain, e.g. `www` or `@` for the apex.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Record type, e.g. `A`, `AAAA`, `CNAME`, `MX`, `TXT`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Time to live in seconds (300–2592000). Defaults to 10800.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(10800),
			},
			"values": schema.SetAttribute{
				MarkdownDescription: "Record values. For CNAME/MX/NS use fully-qualified names ending with a dot.",
				ElementType:         types.StringType,
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "`<domain>/<name>/<type>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *livednsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*gandi.Client)
}

func (r *livednsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan livednsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, diags := plan.toRecord(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.CreateLiveDNSRecord(ctx, plan.Domain.ValueString(), rec); err != nil {
		resp.Diagnostics.AddError("Unable to create LiveDNS record", err.Error())
		return
	}
	plan.ID = types.StringValue(recordID(plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *livednsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state livednsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, err := r.client.GetLiveDNSRecord(ctx, state.Domain.ValueString(), state.Name.ValueString(), state.Type.ValueString())
	if err != nil {
		if gandi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read LiveDNS record", err.Error())
		return
	}

	values, diags := types.SetValueFrom(ctx, types.StringType, rec.Values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.TTL = types.Int64Value(rec.TTL)
	state.Values = values
	state.ID = types.StringValue(recordID(state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *livednsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan livednsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, diags := plan.toRecord(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateLiveDNSRecord(ctx, plan.Domain.ValueString(), rec); err != nil {
		resp.Diagnostics.AddError("Unable to update LiveDNS record", err.Error())
		return
	}
	plan.ID = types.StringValue(recordID(plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *livednsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state livednsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteLiveDNSRecord(ctx, state.Domain.ValueString(), state.Name.ValueString(), state.Type.ValueString()); err != nil {
		if !gandi.IsNotFound(err) {
			resp.Diagnostics.AddError("Unable to delete LiveDNS record", err.Error())
		}
	}
}

func (r *livednsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `<domain>/<name>/<type>`, got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (m livednsRecordModel) toRecord(ctx context.Context) (gandi.LiveDNSRecord, diag.Diagnostics) {
	var values []string
	diags := m.Values.ElementsAs(ctx, &values, false)
	return gandi.LiveDNSRecord{
		Name:   m.Name.ValueString(),
		Type:   m.Type.ValueString(),
		TTL:    m.TTL.ValueInt64(),
		Values: values,
	}, diags
}

func recordID(m livednsRecordModel) string {
	return m.Domain.ValueString() + "/" + m.Name.ValueString() + "/" + m.Type.ValueString()
}
