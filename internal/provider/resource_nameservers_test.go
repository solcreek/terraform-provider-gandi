package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccNameservers_basic mutates the registry nameservers of a real domain,
// which affects DNS resolution. It only runs when GANDI_TEST_NAMESERVERS is set
// (a comma-separated list); supply the domain's *current* nameservers to make
// the run a safe no-op, or use a throwaway domain.
func TestAccNameservers_basic(t *testing.T) {
	raw := os.Getenv("GANDI_TEST_NAMESERVERS")
	if raw == "" {
		t.Skip("set GANDI_TEST_NAMESERVERS (comma-separated) to run the destructive nameservers test")
	}
	domain := testAccDomain()
	ns := strings.Split(raw, ",")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNameserversConfig(domain, ns),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_nameservers.test", "domain", domain),
					resource.TestCheckResourceAttr("gandi_nameservers.test", "id", domain),
					resource.TestCheckResourceAttr("gandi_nameservers.test", "nameservers.#",
						strconv.Itoa(len(ns))),
					resource.TestCheckResourceAttr("gandi_nameservers.test", "nameservers.0",
						strings.TrimSpace(ns[0])),
				),
			},
			{
				ResourceName:      "gandi_nameservers.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNameserversConfig(domain string, ns []string) string {
	quoted := make([]string, len(ns))
	for i, n := range ns {
		quoted[i] = strconv.Quote(strings.TrimSpace(n))
	}
	return fmt.Sprintf(`
provider "gandi" {}

resource "gandi_nameservers" "test" {
  domain      = %[1]q
  nameservers = [%[2]s]
}
`, domain, strings.Join(quoted, ", "))
}
