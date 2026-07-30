package provider

import (
	"context"

	"github.com/Christian-Verderame/terraform-provider-verify/internal/datasources"
	"github.com/Christian-Verderame/terraform-provider-verify/internal/resources"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// VerifyProvider defines our Terraform provider.
type VerifyProvider struct{}

// New creates and returns a new Verify provider.
func New() provider.Provider {
	return &VerifyProvider{}
}

// Metadata tells Terraform the provider type name.
func (p *VerifyProvider) Metadata(
	ctx context.Context,
	req provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = "verify"
}

// Schema defines configuration fields for the provider.
//
// It is empty for now. Later, we will add settings such as:
// - tenant_url
// - client_id
// - client_secret
func (p *VerifyProvider) Schema(
	ctx context.Context,
	req provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

// Configure runs after Terraform reads the provider configuration.
//
// Later, this method will create the IBM Verify HTTP client and make it
// available to all resources and data sources.
func (p *VerifyProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
}

// Resources returns the resources supported by this provider.
func (p *VerifyProvider) Resources(
	ctx context.Context,
) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewCertificateResource,
		resources.NewJWTResource,
		resources.NewTokenExchangeResource,
		// SignerCert uploads a PEM certificate to IBM Verify so it can
		// validate JWT signatures during token exchange.
		resources.NewSignerCertResource,
		// TokenIntrospection is intentionally a data source, not a resource.
		// See internal/datasources/token_introspection_data_source.go.
	}
}

// DataSources returns the data sources supported by this provider.
func (p *VerifyProvider) DataSources(
	ctx context.Context,
) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// JWT — generates a fresh signed JWT on every plan/apply.
		// Preferred over the verify_jwt resource for idempotent workflows.
		datasources.NewJWTDataSource,
		// TokenExchange — calls IBM Verify on every plan/apply.
		// Preferred over the verify_token_exchange resource so the access
		// token is never served from stale Terraform state.
		datasources.NewTokenExchangeDataSource,
		// Introspection is a data source so Terraform re-evaluates it on
		// every plan/apply, always reflecting the live token status.
		datasources.NewTokenIntrospectionDataSource,
	}
}
