data "gandi_domain" "example" {
  fqdn = "example.com"
}

output "current_nameservers" {
  value = data.gandi_domain.example.nameservers
}

output "expires_at" {
  value = data.gandi_domain.example.registry_ends_at
}

# Whether the registry accepts DNSSEC keys (gandi_dnssec_key) for this domain.
output "dnssec_available" {
  value = data.gandi_domain.example.dnssec_available
}

# Automatic renewal setting and lifecycle dates, e.g. for a check block.
output "autorenew_enabled" {
  value = data.gandi_domain.example.autorenew.enabled
}

output "deletes_at" {
  value = data.gandi_domain.example.deletes_at
}
