resource "gandi_glue_record" "ns1" {
  domain = "example.com"
  name   = "ns1"
  ips    = ["203.0.113.53"]
}
