<a href="https://terraform.io">
    <img src=".github/tf.svg" alt="Terraform logo" title="Terraform" align="left" height="50" />
</a>
<a href="https://opentofu.org">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset=".github/opentofu-dark.svg" />
        <img src=".github/opentofu-light.svg" alt="OpenTofu logo" title="OpenTofu" align="left" height="50" />
    </picture>
</a>

# Terraform / OpenTofu Provider for Gandi

[![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/solcreek/terraform-provider-gandi?label=release&style=for-the-badge)](https://github.com/solcreek/terraform-provider-gandi/releases/latest) [![License](https://img.shields.io/github/license/solcreek/terraform-provider-gandi.svg?style=for-the-badge)](LICENSE) [![Tests](https://img.shields.io/github/actions/workflow/status/solcreek/terraform-provider-gandi/test.yml?branch=main&style=for-the-badge)](https://github.com/solcreek/terraform-provider-gandi/actions)

A small, focused provider for managing [Gandi](https://www.gandi.net) domains,
nameservers, glue records, DNSSEC keys and LiveDNS records. It uses a **dependency-free,
standard-library-only** Gandi API client (no `go-gandi`), so the provider owns
its own HTTP behaviour: configurable timeout, rate-limit back-off and clear
credential errors.

> [!NOTE]
> This is an **unofficial, community-maintained** provider. It is **not**
> affiliated with, endorsed by, or supported by Gandi SAS. "Gandi" is a
> trademark of its respective owner and is used here only to describe what the
> provider integrates with.

Proudly built for the open-source IaC ecosystem and dedicated to **OpenTofu**.

## Usage

> [!IMPORTANT]
> The provider is published on the **[OpenTofu Registry](https://search.opentofu.org/provider/solcreek/gandi)**
> only, so always use the **full** `registry.opentofu.org/solcreek/gandi`
> address. It works with both OpenTofu and Terraform. The short
> `solcreek/gandi` form only works with OpenTofu: Terraform resolves short
> addresses against `registry.terraform.io`, where `terraform init` fails with
> *"Failed to query available provider packages"*.

```hcl
terraform {
  required_providers {
    gandi = {
      source = "registry.opentofu.org/solcreek/gandi"
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

See [`examples/`](./examples) for nameservers, glue records, DNSSEC keys,
LiveDNS records and the data sources.

## Resources & data sources

| Kind | Name | Purpose |
|------|------|---------|
| data | `gandi_domain` | Look up a domain (nameservers, status, expiry dates, DNSSEC availability). |
| data | `gandi_dnssec_keys` | List the DNSSEC keys registered for a domain. |
| resource | `gandi_nameservers` | Set a domain's registry nameservers. |
| resource | `gandi_glue_record` | Manage a glue record (host) → IPs. |
| resource | `gandi_livedns_record` | Manage a single LiveDNS rrset. |
| resource | `gandi_dnssec_key` | Submit a DNSKEY to the registry, which publishes the DS record. |

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

## Sandbox

Gandi runs a separate [sandbox environment](https://api.sandbox.gandi.net/docs/)
where you can register test domains and exercise the API for free, without
touching real domains or money.

> [!IMPORTANT]
> The sandbox is a **separate account system**. Your production PAT does **not**
> work there, and the sandbox still requires authentication — there is no
> anonymous or "random token" access (unauthenticated requests return `401`, an
> invalid token returns `403`). Create a sandbox account and a sandbox PAT in the
> Gandi Sandbox admin first.

Point the provider at it with the `sandbox` flag (or `api_url`):

```hcl
provider "gandi" {
  sandbox               = true        # or api_url = "https://api.sandbox.gandi.net"
  personal_access_token = var.sandbox_pat
}
```

`sandbox` can also be set via `GANDI_SANDBOX=true`. If both `api_url` and
`sandbox` are set, `api_url` wins.

## Limitations

- **Gandi v5 API only.** This provider targets the Gandi
  [v5 Public API](https://api.gandi.net/docs/) (`https://api.gandi.net/v5`).
- **PAT only.** No support for the deprecated API key. PATs **expire** — plan a
  rotation strategy. A PAT is bound to a **single organization**; use
  `sharing_id` to scope requests when needed.
- **Focused surface.** Only the resources/data sources above are
  implemented (domains/DNS), not Gandi's full product catalogue (email,
  Simple Hosting, certificates, etc.).
- **LiveDNS vs registry.** `gandi_livedns_record` only resolves while the domain
  uses Gandi LiveDNS nameservers. If `gandi_nameservers` points elsewhere
  (e.g. Cloudflare), LiveDNS records still exist but stop resolving.
- **TXT values are quoted.** Gandi stores TXT values wrapped in literal double
  quotes, so write them quoted, e.g. `values = ["\"hello\""]`.
- **CNAME/MX/NS values** must be fully qualified with a trailing dot.
- **DNSSEC takes a DNSKEY, not a DS.** Gandi derives the DS record from the
  public key; there is no way to submit a raw DS. `gandi_dnssec_key` has no
  update — every change replaces the key, so use `create_before_destroy` for a
  rollover, and only submit a key once the zone is actively signed with it.
  `data.gandi_domain.dnssec_available` tells you whether the registry supports
  it at all.
- **`gandi_nameservers` delete is a no-op** at the registry — a domain must
  always have nameservers, so destroy only drops it from Terraform state.

## Development

```sh
make build      # compile
make test       # unit tests (no network, no credentials)
make testacc    # acceptance tests — needs GANDI_PAT and GANDI_TEST_DOMAIN
make lint       # golangci-lint
make generate   # regenerate docs/ from schema + examples
```

Unit tests use an in-process HTTP stub and need **no credentials**. Acceptance
tests (`TF_ACC`) make real API calls — to avoid touching production domains you
can run them against the **sandbox** with a sandbox PAT:

```sh
GANDI_API_URL=https://api.sandbox.gandi.net \
GANDI_PAT=<sandbox-pat> GANDI_TEST_DOMAIN=<sandbox-domain> make testacc
```

The `gandi_nameservers` acceptance test mutates a domain's nameservers, so it is
additionally gated behind `GANDI_TEST_NAMESERVERS` and skips unless set.
Likewise the `gandi_dnssec_key` acceptance test publishes a DS record for the
test domain, so it only runs when `GANDI_TEST_DNSSEC_PUBLIC_KEY` (and optionally
`GANDI_TEST_DNSSEC_ALGORITHM`, default `13`) is set. Its lifecycle is also
covered by credential-free unit tests against an in-memory fake API.

To run a local build, use a CLI dev override (OpenTofu reads the same file
format as `~/.tofurc`):

```hcl
# ~/.terraformrc  (or set TF_CLI_CONFIG_FILE)
provider_installation {
  dev_overrides { "registry.opentofu.org/solcreek/gandi" = "/abs/path/to/dir/with/binary" }
  direct {}
}
```

## Releasing

Push a `v*` tag (e.g. `git tag v0.2.0 && git push origin v0.2.0`), after moving
the `Unreleased` entries in `CHANGELOG.md` under the new version. The Release
workflow runs GoReleaser, which builds every platform and signs the
`SHA256SUMS` file with the `GPG_PRIVATE_KEY` secret. It then publishes a GitHub
release.

No registry PR is needed per version: the OpenTofu Registry's
[`bump-versions`](https://github.com/opentofu/registry/blob/main/.github/workflows/bump-versions.yml)
job scans the **git tags** of registered providers, fetches the release assets
of any new semver tag and publishes the version. The job is scheduled every
15 minutes, but GitHub delays scheduled runs: observed gaps between runs were
7–63 minutes (median 19, measured 2026-09-28), so allow up to about an hour.
Check with:

```sh
curl -s https://registry.opentofu.org/v1/providers/solcreek/gandi/versions
```

Keep in mind:

- **A pushed tag is a published version, permanently.** The registry indexes
  every semver tag and never re-indexes or removes a version
  ([policy](https://github.com/opentofu/registry/blob/main/POLICY.md#version-immutability));
  to fix a bad release, release a new version. The repository enforces this:
  a `release-tags` ruleset forbids moving or deleting `v*` tags, and
  immutable releases forbid replacing published assets. GoReleaser uploads to
  a draft and publishes last, so it works with both.
- **Do not change the signing key.** `tofu init` verifies `SHA256SUMS.sig`
  against the key registered in
  [opentofu/registry](https://github.com/opentofu/registry/tree/main/keys/s/solcreek);
  a new key must be submitted there first.
- **Keep the asset names.** `.goreleaser.yml` produces the
  `_SHA256SUMS`, `_SHA256SUMS.sig`, `_manifest.json` and per-platform zip
  names the registry protocol expects.

These repository settings (plus the description and topics shown on
search.opentofu.org) are declared in `scripts/repo-settings.sh`. Run it to
check for drift, and with `--apply` (admin rights) to reconcile.

## License

[Mozilla Public License 2.0](LICENSE) — the license used by OpenTofu's core
community providers.
