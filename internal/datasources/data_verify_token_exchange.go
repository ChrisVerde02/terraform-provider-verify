package datasources

import (
	"context"

	verifyclient "github.com/Christian-Verderame/terraform-provider-verify/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TokenExchangeDataSource{}

// TokenExchangeDataSource implements the verify_token_exchange data source.
//
// Unlike the verify_token_exchange resource, this data source calls IBM Verify
// on every plan and apply. That means the access token is always fresh and
// never served from a stale Terraform state file.
type TokenExchangeDataSource struct{}

// TokenExchangeDataSourceModel represents configuration and computed outputs.
type TokenExchangeDataSourceModel struct {
	TenantURL        types.String `tfsdk:"tenant_url"`
	ClientID         types.String `tfsdk:"client_id"`
	ClientSecret     types.String `tfsdk:"client_secret"`
	SubjectToken     types.String `tfsdk:"subject_token"`
	SubjectTokenType types.String `tfsdk:"subject_token_type"`

	// Computed outputs populated from the IBM Verify response.
	AccessToken     types.String `tfsdk:"access_token"`
	ExpiresIn       types.Int64  `tfsdk:"expires_in"`
	GrantID         types.String `tfsdk:"grant_id"`
	IssuedTokenType types.String `tfsdk:"issued_token_type"`
	Scope           types.String `tfsdk:"scope"`
	TokenType       types.String `tfsdk:"token_type"`
}

// NewTokenExchangeDataSource creates the data source.
func NewTokenExchangeDataSource() datasource.DataSource {
	return &TokenExchangeDataSource{}
}

// Metadata defines the Terraform data source name.
func (d *TokenExchangeDataSource) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_token_exchange"
}

// Schema defines the data source inputs and outputs.
func (d *TokenExchangeDataSource) Schema(
	ctx context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Exchanges a custom JWT for an IBM Verify access token. " +
			"Re-evaluated on every plan and apply so the token is always fresh.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant URL, such as https://example.verify.ibm.com.",
				Required:    true,
			},

			"client_id": schema.StringAttribute{
				Description: "Client ID belonging to the configured STS client.",
				Required:    true,
			},

			"client_secret": schema.StringAttribute{
				Description: "Client secret belonging to the configured STS client.",
				Required:    true,
				Sensitive:   true,
			},

			"subject_token": schema.StringAttribute{
				Description: "Signed custom JWT to exchange.",
				Required:    true,
				Sensitive:   true,
			},

			// subject_token_type must match the URN configured in the IBM Verify
			// STS client's Token Exchange settings. Use the standard RFC 8693 value
			// urn:ietf:params:oauth:token-type:jwt unless your tenant is configured
			// with a custom URN (e.g. urn:demo:token-type:user-jwt).
			"subject_token_type": schema.StringAttribute{
				Description: "RFC 8693 token-type URN for the subject token. " +
					"Use urn:ietf:params:oauth:token-type:jwt for a standard JWT, " +
					"or the custom URN configured in your IBM Verify STS client.",
				Required: true,
			},

			"access_token": schema.StringAttribute{
				Description: "Access token returned by IBM Verify.",
				Computed:    true,
				Sensitive:   true,
			},

			"expires_in": schema.Int64Attribute{
				Description: "Access-token lifetime in seconds.",
				Computed:    true,
			},

			"grant_id": schema.StringAttribute{
				Description: "Grant ID returned by IBM Verify.",
				Computed:    true,
			},

			"issued_token_type": schema.StringAttribute{
				Description: "Type of token issued by IBM Verify.",
				Computed:    true,
			},

			"scope": schema.StringAttribute{
				Description: "Scopes associated with the access token.",
				Computed:    true,
			},

			"token_type": schema.StringAttribute{
				Description: "Access-token authorization type, normally bearer.",
				Computed:    true,
			},
		},
	}
}

// Read calls IBM Verify and returns a fresh access token.
// This is called on every plan and apply so the token is always current.
func (d *TokenExchangeDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config TokenExchangeDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := verifyclient.ExchangeToken(
		ctx,
		verifyclient.TokenExchangeRequest{
			TenantURL:        config.TenantURL.ValueString(),
			ClientID:         config.ClientID.ValueString(),
			ClientSecret:     config.ClientSecret.ValueString(),
			SubjectToken:     config.SubjectToken.ValueString(),
			SubjectTokenType: config.SubjectTokenType.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to exchange JWT for access token",
			err.Error(),
		)
		return
	}

	config.AccessToken = types.StringValue(result.AccessToken)
	config.ExpiresIn = types.Int64Value(result.ExpiresIn)
	config.GrantID = types.StringValue(result.GrantID)
	config.IssuedTokenType = types.StringValue(result.IssuedTokenType)
	config.Scope = types.StringValue(result.Scope)
	config.TokenType = types.StringValue(result.TokenType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
