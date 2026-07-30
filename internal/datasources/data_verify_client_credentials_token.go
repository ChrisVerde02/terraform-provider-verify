package datasources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ClientCredentialsTokenDataSource{}

// ClientCredentialsTokenDataSource implements verify_client_credentials_token.
// It calls POST /oauth2/token with grant_type=client_credentials and returns
// the access token. Re-evaluated on every plan and apply so the token is
// always fresh and never read from stale Terraform state.
type ClientCredentialsTokenDataSource struct{}

// ClientCredentialsTokenModel is the Terraform schema model.
type ClientCredentialsTokenModel struct {
	TenantURL    types.String `tfsdk:"tenant_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`

	AccessToken types.String `tfsdk:"access_token"`
	ExpiresIn   types.Int64  `tfsdk:"expires_in"`
	TokenType   types.String `tfsdk:"token_type"`
	Scope       types.String `tfsdk:"scope"`
}

// clientCredentialsResponse is the JSON body returned by IBM Verify.
type clientCredentialsResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// NewClientCredentialsTokenDataSource creates the data source.
func NewClientCredentialsTokenDataSource() datasource.DataSource {
	return &ClientCredentialsTokenDataSource{}
}

// Metadata defines the Terraform data source name.
func (d *ClientCredentialsTokenDataSource) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_client_credentials_token"
}

// Schema defines the data source inputs and outputs.
func (d *ClientCredentialsTokenDataSource) Schema(
	ctx context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Obtains an IBM Verify access token using the OAuth 2.0 " +
			"client credentials grant (grant_type=client_credentials). " +
			"The token carries the API client's own entitlements directly — " +
			"no user impersonation. Re-evaluated on every plan and apply.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant URL, such as https://example.verify.ibm.com.",
				Required:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "IBM Verify API client ID.",
				Required:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "IBM Verify API client secret.",
				Required:    true,
				Sensitive:   true,
			},
			"access_token": schema.StringAttribute{
				Description: "Access token returned by IBM Verify.",
				Computed:    true,
				Sensitive:   true,
			},
			"expires_in": schema.Int64Attribute{
				Description: "Access token lifetime in seconds.",
				Computed:    true,
			},
			"token_type": schema.StringAttribute{
				Description: "Access token type, normally bearer.",
				Computed:    true,
			},
			"scope": schema.StringAttribute{
				Description: "Scopes granted with the access token.",
				Computed:    true,
			},
		},
	}
}

// Read calls IBM Verify and returns a fresh client credentials access token.
func (d *ClientCredentialsTokenDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config ClientCredentialsTokenModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tenantURL := strings.TrimRight(config.TenantURL.ValueString(), "/")
	endpoint := tenantURL + "/v1.0/endpoint/default/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", config.ClientID.ValueString())
	form.Set("client_secret", config.ClientSecret.ValueString())

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewBufferString(form.Encode()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create client credentials request", err.Error())
		return
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to call IBM Verify token endpoint", err.Error())
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read IBM Verify response", err.Error())
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"IBM Verify client credentials token failed",
			fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, string(body)),
		)
		return
	}

	var result clientCredentialsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		resp.Diagnostics.AddError("Unable to parse IBM Verify response", err.Error())
		return
	}

	config.AccessToken = types.StringValue(result.AccessToken)
	config.ExpiresIn = types.Int64Value(result.ExpiresIn)
	config.TokenType = types.StringValue(result.TokenType)
	config.Scope = types.StringValue(result.Scope)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
