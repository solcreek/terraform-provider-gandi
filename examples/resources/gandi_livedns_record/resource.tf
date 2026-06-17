resource "gandi_livedns_record" "www" {
  domain = "example.com"
  name   = "www"
  type   = "A"
  ttl    = 3600
  values = ["203.0.113.10"]
}

# TXT values must be quoted, as Gandi stores them wrapped in double quotes.
resource "gandi_livedns_record" "spf" {
  domain = "example.com"
  name   = "@"
  type   = "TXT"
  ttl    = 10800
  values = ["\"v=spf1 include:_mailcust.gandi.net ~all\""]
}
