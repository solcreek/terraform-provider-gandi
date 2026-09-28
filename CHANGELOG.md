# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-09-28

### Added

- `gandi_dnssec_key` resource: submit a DNSKEY to the registry so Gandi
  publishes the DS record, completing the DNSSEC chain of trust for zones signed
  by an external DNS host (e.g. Cloudflare). Import by `<domain>/<key_id>`.
  ([#1](https://github.com/solcreek/terraform-provider-gandi/issues/1))
- `gandi_dnssec_keys` data source: list the DNSSEC keys registered for a domain.
  ([#1](https://github.com/solcreek/terraform-provider-gandi/issues/1))
- `dnssec_available` attribute on the `gandi_domain` data source.
  ([#1](https://github.com/solcreek/terraform-provider-gandi/issues/1))

### Fixed

- Documented the full `registry.opentofu.org/solcreek/gandi` source address:
  the short `solcreek/gandi` form fails under Terraform, which resolves it
  against `registry.terraform.io`.
  ([#3](https://github.com/solcreek/terraform-provider-gandi/pull/3))
- `-debug` mode now advertises the provider under
  `registry.opentofu.org/solcreek/gandi`, so debugger reattach matches the
  configured source.
  ([#3](https://github.com/solcreek/terraform-provider-gandi/pull/3))

## [0.1.0] - 2026-06-17

### Added

- `gandi_domain` data source.
- `gandi_nameservers`, `gandi_glue_record` and `gandi_livedns_record` resources,
  all importable.
- Dependency-free Gandi v5 API client with Personal Access Token auth,
  configurable timeout, HTTP 429 back-off and actionable credential errors.
- `sandbox` provider argument (and `GANDI_SANDBOX`) for the Gandi sandbox API.

[unreleased]: https://github.com/solcreek/terraform-provider-gandi/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/solcreek/terraform-provider-gandi/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/solcreek/terraform-provider-gandi/releases/tag/v0.1.0
