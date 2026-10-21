package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ datasource.DataSource = (*domainDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*domainDataSource)(nil)

type domainDataSource struct {
	client *gandi.Client
}

func newDomainDataSource() datasource.DataSource { return &domainDataSource{} }

type domainModel struct {
	FQDN           types.String `tfsdk:"fqdn"`
	TLD            types.String `tfsdk:"tld"`
	ID             types.String `tfsdk:"id"`
	Status         types.List   `tfsdk:"status"`
	Nameservers    types.List   `tfsdk:"nameservers"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	RegistryEndsAt types.String `tfsdk:"registry_ends_at"`
	DNSSECAvail    types.Bool   `tfsdk:"dnssec_available"`

	RenewBeginsAt       types.String `tfsdk:"renew_begins_at"`
	HoldBeginsAt        types.String `tfsdk:"hold_begins_at"`
	HoldEndsAt          types.String `tfsdk:"hold_ends_at"`
	DeletesAt           types.String `tfsdk:"deletes_at"`
	RestoreEndsAt       types.String `tfsdk:"restore_ends_at"`
	PendingDeleteEndsAt types.String `tfsdk:"pending_delete_ends_at"`
	Autorenew           types.Object `tfsdk:"autorenew"`
}

var domainAutorenewAttrTypes = map[string]attr.Type{
	"enabled":  types.BoolType,
	"duration": types.Int64Type,
	"org_id":   types.StringType,
	"dates":    types.ListType{ElemType: types.StringType},
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Gandi domain.",
		Attributes: map[string]schema.Attribute{
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "Fully qualified domain name to look up.",
				Required:            true,
			},
			"tld":              schema.StringAttribute{Computed: true},
			"id":               schema.StringAttribute{Computed: true},
			"status":           schema.ListAttribute{ElementType: types.StringType, Computed: true},
			"nameservers":      schema.ListAttribute{ElementType: types.StringType, Computed: true},
			"created_at":       schema.StringAttribute{Computed: true},
			"updated_at":       schema.StringAttribute{Computed: true},
			"registry_ends_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Expiry date at the registry."},
			"dnssec_available": schema.BoolAttribute{
				Computed: true,
				MarkdownDescription: "Whether the registry accepts DNSSEC keys for this domain (see `gandi_dnssec_key`). " +
					"Null if Gandi does not report it.",
			},
			"renew_begins_at":        lifecycleDate("Start of the renewal period. "),
			"hold_begins_at":         lifecycleDate("When the domain enters hold after expiring. "),
			"hold_ends_at":           lifecycleDate("End of the hold period. "),
			"deletes_at":             lifecycleDate("When the domain is deleted if not renewed. "),
			"restore_ends_at":        lifecycleDate("End of the restore (redemption) period. "),
			"pending_delete_ends_at": lifecycleDate("End of the pending-delete period. "),
			"autorenew": schema.SingleNestedAttribute{
				MarkdownDescription: "Automatic renewal setting (see `gandi_domain_autorenew`). Null if Gandi does not report it.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether automatic renewal is on."},
					"duration": schema.Int64Attribute{
						Computed: true, MarkdownDescription: "Years added at each automatic renewal.",
					},
					"org_id": schema.StringAttribute{
						Computed: true, MarkdownDescription: "UUID of the organization billed for automatic renewals.",
					},
					"dates": schema.ListAttribute{
						ElementType: types.StringType, Computed: true,
						MarkdownDescription: "Scheduled automatic renewal attempts.",
					},
				},
			},
		},
	}
}

func lifecycleDate(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: desc + "Null if Gandi does not report it."}
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*gandi.Client)
}

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data domainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dom, err := d.client.GetDomain(ctx, data.FQDN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}

	live, err := d.client.GetDomainLiveDNS(ctx, data.FQDN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain DNSSEC availability", err.Error())
		return
	}

	status, diags := types.ListValueFrom(ctx, types.StringType, dom.Status)
	resp.Diagnostics.Append(diags...)
	ns, diags := types.ListValueFrom(ctx, types.StringType, dom.Nameservers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.TLD = types.StringValue(dom.TLD)
	data.ID = types.StringValue(dom.ID)
	data.Status = status
	data.Nameservers = ns
	data.CreatedAt = types.StringValue(dom.Dates.CreatedAt)
	data.UpdatedAt = types.StringValue(dom.Dates.UpdatedAt)
	data.RegistryEndsAt = types.StringValue(dom.Dates.RegistryEndsAt)
	data.DNSSECAvail = types.BoolPointerValue(live.DNSSECAvailable)
	data.RenewBeginsAt = stringOrNull(dom.Dates.RenewBeginsAt)
	data.HoldBeginsAt = stringOrNull(dom.Dates.HoldBeginsAt)
	data.HoldEndsAt = stringOrNull(dom.Dates.HoldEndsAt)
	data.DeletesAt = stringOrNull(dom.Dates.DeletesAt)
	data.RestoreEndsAt = stringOrNull(dom.Dates.RestoreEndsAt)
	data.PendingDeleteEndsAt = stringOrNull(dom.Dates.PendingDeleteEndsAt)

	data.Autorenew = types.ObjectNull(domainAutorenewAttrTypes)
	if a := dom.Autorenew; a != nil {
		if a.Dates == nil {
			a.Dates = []string{}
		}
		dates, diags := types.ListValueFrom(ctx, types.StringType, a.Dates)
		resp.Diagnostics.Append(diags...)
		obj, diags := types.ObjectValue(domainAutorenewAttrTypes, map[string]attr.Value{
			"enabled":  types.BoolValue(a.Enabled),
			"duration": types.Int64Value(int64(a.Duration)),
			"org_id":   stringOrNull(a.OrgID),
			"dates":    dates,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Autorenew = obj
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// stringOrNull maps an absent (empty) API string to null rather than "".
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
