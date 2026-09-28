package provider

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	testDNSSECDomain = "example.com"
	// An ECDSAP256SHA256 (algorithm 13) public key, as Cloudflare reports it.
	testDNSSECPublicKey = "mdsswUyr3DPW132mOi8V9xESWE8jTo0dxCjjnopKl+GqJxpVXckHAeF+KkxLbxILfDLUT0rAK9iUzy1L53eKGQ=="
)

func testDNSSECKeyConfig(f *fakeGandi, extra string) string {
	return testDNSSECKeyConfigPK(f, testDNSSECPublicKey, extra)
}

func testDNSSECKeyConfigPK(f *fakeGandi, pk, extra string) string {
	return f.providerConfig() + fmt.Sprintf(`
resource "gandi_dnssec_key" "test" {
  domain     = %q
  algorithm  = 13
  public_key = %q
  %s
}
`, testDNSSECDomain, pk, extra)
}

// TestDNSSECKey_lifecycle runs create → no-op plan → import → out-of-band
// deletion → re-create → destroy against the in-memory fake API.
func TestDNSSECKey_lifecycle(t *testing.T) {
	f := newFakeGandi(t)
	var firstID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if n := f.keyCount(testDNSSECDomain); n != 0 {
				return fmt.Errorf("%d DNSSEC keys left after destroy", n)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testDNSSECKeyConfig(f, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "type", "ksk"),
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "algorithm", "13"),
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "public_key", testDNSSECPublicKey),
					resource.TestMatchResourceAttr("gandi_dnssec_key.test", "id", regexp.MustCompile(`^example\.com/\d+$`)),
					resource.TestCheckResourceAttrSet("gandi_dnssec_key.test", "key_id"),
					resource.TestCheckResourceAttrSet("gandi_dnssec_key.test", "keytag"),
					resource.TestCheckResourceAttrSet("gandi_dnssec_key.test", "digest"),
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "digest_type", "2"),
					testDNSSECKeyCount(f, 1),
					func(s *terraform.State) error {
						firstID = s.RootModule().Resources["gandi_dnssec_key.test"].Primary.ID
						return nil
					},
				),
			},
			{
				ResourceName:      "gandi_dnssec_key.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // Key deleted in the dashboard: Read drops it and the plan re-creates it.
				PreConfig: func() {
					id, _ := strconv.Atoi(firstID[len(testDNSSECDomain)+1:])
					if !f.removeKey(testDNSSECDomain, id) {
						t.Fatalf("key %s not in fake", firstID)
					}
				},
				Config:             testDNSSECKeyConfig(f, ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testDNSSECKeyConfig(f, ""),
				Check: func(s *terraform.State) error {
					if id := s.RootModule().Resources["gandi_dnssec_key.test"].Primary.ID; id == firstID {
						return fmt.Errorf("id = %s, want a new key id after re-create", id)
					}
					return nil
				},
			},
		},
	})
}

// TestDNSSECKey_replaceOnChange checks that changing an argument replaces the
// key (the API has no update).
func TestDNSSECKey_replaceOnChange(t *testing.T) {
	f := newFakeGandi(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testDNSSECKeyConfig(f, `type = "zsk"`)},
			{
				Config: testDNSSECKeyConfig(f, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "type", "ksk"),
					testDNSSECKeyCount(f, 1),
				),
			},
		},
	})
}

// TestDNSSECKey_rollover models a KSK rollover with create_before_destroy: the
// new key is submitted while the old one still exists, then the old one goes.
func TestDNSSECKey_rollover(t *testing.T) {
	f := newFakeGandi(t)
	const newKey = "Rollover0rWV9Nc4Cr0JlWBPwLAk9sZ9s7O0YfYlFt2QvGmYSPC3sMu4Ak6TQi6pEHJhD1W9iHrbJmm4eG9o6A=="
	cbd := "lifecycle {\n    create_before_destroy = true\n  }"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testDNSSECKeyConfigPK(f, testDNSSECPublicKey, cbd)},
			{
				Config: testDNSSECKeyConfigPK(f, newKey, cbd),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "public_key", newKey),
					testDNSSECKeyCount(f, 1),
				),
			},
		},
	})
}

func testDNSSECKeyCount(f *fakeGandi, want int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if n := f.keyCount(testDNSSECDomain); n != want {
			return fmt.Errorf("fake has %d keys, want %d", n, want)
		}
		return nil
	}
}

// TestDNSSECKey_whitespaceInPublicKey checks that a key pasted in zone-file
// form (space-separated base64 chunks) does not cause a perpetual diff: the
// API returns it compact, and the framework fails the step if the post-apply
// plan is not empty.
func TestDNSSECKey_whitespaceInPublicKey(t *testing.T) {
	f := newFakeGandi(t)
	spaced := testDNSSECPublicKey[:40] + " " + testDNSSECPublicKey[40:]
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: f.providerConfig() + fmt.Sprintf(`
resource "gandi_dnssec_key" "test" {
  domain     = %q
  algorithm  = 13
  public_key = %q
}
`, testDNSSECDomain, spaced),
				Check: resource.TestCheckResourceAttr("gandi_dnssec_key.test", "public_key", spaced),
			},
		},
	})
}

func TestDNSSECKey_refusesToAdoptExistingKey(t *testing.T) {
	f := newFakeGandi(t)
	existing := f.addKey(testDNSSECDomain, 13, "ksk", testDNSSECPublicKey)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testDNSSECKeyConfig(f, ""),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`import <address>\s+example\.com/%d`, existing.ID)),
			},
		},
	})
}

func TestDNSSECKey_apiErrors(t *testing.T) {
	f := newFakeGandi(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig:   func() { f.failRequests(http.MethodPost) },
				Config:      testDNSSECKeyConfig(f, ""),
				ExpectError: regexp.MustCompile(`Unable to create DNSSEC key[\s\S]*injected failure`),
			},
			{
				PreConfig: func() { f.failRequests("") },
				Config:    testDNSSECKeyConfig(f, ""),
			},
			{
				PreConfig:   func() { f.failRequests(http.MethodDelete) },
				Config:      testDNSSECKeyConfig(f, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to delete DNSSEC key`),
			},
			{ // Let the test framework's final destroy succeed.
				PreConfig: func() { f.failRequests("") },
				Config:    testDNSSECKeyConfig(f, ""),
			},
			{
				PreConfig:   func() { f.failRequests(http.MethodGet) },
				Config:      testDNSSECKeyConfig(f, ""),
				ExpectError: regexp.MustCompile(`Unable to read DNSSEC key`),
			},
			{
				PreConfig: func() { f.failRequests("") },
				Config:    testDNSSECKeyConfig(f, ""),
			},
		},
	})
}

func TestDNSSECKey_validation(t *testing.T) {
	f := newFakeGandi(t)
	for name, tc := range map[string]struct{ extra, algo, want string }{
		"bad type":      {extra: `type = "csk"`, algo: "13", want: `Invalid key type`},
		"bad algorithm": {algo: "256", want: `Invalid algorithm`},
	} {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: f.providerConfig() + fmt.Sprintf(`
resource "gandi_dnssec_key" "test" {
  domain     = "example.com"
  algorithm  = %s
  public_key = "AbC="
  %s
}
`, tc.algo, tc.extra),
						ExpectError: regexp.MustCompile(tc.want),
					},
				},
			})
		})
	}
}

func TestDNSSECKey_invalidImportID(t *testing.T) {
	f := newFakeGandi(t)
	for _, id := range []string{"example.com", "example.com/abc", "/1"} {
		t.Run(id, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:        testDNSSECKeyConfig(f, ""),
						ResourceName:  "gandi_dnssec_key.test",
						ImportState:   true,
						ImportStateId: id,
						ExpectError:   regexp.MustCompile(`Invalid import ID`),
					},
				},
			})
		})
	}
}

// TestAccDNSSECKey_basic submits a real DNSKEY to the registry, which publishes
// a DS record and can break resolution of GANDI_TEST_DOMAIN if its zone is not
// signed with that key. It only runs when GANDI_TEST_DNSSEC_PUBLIC_KEY is set;
// use a throwaway (e.g. sandbox) domain. GANDI_TEST_DNSSEC_ALGORITHM defaults
// to 13.
func TestAccDNSSECKey_basic(t *testing.T) {
	pk := os.Getenv("GANDI_TEST_DNSSEC_PUBLIC_KEY")
	if pk == "" {
		t.Skip("set GANDI_TEST_DNSSEC_PUBLIC_KEY to run the destructive DNSSEC key test")
	}
	algo := os.Getenv("GANDI_TEST_DNSSEC_ALGORITHM")
	if algo == "" {
		algo = "13"
	}
	domain := testAccDomain()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "gandi" {}

resource "gandi_dnssec_key" "test" {
  domain     = %q
  algorithm  = %s
  public_key = %q
}
`, domain, algo, pk),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gandi_dnssec_key.test", "type", "ksk"),
					resource.TestCheckResourceAttrSet("gandi_dnssec_key.test", "key_id"),
					resource.TestCheckResourceAttrSet("gandi_dnssec_key.test", "keytag"),
				),
			},
			{
				ResourceName:      "gandi_dnssec_key.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
