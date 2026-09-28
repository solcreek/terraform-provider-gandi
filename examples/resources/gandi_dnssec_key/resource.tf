# Complete the DNSSEC chain of trust for a domain whose DNS is hosted and signed
# on Cloudflare: Cloudflare signs the zone, Gandi publishes the DS at the
# registry. Referencing cloudflare_zone_dnssec orders the DS submission after
# signing is enabled — publishing a DS for an unsigned zone breaks resolution.
resource "cloudflare_zone_dnssec" "example" {
  zone_id = cloudflare_zone.example.id
  status  = "active"
}

resource "gandi_dnssec_key" "example" {
  domain     = "example.com"
  algorithm  = tonumber(cloudflare_zone_dnssec.example.algorithm) # 13
  type       = "ksk"                                              # DNSKEY flags 257
  public_key = cloudflare_zone_dnssec.example.public_key

  # Every argument forces replacement. On a key rollover, submit the new DS
  # before removing the old one, or validating resolvers SERVFAIL the domain.
  lifecycle {
    create_before_destroy = true
  }
}

output "ds_record" {
  value = "${gandi_dnssec_key.example.keytag} ${gandi_dnssec_key.example.algorithm} ${gandi_dnssec_key.example.digest_type} ${gandi_dnssec_key.example.digest}"
}
