package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

var _ resource.Resource = (*dnssecKeyResource)(nil)
var _ resource.ResourceWithConfigure = (*dnssecKeyResource)(nil)
var _ resource.ResourceWithImportState = (*dnssecKeyResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dnssecKeyResource)(nil)

type dnssecKeyResource struct {
	client *gandi.Client
}

func newDNSSECKeyResource() resource.Resource { return &dnssecKeyResource{} }

type dnssecKeyModel struct {
	Domain     types.String `tfsdk:"domain"`
	Algorithm  types.Int64  `tfsdk:"algorithm"`
	Type       types.String `tfsdk:"type"`
	PublicKey  types.String `tfsdk:"public_key"`
	ID         types.String `tfsdk:"id"`
	KeyID      types.Int64  `tfsdk:"key_id"`
	KeyTag     types.Int64  `tfsdk:"keytag"`
	Digest     types.String `tfsdk:"digest"`
	DigestType types.Int64  `tfsdk:"digest_type"`
}

var dnssecKeyTypes = []string{"ksk", "zsk", "none"}

func (r *dnssecKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dnssec_key"
}

func (r *dnssecKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedInt := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{
			MarkdownDescription: desc,
			Computed:            true,
			PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Submits a DNSSEC public key (DNSKEY) for a Gandi domain to its registry, which " +
			"publishes the corresponding DS record in the parent zone. Use it to complete the chain of trust " +
			"when the zone is signed by an external DNS host (e.g. Cloudflare).\n\n" +
			"The API has no update operation, so changing any argument replaces the key. For a KSK rollover, " +
			"set `lifecycle { create_before_destroy = true }` so the new DS is published before the old one is " +
			"removed; otherwise validating resolvers will SERVFAIL the domain in between.\n\n" +
			"Only submit the key once the zone is actively signed with it — a DS pointing at an unsigned zone " +
			"also breaks resolution.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain (FQDN) the key belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"algorithm": schema.Int64Attribute{
				MarkdownDescription: "IANA DNSSEC algorithm number, e.g. `13` (ECDSAP256SHA256) or `8` (RSASHA256).",
				Required:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Key type: `ksk` (DNSKEY flags 257), `zsk` (flags 256) or `none`. Defaults to `ksk`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ksk"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"public_key": schema.StringAttribute{
				MarkdownDescription: "Base64 DNSKEY public key. Whitespace is ignored when comparing with the registry.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "`<domain>/<key_id>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key_id":      computedInt("Gandi's numeric id for the key."),
			"keytag":      computedInt("Key tag of the resulting DS record."),
			"digest":      schema.StringAttribute{MarkdownDescription: "Digest of the resulting DS record.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"digest_type": computedInt("Digest type of the resulting DS record, e.g. `2` (SHA-256)."),
		},
	}
}

func (r *dnssecKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg dnssecKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.Type.IsNull() && !cfg.Type.IsUnknown() {
		valid := false
		for _, t := range dnssecKeyTypes {
			if cfg.Type.ValueString() == t {
				valid = true
			}
		}
		if !valid {
			resp.Diagnostics.AddAttributeError(path.Root("type"), "Invalid key type",
				fmt.Sprintf("Expected one of %s, got %q.", strings.Join(dnssecKeyTypes, ", "), cfg.Type.ValueString()))
		}
	}
	if !cfg.Algorithm.IsNull() && !cfg.Algorithm.IsUnknown() {
		if a := cfg.Algorithm.ValueInt64(); a < 0 || a > 255 {
			resp.Diagnostics.AddAttributeError(path.Root("algorithm"), "Invalid algorithm",
				fmt.Sprintf("Expected an IANA DNSSEC algorithm number (0–255), got %d.", a))
		}
	}
}

func (r *dnssecKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*gandi.Client)
}

func (r *dnssecKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnssecKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain := plan.Domain.ValueString()
	pk := plan.PublicKey.ValueString()

	// The create response carries no id; we find the key again by its public
	// key. Refuse to adopt a pre-existing one silently.
	existing, err := r.client.FindDNSSECKeyByPublicKey(ctx, domain, pk)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list DNSSEC keys", err.Error())
		return
	}
	if existing != nil {
		resp.Diagnostics.AddError("DNSSEC key already exists",
			fmt.Sprintf("%s already has this public key (key id %d). Import it with `terraform import <address> %s/%d`.",
				domain, existing.ID, domain, existing.ID))
		return
	}

	if err := r.client.CreateDNSSECKey(ctx, domain, int(plan.Algorithm.ValueInt64()), plan.Type.ValueString(), pk); err != nil {
		resp.Diagnostics.AddError("Unable to create DNSSEC key", err.Error())
		return
	}
	// Creation is asynchronous; wait until the key is listed.
	key, err := r.client.WaitForDNSSECKey(ctx, domain, pk)
	if err != nil {
		resp.Diagnostics.AddError("DNSSEC key did not appear", err.Error())
		return
	}
	setDNSSECKeyComputed(&plan, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnssecKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnssecKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.GetDNSSECKey(ctx, state.Domain.ValueString(), int(state.KeyID.ValueInt64()))
	if err != nil {
		if gandi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read DNSSEC key", err.Error())
		return
	}

	state.Algorithm = types.Int64Value(int64(key.Algorithm))
	state.Type = types.StringValue(key.Type)
	// Keep the configured spelling unless the key actually differs, so a key
	// written with whitespace does not show a perpetual diff.
	if gandi.NormalizePublicKey(state.PublicKey.ValueString()) != gandi.NormalizePublicKey(key.PublicKey) {
		state.PublicKey = types.StringValue(key.PublicKey)
	}
	setDNSSECKeyComputed(&state, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never reached with a real change: every argument requires
// replacement. It only persists the plan.
func (r *dnssecKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnssecKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnssecKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnssecKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, id := state.Domain.ValueString(), int(state.KeyID.ValueInt64())
	if err := r.client.DeleteDNSSECKey(ctx, domain, id); err != nil {
		if !gandi.IsNotFound(err) {
			resp.Diagnostics.AddError("Unable to delete DNSSEC key", err.Error())
		}
		return
	}
	// Deletion is asynchronous; wait until it is gone so re-creates are clean.
	if err := r.client.WaitForDNSSECKeyGone(ctx, domain, id); err != nil {
		resp.Diagnostics.AddError("DNSSEC key was not deleted in time", err.Error())
	}
}

func (r *dnssecKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, rawID, ok := strings.Cut(req.ID, "/")
	keyID, err := strconv.ParseInt(rawID, 10, 64)
	if !ok || domain == "" || err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `<domain>/<key_id>` with a numeric key id, got %q. "+
				"List key ids with the `gandi_dnssec_keys` data source.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key_id"), keyID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func setDNSSECKeyComputed(m *dnssecKeyModel, k *gandi.DNSSECKey) {
	m.ID = types.StringValue(fmt.Sprintf("%s/%d", m.Domain.ValueString(), k.ID))
	m.KeyID = types.Int64Value(int64(k.ID))
	m.KeyTag = types.Int64Value(int64(k.KeyTag))
	m.Digest = types.StringValue(k.Digest)
	m.DigestType = types.Int64Value(int64(k.DigestType))
}
