data "gandi_domain" "example" {
  fqdn = "example.com"
}

output "current_nameservers" {
  value = data.gandi_domain.example.nameservers
}

output "expires_at" {
  value = data.gandi_domain.example.registry_ends_at
}
