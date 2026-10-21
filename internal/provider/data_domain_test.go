package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
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
							resource.TestCheckResourceAttr("data.gandi_domain.test", "renew_begins_at", "2012-01-01T00:00:00Z"),
							resource.TestCheckResourceAttr("data.gandi_domain.test", "deletes_at", "2030-02-15T00:00:00Z"),
							resource.TestCheckNoResourceAttr("data.gandi_domain.test", "hold_begins_at"),
							resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.enabled", "false"),
							resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.duration", "1"),
							resource.TestCheckNoResourceAttr("data.gandi_domain.test", "autorenew.org_id"),
						),
					},
				},
			})
		})
	}
}

func TestDomainDataSource_autorenew(t *testing.T) {
	f := newFakeGandi(t)
	f.setAutorenew("example.build", gandi.Autorenew{
		Enabled: true, Duration: 2, OrgID: "org-1", Dates: []string{"2029-12-01T00:00:00Z"},
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: f.providerConfig() + `
data "gandi_domain" "test" {
  fqdn = "example.build"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.enabled", "true"),
					resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.duration", "2"),
					resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.org_id", "org-1"),
					resource.TestCheckResourceAttr("data.gandi_domain.test", "autorenew.dates.#", "1"),
				),
			},
		},
	})
}
