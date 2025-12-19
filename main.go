package main

import (
	"flag"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = ""
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := &plugin.ServeOpts{
		Debug:        debugMode,
		ProviderAddr: "registry.terraform.io/providers/adaptive-scale/adaptive",
		ProviderFunc: provider.New(version),
	}

	plugin.Serve(opts)
}
