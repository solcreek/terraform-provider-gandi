package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/solcreek/terraform-provider-gandi/internal/provider"
)

// Generate the registry documentation from schema + examples/ + templates/.
//go:generate go tool tfplugindocs generate --provider-name gandi

// version and commit are overridden at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
)

var _ = commit // surfaced via -ldflags; referenced to avoid "unused" in dev builds

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to run the provider with support for debuggers")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/solcreek/gandi",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
