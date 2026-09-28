data "gandi_dnssec_keys" "example" {
  domain = "example.com"
}

# Import ids for keys added in the Gandi dashboard.
output "dnssec_key_ids" {
  value = [for k in data.gandi_dnssec_keys.example.keys : k.id]
}
