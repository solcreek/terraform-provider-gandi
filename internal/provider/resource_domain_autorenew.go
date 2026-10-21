package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ resource.Resource = (*domainAutorenewResource)(nil)
var _ resource.ResourceWithConfigure = (*domainAutorenewResource)(nil)
var _ resource.ResourceWithImportState = (*domainAutorenewResource)(nil)
var _ resource.ResourceWithValidateConfig = (*domainAutorenewResource)(nil)

// Gandi accepts an automatic renewal duration of 1–9 years.
const (
	autorenewMinDuration = 1
	autorenewMaxDuration = 9
)

type domainAutorenewResource struct {
	client *gandi.Client
}

func newDomainAutorenewResource() resource.Resource { return &domainAutorenewResource{} }

type domainAutorenewModel struct {
	Domain           types.String `tfsdk:"domain"`
	ID               types.String `tfsdk:"id"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Duration         types.Int64  `tfsdk:"duration"`
	OrgID            types.String `tfsdk:"org_id"`
	DisableOnDestroy types.Bool   `tfsdk:"disable_on_destroy"`
}

func (r *domainAutorenewResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_autorenew"
}

func (r *domainAutorenewResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the automatic renewal setting of a Gandi domain. " +
			"Changing the setting is free; Gandi charges when it renews the domain. " +
			"By default, deleting this resource stops Terraform managing the setting but leaves it as it is at Gandi " +
			"(see `disable_on_destroy`).",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) whose automatic renewal is managed.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN); same as `domain`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether Gandi renews the domain automatically before it expires.",
				Required:            true,
			},
			"duration": schema.Int64Attribute{
				MarkdownDescription: fmt.Sprintf("Years added at each automatic renewal (%d–%d). Defaults to 1.",
					autorenewMinDuration, autorenewMaxDuration),
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1),
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the organization billed for automatic renewals. " +
					"If unset, Gandi chooses it and the value it reports is kept.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"disable_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Turn automatic renewal off when this resource is destroyed. " +
					"Defaults to `false`, so removing the resource never lets a domain lapse by accident.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
		},
	}
}

func (r *domainAutorenewResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg domainAutorenewModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.Duration.IsNull() && !cfg.Duration.IsUnknown() {
		if d := cfg.Duration.ValueInt64(); d < autorenewMinDuration || d > autorenewMaxDuration {
			resp.Diagnostics.AddAttributeError(path.Root("duration"), "Invalid duration",
				fmt.Sprintf("Expected %d–%d years, got %d.", autorenewMinDuration, autorenewMaxDuration, d))
		}
	}
}

func (r *domainAutorenewResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*gandi.Client)
}

func (r *domainAutorenewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainAutorenewModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainAutorenewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainAutorenewModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dom, err := r.client.GetDomain(ctx, state.Domain.ValueString())
	if err != nil {
		if gandi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read domain automatic renewal", err.Error())
		return
	}

	setAutorenewState(&state, dom.Autorenew)
	// Unset after import; keep the schema default.
	if state.DisableOnDestroy.IsNull() {
		state.DisableOnDestroy = types.BoolValue(false)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainAutorenewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainAutorenewModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete leaves automatic renewal as it is unless disable_on_destroy is set:
// silently turning it off could let a domain expire.
func (r *domainAutorenewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainAutorenewModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !state.DisableOnDestroy.ValueBool() {
		return
	}
	err := r.client.SetAutorenew(ctx, state.Domain.ValueString(), gandi.AutorenewRequest{Enabled: false})
	if err != nil && !gandi.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to disable domain automatic renewal", err.Error())
	}
}

func (r *domainAutorenewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain"), req, resp)
}

// apply writes the planned setting. The plan is what lands in state (Gandi
// echoing something else would be an inconsistent result); only an org_id
// left for Gandi to choose is read back.
func (r *domainAutorenewResource) apply(ctx context.Context, plan *domainAutorenewModel, diags *diag.Diagnostics) {
	fqdn := plan.Domain.ValueString()
	body := gandi.AutorenewRequest{
		Enabled:  plan.Enabled.ValueBool(),
		Duration: int(plan.Duration.ValueInt64()),
		OrgID:    plan.OrgID.ValueString(),
	}
	if err := r.client.SetAutorenew(ctx, fqdn, body); err != nil {
		diags.AddError("Unable to set domain automatic renewal", err.Error())
		return
	}
	plan.ID = types.StringValue(fqdn)
	if !plan.OrgID.IsUnknown() {
		return
	}
	dom, err := r.client.GetDomain(ctx, fqdn)
	if err != nil {
		diags.AddError("Unable to read domain automatic renewal", err.Error())
		return
	}
	plan.OrgID = types.StringNull()
	if dom.Autorenew != nil && dom.Autorenew.OrgID != "" {
		plan.OrgID = types.StringValue(dom.Autorenew.OrgID)
	}
}

// setAutorenewState copies Gandi's setting into m, so out-of-band changes
// show up as drift.
func setAutorenewState(m *domainAutorenewModel, a *gandi.Autorenew) {
	if a == nil {
		a = &gandi.Autorenew{}
	}
	m.ID = types.StringValue(m.Domain.ValueString())
	m.Enabled = types.BoolValue(a.Enabled)
	if a.Duration > 0 {
		m.Duration = types.Int64Value(int64(a.Duration))
	} else if m.Duration.IsNull() {
		m.Duration = types.Int64Value(1)
	}
	// Null, not the previous value, when Gandi reports none: a configured
	// org_id then shows as drift and is restored.
	m.OrgID = stringOrNull(a.OrgID)
}
