package provider

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/solcreek/terraform-provider-gandi/internal/gandi"
)

const testAutorenewDomain = "example.com"

func testDomainAutorenewConfig(f *fakeGandi, body string) string {
	return f.providerConfig() + fmt.Sprintf(`
resource "gandi_domain_autorenew" "test" {
  domain = %q
  %s
}
`, testAutorenewDomain, body)
}

// TestDomainAutorenew_lifecycle runs create → import → update → out-of-band
// change → destroy (left enabled) against the in-memory fake API.
func TestDomainAutorenew_lifecycle(t *testing.T) {
	f := newFakeGandi(t)
	const res = "gandi_domain_autorenew.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if a, _ := f.getAutorenew(testAutorenewDomain); !a.Enabled {
				return fmt.Errorf("destroy turned automatic renewal off without disable_on_destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testDomainAutorenewConfig(f, "enabled = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "id", testAutorenewDomain),
					resource.TestCheckResourceAttr(res, "enabled", "true"),
					resource.TestCheckResourceAttr(res, "duration", "1"),
					resource.TestCheckResourceAttr(res, "org_id", "org-default"),
					resource.TestCheckResourceAttr(res, "disable_on_destroy", "false"),
					testAutorenewInFake(f, true, 1),
				),
			},
			{
				ResourceName:      res,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testDomainAutorenewConfig(f, `
  enabled  = true
  duration = 3
  org_id   = "org-billing"
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "duration", "3"),
					resource.TestCheckResourceAttr(res, "org_id", "org-billing"),
					testAutorenewInFake(f, true, 3),
				),
			},
			{ // Turned off in the dashboard: the next apply turns it back on.
				PreConfig: func() {
					f.setAutorenew(testAutorenewDomain, gandi.Autorenew{Duration: 3, OrgID: "org-billing"})
				},
				Config: testDomainAutorenewConfig(f, `
  enabled  = true
  duration = 3
  org_id   = "org-billing"
`),
				Check: testAutorenewInFake(f, true, 3),
			},
			{ // Billing org cleared at Gandi: the configured one is restored.
				PreConfig: func() {
					f.setAutorenew(testAutorenewDomain, gandi.Autorenew{Enabled: true, Duration: 3})
				},
				Config: testDomainAutorenewConfig(f, `
  enabled  = true
  duration = 3
  org_id   = "org-billing"
`),
				Check: func(*terraform.State) error {
					if a, _ := f.getAutorenew(testAutorenewDomain); a.OrgID != "org-billing" {
						return fmt.Errorf("fake org_id = %q; want the configured org restored", a.OrgID)
					}
					return nil
				},
			},
		},
	})
}

// TestDomainAutorenew_disableOnDestroy turns automatic renewal off on destroy
// only when asked to.
func TestDomainAutorenew_disableOnDestroy(t *testing.T) {
	f := newFakeGandi(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if a, _ := f.getAutorenew(testAutorenewDomain); a.Enabled {
				return fmt.Errorf("automatic renewal still on after destroy with disable_on_destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testDomainAutorenewConfig(f, `
  enabled            = true
  disable_on_destroy = true
`),
				Check: testAutorenewInFake(f, true, 1),
			},
		},
	})
}

func TestDomainAutorenew_invalidDuration(t *testing.T) {
	f := newFakeGandi(t)
	for _, d := range []int{0, 10} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config:      testDomainAutorenewConfig(f, fmt.Sprintf("enabled = true\n  duration = %d", d)),
					ExpectError: regexp.MustCompile(`Expected 1–9 years`),
				},
			},
		})
	}
	if _, n := f.getAutorenew(testAutorenewDomain); n != 0 {
		t.Fatalf("invalid config reached the API (%d PATCH requests)", n)
	}
}

// TestDomainAutorenew_apiError surfaces a failed PATCH instead of saving state.
func TestDomainAutorenew_apiError(t *testing.T) {
	f := newFakeGandi(t)
	f.failRequests(http.MethodPatch)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testDomainAutorenewConfig(f, "enabled = true"),
				ExpectError: regexp.MustCompile(`Unable to set domain automatic renewal`),
			},
		},
	})
}

func testAutorenewInFake(f *fakeGandi, enabled bool, duration int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a, _ := f.getAutorenew(testAutorenewDomain)
		if a.Enabled != enabled || a.Duration != duration {
			return fmt.Errorf("fake autorenew = enabled %t, duration %d; want %t, %d",
				a.Enabled, a.Duration, enabled, duration)
		}
		return nil
	}
}

// TestAccDomainAutorenew_basic toggles automatic renewal on a real domain. It
// is free, but it changes whether the domain renews (and is billed), so it only
// runs when GANDI_TEST_AUTORENEW is set to the domain's current setting
// ("true" or "false"), which the test restores.
func TestAccDomainAutorenew_basic(t *testing.T) {
	current := os.Getenv("GANDI_TEST_AUTORENEW")
	if current != "true" && current != "false" {
		t.Skip("set GANDI_TEST_AUTORENEW to the test domain's current setting (true/false) to run the autorenew test")
	}
	domain := testAccDomain()
	flipped := "true"
	if current == "true" {
		flipped = "false"
	}
	cfg := func(enabled string) string {
		return fmt.Sprintf(`
provider "gandi" {}

resource "gandi_domain_autorenew" "test" {
  domain  = %q
  enabled = %s
}
`, domain, enabled)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(flipped),
				Check:  resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "enabled", flipped),
			},
			{
				ResourceName:      "gandi_domain_autorenew.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg(current),
				Check:  resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "enabled", current),
			},
		},
	})
}
