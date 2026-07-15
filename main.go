package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/Christian-Verderame/terraform-provider-verify/internal/provider"
)

func main() {
	err := providerserver.Serve(
		context.Background(),
		provider.New,
		providerserver.ServeOpts{
			Address: "registry.terraform.io/christian-verderame/verify",
		},
	)

	if err != nil {
		log.Fatal(err)
	}
}
