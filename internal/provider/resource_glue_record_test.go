package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGlueRecord_basic(t *testing.T) {
	domain := testAccDomain()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create with one IP
				Config: testAccGlueRecordConfig(domain, `"203.0.113.10"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_glue_record.test", "name", "nsacctest"),
					resource.TestCheckResourceAttr("gandi_glue_record.test", "id",
						fmt.Sprintf("%s/nsacctest", domain)),
					resource.TestCheckResourceAttr("gandi_glue_record.test", "ips.#", "1"),
					resource.TestCheckTypeSetElemAttr("gandi_glue_record.test", "ips.*", "203.0.113.10"),
				),
			},
			{ // update to two IPs
				Config: testAccGlueRecordConfig(domain, `"203.0.113.10", "203.0.113.11"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_glue_record.test", "ips.#", "2"),
					resource.TestCheckTypeSetElemAttr("gandi_glue_record.test", "ips.*", "203.0.113.11"),
				),
			},
			{ // import
				ResourceName:      "gandi_glue_record.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccGlueRecordConfig(domain, ips string) string {
	return fmt.Sprintf(`
provider "gandi" {}

resource "gandi_glue_record" "test" {
  domain = %[1]q
  name   = "nsacctest"
  ips    = [%[2]s]
}
`, domain, ips)
}
