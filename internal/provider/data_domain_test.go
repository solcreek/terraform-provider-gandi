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
