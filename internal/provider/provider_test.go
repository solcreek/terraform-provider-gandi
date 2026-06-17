package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories wires the in-process provider for acceptance
// tests, matching the source address declared in the test configs.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"gandi": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies the environment is configured for acceptance tests.
// Acceptance tests make real API calls and require:
//   - TF_ACC=1
//   - GANDI_PAT          a valid Personal Access Token
//   - GANDI_TEST_DOMAIN  a domain in the account that tests may mutate
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("GANDI_PAT") == "" {
		t.Fatal("GANDI_PAT must be set for acceptance tests")
	}
	if os.Getenv("GANDI_TEST_DOMAIN") == "" {
		t.Fatal("GANDI_TEST_DOMAIN must be set for acceptance tests")
	}
}

func testAccDomain() string { return os.Getenv("GANDI_TEST_DOMAIN") }
