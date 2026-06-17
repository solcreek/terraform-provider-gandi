package provider

import (
	"context"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

// Ensure the provider satisfies the framework interface.
var _ provider.Provider = (*gandiProvider)(nil)

type gandiProvider struct {
	version string
}

type providerModel struct {
	PersonalAccessToken types.String `tfsdk:"personal_access_token"`
	APIURL              types.String `tfsdk:"api_url"`
	SharingID           types.String `tfsdk:"sharing_id"`
	TimeoutSeconds      types.Int64  `tfsdk:"timeout_seconds"`
}

// New returns a provider factory for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &gandiProvider{version: version}
	}
}

func (p *gandiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gandi"
	resp.Version = p.version
}

func (p *gandiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Gandi domains, nameservers, glue records and LiveDNS records. " +
			"Authenticates with a Personal Access Token (PAT); the deprecated API key is not supported.",
		Attributes: map[string]schema.Attribute{
			"personal_access_token": schema.StringAttribute{
				MarkdownDescription: "Gandi Personal Access Token. Falls back to the `GANDI_PAT` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "Gandi API base URL. Defaults to `https://api.gandi.net`. Falls back to `GANDI_API_URL`.",
				Optional:            true,
			},
			"sharing_id": schema.StringAttribute{
				MarkdownDescription: "Organization ID (sharing_id) to scope requests. Falls back to `GANDI_SHARING_ID`.",
				Optional:            true,
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Per-request HTTP timeout in seconds. Defaults to 30.",
				Optional:            true,
			},
		},
	}
}

func (p *gandiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pat := firstNonEmpty(cfg.PersonalAccessToken.ValueString(), os.Getenv("GANDI_PAT"))
	if pat == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("personal_access_token"),
			"Missing Gandi credentials",
			"Set the provider `personal_access_token` argument or the GANDI_PAT environment variable.",
		)
		return
	}

	apiURL := firstNonEmpty(cfg.APIURL.ValueString(), os.Getenv("GANDI_API_URL"))
	sharingID := firstNonEmpty(cfg.SharingID.ValueString(), os.Getenv("GANDI_SHARING_ID"))

	timeout := 30 * time.Second
	if !cfg.TimeoutSeconds.IsNull() && cfg.TimeoutSeconds.ValueInt64() > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds.ValueInt64()) * time.Second
	}

	client := gandi.New(pat,
		gandi.WithBaseURL(apiURL),
		gandi.WithSharingID(sharingID),
		gandi.WithTimeout(timeout),
	)

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *gandiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newNameserversResource,
		newGlueRecordResource,
		newLiveDNSRecordResource,
	}
}

func (p *gandiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newDomainDataSource,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
