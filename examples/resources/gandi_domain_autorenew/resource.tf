resource "gandi_domain_autorenew" "example" {
  domain   = "example.com"
  enabled  = true
  duration = 1 # years added at each renewal, 1–9

  # org_id           = "..."  # organization billed; Gandi's choice if unset
  # disable_on_destroy = true # turn renewal off when this resource is removed
}
