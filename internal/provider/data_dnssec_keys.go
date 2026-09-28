package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ datasource.DataSource = (*dnssecKeysDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*dnssecKeysDataSource)(nil)

type dnssecKeysDataSource struct {
	client *gandi.Client
}

func newDNSSECKeysDataSource() datasource.DataSource { return &dnssecKeysDataSource{} }

type dnssecKeysModel struct {
	Domain types.String          `tfsdk:"domain"`
	ID     types.String          `tfsdk:"id"`
	Keys   []dnssecKeyEntryModel `tfsdk:"keys"`
}

type dnssecKeyEntryModel struct {
	ID         types.String `tfsdk:"id"`
	KeyID      types.Int64  `tfsdk:"key_id"`
	Algorithm  types.Int64  `tfsdk:"algorithm"`
	Type       types.String `tfsdk:"type"`
	PublicKey  types.String `tfsdk:"public_key"`
	KeyTag     types.Int64  `tfsdk:"keytag"`
	Digest     types.String `tfsdk:"digest"`
	DigestType types.Int64  `tfsdk:"digest_type"`
}

func (d *dnssecKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dnssec_keys"
}

func (d *dnssecKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List the DNSSEC keys registered for a Gandi domain, e.g. to find the id of a key " +
			"added in the dashboard before importing it as a `gandi_dnssec_key`.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) to list keys for.",
				Required:            true,
			},
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "The domain."},
			"keys": schema.ListNestedAttribute{
				MarkdownDescription: "Keys registered at the registry.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "`<domain>/<key_id>`, usable as a `gandi_dnssec_key` import id."},
						"key_id":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Gandi's numeric id for the key."},
						"algorithm":   schema.Int64Attribute{Computed: true, MarkdownDescription: "IANA DNSSEC algorithm number."},
						"type":        schema.StringAttribute{Computed: true, MarkdownDescription: "`ksk`, `zsk` or `none`."},
						"public_key":  schema.StringAttribute{Computed: true, MarkdownDescription: "Base64 DNSKEY public key."},
						"keytag":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Key tag of the DS record."},
						"digest":      schema.StringAttribute{Computed: true, MarkdownDescription: "Digest of the DS record."},
						"digest_type": schema.Int64Attribute{Computed: true, MarkdownDescription: "Digest type of the DS record."},
					},
				},
			},
		},
	}
}

func (d *dnssecKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*gandi.Client)
}

func (d *dnssecKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data dnssecKeysModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain := data.Domain.ValueString()

	keys, err := d.client.ListDNSSECKeys(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list DNSSEC keys", err.Error())
		return
	}

	data.ID = types.StringValue(domain)
	data.Keys = make([]dnssecKeyEntryModel, 0, len(keys))
	for _, k := range keys {
		data.Keys = append(data.Keys, dnssecKeyEntryModel{
			ID:         types.StringValue(fmt.Sprintf("%s/%d", domain, k.ID)),
			KeyID:      types.Int64Value(int64(k.ID)),
			Algorithm:  types.Int64Value(int64(k.Algorithm)),
			Type:       types.StringValue(k.Type),
			PublicKey:  types.StringValue(k.PublicKey),
			KeyTag:     types.Int64Value(int64(k.KeyTag)),
			Digest:     types.StringValue(k.Digest),
			DigestType: types.Int64Value(int64(k.DigestType)),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
