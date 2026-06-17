<a href="https://opentofu.org">
    <img src=".github/tf.svg" alt="Terraform logo" title="Terraform / OpenTofu" align="left" height="50" />
</a>

# Terraform / OpenTofu Provider for Gandi

[![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/solcreek/terraform-provider-gandi?label=release&style=for-the-badge)](https://github.com/solcreek/terraform-provider-gandi/releases/latest) [![License](https://img.shields.io/github/license/solcreek/terraform-provider-gandi.svg?style=for-the-badge)](LICENSE) [![Tests](https://img.shields.io/github/actions/workflow/status/solcreek/terraform-provider-gandi/test.yml?branch=main&style=for-the-badge)](https://github.com/solcreek/terraform-provider-gandi/actions)

A small, focused provider for managing [Gandi](https://www.gandi.net) domains,
nameservers, glue records and LiveDNS records. It uses a **dependency-free,
standard-library-only** Gandi API client (no `go-gandi`), so the provider owns
its own HTTP behaviour: configurable timeout, rate-limit back-off and clear
credential errors.

Proudly built for the open-source IaC ecosystem and dedicated to **OpenTofu**:

![](https://raw.githubusercontent.com/opentofu/brand-artifacts/main/full/transparent/SVG/on-dark.svg#gh-dark-mode-only)
![](https://raw.githubusercontent.com/opentofu/brand-artifacts/main/full/transparent/SVG/on-light.svg#gh-light-mode-only)

## Usage

```hcl
terraform {
  required_providers {
    gandi = {
      source = "solcreek/gandi"
    }
  }
}

provider "gandi" {
  # personal_access_token = "..."   # or the GANDI_PAT environment variable
  timeout_seconds = 30              # optional, default 30
  # sharing_id    = "..."           # optional org scope; or GANDI_SHARING_ID
  # api_url       = "https://api.sandbox.gandi.net"  # optional; or GANDI_API_URL
}

resource "gandi_nameservers" "example" {
  domain      = "example.com"
  nameservers = ["dakota.ns.cloudflare.com", "zoe.ns.cloudflare.com"]
}
```

See [`examples/`](./examples) for nameservers, glue records, LiveDNS records and
the `gandi_domain` data source.

## Resources & data sources

| Kind | Name | Purpose |
|------|------|---------|
| data | `gandi_domain` | Look up a domain (nameservers, status, expiry dates). |
| resource | `gandi_nameservers` | Set a domain's registry nameservers. |
| resource | `gandi_glue_record` | Manage a glue record (host) → IPs. |
| resource | `gandi_livedns_record` | Manage a single LiveDNS rrset. |

All resources support `terraform import`.

## Authentication

This provider authenticates **only** with a Gandi
[Personal Access Token (PAT)](https://api.gandi.net/docs/authentication/),
supplied via the `personal_access_token` argument or the `GANDI_PAT`
environment variable.

> [!IMPORTANT]
> Gandi has **deprecated API keys** in favour of PATs. The old `Apikey`
> authentication scheme is intentionally **not supported** by this provider.

What happens with bad credentials:

- **No token** → the provider fails fast at configuration time with
  *"Missing Gandi credentials"*.
- **Invalid / expired token** → the API returns `401`, surfaced as a clear
  error hinting that the PAT is missing, invalid or expired.
- **Insufficient scope** → the API returns `403`, surfaced as a hint that the
  PAT lacks permission or organization scope for that resource.

## Limitations

- **Gandi v5 API only.** This provider targets the Gandi
  [v5 Public API](https://api.gandi.net/docs/) (`https://api.gandi.net/v5`).
- **PAT only.** No support for the deprecated API key. PATs **expire** — plan a
  rotation strategy. A PAT is bound to a **single organization**; use
  `sharing_id` to scope requests when needed.
- **Focused surface.** Only the four resources/data sources above are
  implemented (domains/DNS), not Gandi's full product catalogue (email,
  Simple Hosting, certificates, etc.).
- **LiveDNS vs registry.** `gandi_livedns_record` only resolves while the domain
  uses Gandi LiveDNS nameservers. If `gandi_nameservers` points elsewhere
  (e.g. Cloudflare), LiveDNS records still exist but stop resolving.
- **TXT values are quoted.** Gandi stores TXT values wrapped in literal double
  quotes, so write them quoted, e.g. `values = ["\"hello\""]`.
- **CNAME/MX/NS values** must be fully qualified with a trailing dot.
- **`gandi_nameservers` delete is a no-op** at the registry — a domain must
  always have nameservers, so destroy only drops it from Terraform state.

## Development

```sh
make build      # compile
make test       # unit tests (no network)
make testacc    # acceptance tests — needs GANDI_PAT and GANDI_TEST_DOMAIN
make lint       # golangci-lint
```

To run a local build, use a Terraform CLI dev override:

```hcl
# ~/.terraformrc  (or set TF_CLI_CONFIG_FILE)
provider_installation {
  dev_overrides { "solcreek/gandi" = "/abs/path/to/dir/with/binary" }
  direct {}
}
```

## License

[Mozilla Public License 2.0](LICENSE) — the license used by OpenTofu's core
community providers.
