package datasources

import (
	"context"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TokenIntrospectionDataSource{}

// TokenIntrospectionDataSource implements the verify_token_introspection
// data source. It is a data source (not a resource) because introspection
// is a read-only point-in-time query — the result changes as soon as the
// token expires, so Terraform must re-evaluate it on every plan/apply.
// A resource only calls Create once and caches the result in state forever,
// which would show active=true long after the token has expired.
type TokenIntrospectionDataSource struct{}

// TokenIntrospectionDataSourceModel represents configuration and result.
type TokenIntrospectionDataSourceModel struct {
	TenantURL    types.String `tfsdk:"tenant_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Token        types.String `tfsdk:"token"`

	// Computed fields populated from the IBM Verify introspection response.
	Active            types.Bool   `tfsdk:"active"`
	Username          types.String `tfsdk:"username"`
	PreferredUsername types.String `tfsdk:"preferred_username"`
	Name              types.String `tfsdk:"name"`
	GivenName         types.String `tfsdk:"given_name"`
	Subject           types.String `tfsdk:"subject"`
	Scope             types.String `tfsdk:"scope"`
	TokenType         types.String `tfsdk:"token_type"`
	Issuer            types.String `tfsdk:"issuer"`
	IssuedAt          types.Int64  `tfsdk:"issued_at"`
	ExpiresAt         types.Int64  `tfsdk:"expires_at"`
}

// NewTokenIntrospectionDataSource creates the data source.
func NewTokenIntrospectionDataSource() datasource.DataSource {
	return &TokenIntrospectionDataSource{}
}

// Metadata defines the Terraform data source name.
func (d *TokenIntrospectionDataSource) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_token_introspection"
}

// Schema defines the data source inputs and outputs.
func (d *TokenIntrospectionDataSource) Schema(
	ctx context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Introspects an IBM Verify access token. " +
			"Re-evaluated on every plan and apply.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL.",
				Required:    true,
			},

			"client_id": schema.StringAttribute{
				Description: "STS client ID.",
				Required:    true,
			},

			"client_secret": schema.StringAttribute{
				Description: "STS client secret.",
				Required:    true,
				Sensitive:   true,
			},

			"token": schema.StringAttribute{
				Description: "Access token to introspect.",
				Required:    true,
				Sensitive:   true,
			},

			"active": schema.BoolAttribute{
				Description: "Whether IBM Verify considers the token active.",
				Computed:    true,
			},

			// username is the Cloud Directory username (e.g. "Bretton").
			// Only populated if the STS client is configured to include it.
			"username": schema.StringAttribute{
				Description: "Cloud Directory username (e.g. Bretton).",
				Computed:    true,
			},

			// preferred_username is the OIDC standard claim for the login name.
			// IBM Verify may return this instead of or in addition to username.
			"preferred_username": schema.StringAttribute{
				Description: "OIDC preferred_username claim.",
				Computed:    true,
			},

			// name is the user's full display name (e.g. "Jessica").
			"name": schema.StringAttribute{
				Description: "Full display name of the user.",
				Computed:    true,
			},

			// given_name is the user's first name (e.g. "Jessica").
			"given_name": schema.StringAttribute{
				Description: "Given (first) name of the user.",
				Computed:    true,
			},

			"subject": schema.StringAttribute{
				Description: "Token subject.",
				Computed:    true,
			},

			"scope": schema.StringAttribute{
				Description: "Scopes associated with the token.",
				Computed:    true,
			},

			"token_type": schema.StringAttribute{
				Description: "Token type returned by introspection.",
				Computed:    true,
			},

			"issuer": schema.StringAttribute{
				Description: "Issuer associated with the token.",
				Computed:    true,
			},

			"issued_at": schema.Int64Attribute{
				Description: "Token issue time as a Unix timestamp.",
				Computed:    true,
			},

			"expires_at": schema.Int64Attribute{
				Description: "Token expiration as a Unix timestamp.",
				Computed:    true,
			},
		},
	}
}

// Read calls IBM Verify and populates the introspection result.
// This is called on every plan and apply, so the result is always current.
func (d *TokenIntrospectionDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config TokenIntrospectionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := verifyclient.New(config.TenantURL.ValueString(),
		verifyclient.WithClientCredentials(config.ClientID.ValueString(), config.ClientSecret.ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build IBM Verify client", err.Error())
		return
	}
	result, err := c.Token.Introspect(ctx, config.Token.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to introspect access token",
			err.Error(),
		)
		return
	}

	config.Active = types.BoolValue(result.Active)
	config.Username = types.StringValue(result.Username)
	config.PreferredUsername = types.StringValue(result.PreferredUsername)
	config.Name = types.StringValue(result.Name)
	config.GivenName = types.StringValue(result.GivenName)
	config.Subject = types.StringValue(result.Subject)
	config.Scope = types.StringValue(result.Scope)
	config.TokenType = types.StringValue(result.TokenType)
	config.Issuer = types.StringValue(result.Issuer)
	config.IssuedAt = types.Int64Value(result.IssuedAt)
	config.ExpiresAt = types.Int64Value(result.ExpiresAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
