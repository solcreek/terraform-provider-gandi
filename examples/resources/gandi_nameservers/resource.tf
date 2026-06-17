resource "gandi_nameservers" "example" {
  domain = "example.com"
  nameservers = [
    "dakota.ns.cloudflare.com",
    "zoe.ns.cloudflare.com",
  ]
}
