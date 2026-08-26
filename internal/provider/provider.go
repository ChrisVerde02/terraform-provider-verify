package provider

import (
	"context"
	"os"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"

	"github.com/Christian-Verderame/terraform-provider-verify/internal/datasources"
	"github.com/Christian-Verderame/terraform-provider-verify/internal/resources"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ProviderData is passed to every resource and data source via Configure().
// It carries pre-built SDK clients so resources never handle credentials directly.
type ProviderData struct {
	// STSClient is configured with the STS client credentials and is used for
	// token exchange and introspection operations.
	STSClient *verifyclient.Client

	// CertClient is configured with the cert-manager client credentials and is
	// used for signer certificate management operations.
	CertClient *verifyclient.Client

	// AppsClient is configured with the app client credentials and is used for
	// IBM Verify application management operations. Falls back to STSClient if
	// dedicated app credentials are not provided.
	AppsClient *verifyclient.Client
}

// GetSTSClient returns the STS SDK client. May be nil if not configured.
func (pd *ProviderData) GetSTSClient() *verifyclient.Client { return pd.STSClient }

// GetCertClient returns the cert-manager SDK client. May be nil if not configured.
func (pd *ProviderData) GetCertClient() *verifyclient.Client { return pd.CertClient }

// GetAppsClient returns the apps SDK client. May be nil if not configured.
func (pd *ProviderData) GetAppsClient() *verifyclient.Client { return pd.AppsClient }

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

// verifyProviderModel maps the provider schema to Go types.
type verifyProviderModel struct {
	TenantURL               types.String `tfsdk:"tenant_url"`
	STSClientID             types.String `tfsdk:"sts_client_id"`
	STSClientSecret         types.String `tfsdk:"sts_client_secret"`
	CertManagerClientID     types.String `tfsdk:"cert_manager_client_id"`
	CertManagerClientSecret types.String `tfsdk:"cert_manager_client_secret"`
	AppClientID             types.String `tfsdk:"app_client_id"`
	AppClientSecret         types.String `tfsdk:"app_client_secret"`
}

// Schema defines provider-level configuration attributes.
// All attributes are optional — values can also be supplied via environment
// variables (VERIFY_TENANT_URL, VERIFY_STS_CLIENT_ID, etc.).
func (p *VerifyProvider) Schema(
	ctx context.Context,
	req provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "The IBM Verify provider manages IBM Verify resources including " +
			"signer certificates, JWTs, and token exchange. Credentials can be " +
			"supplied in the provider block or via environment variables.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL, e.g. https://example.verify.ibm.com. " +
					"Can also be set with the VERIFY_TENANT_URL environment variable.",
				Optional: true,
			},
			"sts_client_id": schema.StringAttribute{
				Description: "Client ID of the STS API client used for token exchange and " +
					"introspection. Can also be set with VERIFY_STS_CLIENT_ID.",
				Optional: true,
			},
			"sts_client_secret": schema.StringAttribute{
				Description: "Client secret of the STS API client. " +
					"Can also be set with VERIFY_STS_CLIENT_SECRET.",
				Optional:  true,
				Sensitive: true,
			},
			"cert_manager_client_id": schema.StringAttribute{
				Description: "Client ID of the cert-manager API client used for signer " +
					"certificate management. Can also be set with VERIFY_CERT_MANAGER_CLIENT_ID.",
				Optional: true,
			},
			"cert_manager_client_secret": schema.StringAttribute{
				Description: "Client secret of the cert-manager API client. " +
					"Can also be set with VERIFY_CERT_MANAGER_CLIENT_SECRET.",
				Optional:  true,
				Sensitive: true,
			},
			"app_client_id": schema.StringAttribute{
				Description: "Client ID of the API client used for application management. " +
					"Falls back to sts_client_id if not set. " +
					"Can also be set with VERIFY_APP_CLIENT_ID.",
				Optional: true,
			},
			"app_client_secret": schema.StringAttribute{
				Description: "Client secret of the app management API client. " +
					"Falls back to sts_client_secret if not set. " +
					"Can also be set with VERIFY_APP_CLIENT_SECRET.",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

// Configure runs after Terraform reads the provider block.
// It builds two SDK clients (STS and cert-manager) and stores them in
// ProviderData so every resource and data source can use them directly.
func (p *VerifyProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var config verifyProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve each value: provider block → environment variable.
	tenantURL := resolveValue(config.TenantURL, "VERIFY_TENANT_URL")
	stsClientID := resolveValue(config.STSClientID, "VERIFY_STS_CLIENT_ID")
	stsClientSecret := resolveValue(config.STSClientSecret, "VERIFY_STS_CLIENT_SECRET")
	certClientID := resolveValue(config.CertManagerClientID, "VERIFY_CERT_MANAGER_CLIENT_ID")
	certClientSecret := resolveValue(config.CertManagerClientSecret, "VERIFY_CERT_MANAGER_CLIENT_SECRET")
	appClientID := resolveValue(config.AppClientID, "VERIFY_APP_CLIENT_ID")
	appClientSecret := resolveValue(config.AppClientSecret, "VERIFY_APP_CLIENT_SECRET")

	// tenantURL is required for any API operation.
	if tenantURL == "" {
		resp.Diagnostics.AddError(
			"Missing IBM Verify tenant URL",
			"Set tenant_url in the provider block or the VERIFY_TENANT_URL environment variable.",
		)
		return
	}

	pd := &ProviderData{}

	// Build the STS client if credentials are present.
	if stsClientID != "" && stsClientSecret != "" {
		c, err := verifyclient.New(tenantURL,
			verifyclient.WithClientCredentials(stsClientID, stsClientSecret),
		)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create STS client", err.Error())
			return
		}
		pd.STSClient = c
	}

	// Build the cert-manager client if credentials are present.
	if certClientID != "" && certClientSecret != "" {
		c, err := verifyclient.New(tenantURL,
			verifyclient.WithClientCredentials(certClientID, certClientSecret),
		)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create cert-manager client", err.Error())
			return
		}
		pd.CertClient = c
	}

	// Build the apps client. Prefer dedicated app credentials; fall back to
	// the STS client if app-specific credentials are not provided (the same
	// verifyclient.Client already has .Apps wired internally).
	if appClientID != "" && appClientSecret != "" {
		c, err := verifyclient.New(tenantURL,
			verifyclient.WithClientCredentials(appClientID, appClientSecret),
		)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create apps client", err.Error())
			return
		}
		pd.AppsClient = c
	} else if pd.STSClient != nil {
		// Reuse the STS client — it has .Apps wired from the same verifyclient.New call.
		pd.AppsClient = pd.STSClient
	}

	// Make ProviderData available to all resources and data sources.
	resp.ResourceData = pd
	resp.DataSourceData = pd
}

// resolveValue returns the string value from a Terraform attribute if set,
// otherwise falls back to the named environment variable.
func resolveValue(attr types.String, envVar string) string {
	if !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != "" {
		return attr.ValueString()
	}
	return os.Getenv(envVar)
}

// Resources returns the resources supported by this provider.
func (p *VerifyProvider) Resources(
	ctx context.Context,
) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewCertificateResource,
		resources.NewJWTResource,
		resources.NewTokenExchangeResource,
		resources.NewSignerCertResource,
		resources.NewApplicationResource,
	}
}

// DataSources returns the data sources supported by this provider.
func (p *VerifyProvider) DataSources(
	ctx context.Context,
) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewJWTDataSource,
		datasources.NewTokenExchangeDataSource,
		datasources.NewTokenIntrospectionDataSource,
		datasources.NewClientCredentialsTokenDataSource,
	}
}
