package provider

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDNSSECKeysDataSource(t *testing.T) {
	f := newFakeGandi(t)
	ksk := f.addKey("example.com", 13, "ksk", "KSKKEY==")
	f.addKey("example.com", 13, "zsk", "ZSKKEY==")
	f.addKey("other.example", 8, "ksk", "OTHER==")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: f.providerConfig() + `
data "gandi_dnssec_keys" "test" {
  domain = "example.com"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "id", "example.com"),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.#", "2"),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.id",
						fmt.Sprintf("example.com/%d", ksk.ID)),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.key_id", strconv.Itoa(ksk.ID)),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.algorithm", "13"),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.type", "ksk"),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.public_key", "KSKKEY=="),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.keytag", strconv.Itoa(ksk.KeyTag)),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.digest", ksk.Digest),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.0.digest_type", "2"),
					resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.1.type", "zsk"),
				),
			},
		},
	})
}

func TestDNSSECKeysDataSource_empty(t *testing.T) {
	f := newFakeGandi(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: f.providerConfig() + `
data "gandi_dnssec_keys" "test" {
  domain = "example.com"
}
`,
				Check: resource.TestCheckResourceAttr("data.gandi_dnssec_keys.test", "keys.#", "0"),
			},
		},
	})
}
