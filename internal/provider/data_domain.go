package provider

import (
	"context"

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
		},
	}
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
