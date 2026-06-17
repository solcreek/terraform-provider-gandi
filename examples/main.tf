terraform {
  required_providers {
    gandi = {
      source = "solcreek/gandi"
    }
  }
}

provider "gandi" {
  # Reads GANDI_PAT from the environment by default.
  timeout_seconds = 30
}

# Look up an existing domain.
data "gandi_domain" "example" {
  fqdn = "example.com"
}

output "current_nameservers" {
  value = data.gandi_domain.example.nameservers
}

output "expires_at" {
  value = data.gandi_domain.example.registry_ends_at
}

# Point the domain at external nameservers (e.g. Cloudflare).
resource "gandi_nameservers" "example" {
  domain = "example.com"
  nameservers = [
    "dakota.ns.cloudflare.com",
    "zoe.ns.cloudflare.com",
  ]
}

# A LiveDNS record (only resolves while the domain uses Gandi LiveDNS NS).
resource "gandi_livedns_record" "www" {
  domain = "example.com"
  name   = "www"
  type   = "A"
  ttl    = 3600
  values = ["203.0.113.10"]
}

# A glue record: ns1.example.com -> 203.0.113.53
resource "gandi_glue_record" "ns1" {
  domain = "example.com"
  name   = "ns1"
  ips    = ["203.0.113.53"]
}
