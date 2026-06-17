package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLiveDNSRecord_basic(t *testing.T) {
	domain := testAccDomain()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create
				Config: testAccLiveDNSRecordConfig(domain, 300, "acctest-v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_livedns_record.test", "name", "_acctest"),
					resource.TestCheckResourceAttr("gandi_livedns_record.test", "type", "TXT"),
					resource.TestCheckResourceAttr("gandi_livedns_record.test", "ttl", "300"),
					resource.TestCheckResourceAttr("gandi_livedns_record.test", "id",
						fmt.Sprintf("%s/_acctest/TXT", domain)),
				),
			},
			{ // update ttl + value
				Config: testAccLiveDNSRecordConfig(domain, 600, "acctest-v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_livedns_record.test", "ttl", "600"),
				),
			},
			{ // import
				ResourceName:      "gandi_livedns_record.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLiveDNSRecordConfig(domain string, ttl int, value string) string {
	// Gandi stores TXT values wrapped in literal double quotes, so the managed
	// value must include them to keep the plan stable across refreshes.
	return fmt.Sprintf(`
provider "gandi" {}

resource "gandi_livedns_record" "test" {
  domain = %[1]q
  name   = "_acctest"
  type   = "TXT"
  ttl    = %[2]d
  values = ["\"%[3]s\""]
}
`, domain, ttl, value)
}
