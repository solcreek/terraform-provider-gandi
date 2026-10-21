package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
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
// only when asked to. The domain already has a billing org, which neither the
// unconfigured org_id nor the destroy PATCH may change.
func TestDomainAutorenew_disableOnDestroy(t *testing.T) {
	f := newFakeGandi(t)
	f.setAutorenew(testAutorenewDomain, gandi.Autorenew{Duration: 1, OrgID: "org-existing"})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			a, _ := f.getAutorenew(testAutorenewDomain)
			if a.Enabled {
				return fmt.Errorf("automatic renewal still on after destroy with disable_on_destroy")
			}
			if a.OrgID != "org-existing" {
				return fmt.Errorf("destroy changed the billing org to %q", a.OrgID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testDomainAutorenewConfig(f, `
  enabled            = true
  disable_on_destroy = true
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAutorenewInFake(f, true, 1),
					resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "org_id", "org-existing"),
				),
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
// runs when GANDI_TEST_AUTORENEW is set. It reads the domain's setting first,
// keeps its duration and billing org throughout, and restores the original
// setting through the API even if a step fails.
func TestAccDomainAutorenew_basic(t *testing.T) {
	if os.Getenv("GANDI_TEST_AUTORENEW") == "" {
		t.Skip("set GANDI_TEST_AUTORENEW to run the autorenew test, which flips and restores the test domain's setting")
	}
	// resource.Test checks TF_ACC, but the read and restore below run first.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test: set TF_ACC=1")
	}
	testAccPreCheck(t)
	domain := testAccDomain()
	// Resolve the API the way the provider does, so the read and restore hit
	// the same environment the resource changes.
	apiURL := os.Getenv("GANDI_API_URL")
	if apiURL == "" && envTrue(os.Getenv("GANDI_SANDBOX")) {
		apiURL = gandi.SandboxBaseURL
	}
	client := gandi.New(os.Getenv("GANDI_PAT"),
		gandi.WithBaseURL(apiURL),
		gandi.WithSharingID(os.Getenv("GANDI_SHARING_ID")),
	)

	dom, err := client.GetDomain(context.Background(), domain)
	if err != nil {
		t.Fatalf("read original autorenew setting: %v", err)
	}
	orig := gandi.Autorenew{Duration: 1}
	if dom.Autorenew != nil {
		orig = *dom.Autorenew
	}
	if orig.Duration < autorenewMinDuration {
		orig.Duration = autorenewMinDuration
	}
	t.Cleanup(func() {
		restore := gandi.AutorenewRequest{Enabled: orig.Enabled, Duration: orig.Duration, OrgID: orig.OrgID}
		if err := client.SetAutorenew(context.Background(), domain, restore); err != nil {
			t.Errorf("RESTORE FAILED: %s automatic renewal must be reset by hand to enabled=%t duration=%d: %v",
				domain, orig.Enabled, orig.Duration, err)
		}
	})

	orgLine := ""
	if orig.OrgID != "" {
		orgLine = fmt.Sprintf("org_id = %q", orig.OrgID)
	}
	cfg := func(enabled bool) string {
		return fmt.Sprintf(`
provider "gandi" {}

resource "gandi_domain_autorenew" "test" {
  domain   = %q
  enabled  = %t
  duration = %d
  %s
}
`, domain, enabled, orig.Duration, orgLine)
	}
	enabled := strconv.FormatBool

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(!orig.Enabled),
				Check:  resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "enabled", enabled(!orig.Enabled)),
			},
			{
				ResourceName:      "gandi_domain_autorenew.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg(orig.Enabled),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "enabled", enabled(orig.Enabled)),
					resource.TestCheckResourceAttr("gandi_domain_autorenew.test", "duration", strconv.Itoa(orig.Duration)),
				),
			},
		},
	})
}
