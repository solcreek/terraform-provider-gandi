package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainDataSource_basic(t *testing.T) {
	domain := testAccDomain()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainDataSourceConfig(domain),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.gandi_domain.test", "fqdn", domain),
					resource.TestCheckResourceAttrSet("data.gandi_domain.test", "nameservers.#"),
					resource.TestMatchResourceAttr("data.gandi_domain.test", "registry_ends_at",
						regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)),
				),
			},
		},
	})
}

func testAccDomainDataSourceConfig(domain string) string {
	return fmt.Sprintf(`
provider "gandi" {}

data "gandi_domain" "test" {
  fqdn = %[1]q
}
`, domain)
}

// TestDomainDataSource_dnssecAvailable covers the flag being true, false and
// absent from the API response (null, not a misleading false).
func TestDomainDataSource_dnssecAvailable(t *testing.T) {
	cfg := func(f *fakeGandi) string {
		return f.providerConfig() + `
data "gandi_domain" "test" {
  fqdn = "example.build"
}
`
	}
	for name, tc := range map[string]struct {
		avail *bool
		check resource.TestCheckFunc
	}{
		"true":   {ptr(true), resource.TestCheckResourceAttr("data.gandi_domain.test", "dnssec_available", "true")},
		"false":  {ptr(false), resource.TestCheckResourceAttr("data.gandi_domain.test", "dnssec_available", "false")},
		"absent": {nil, resource.TestCheckNoResourceAttr("data.gandi_domain.test", "dnssec_available")},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGandi(t)
			f.dnssecAvailable = tc.avail
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: cfg(f),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.gandi_domain.test", "tld", "build"),
							tc.check,
						),
					},
				},
			})
		})
	}
}
