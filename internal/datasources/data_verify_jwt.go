package datasources

import (
	"context"
	"time"

	providercrypto "github.com/Christian-Verderame/terraform-provider-verify/internal/crypto"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &JWTDataSource{}

// JWTDataSource implements the verify_jwt data source.
//
// Unlike the verify_jwt resource, this data source re-generates the JWT on
// every plan and apply. That means the token is always fresh and the jti
// claim never replays between runs, eliminating the need for random_uuid.
type JWTDataSource struct{}

// JWTDataSourceModel represents configuration and computed outputs.
type JWTDataSourceModel struct {
	Issuer        types.String `tfsdk:"issuer"`
	Subject       types.String `tfsdk:"subject"`
	KeyID         types.String `tfsdk:"key_id"`
	PrivateKeyPEM types.String `tfsdk:"private_key_pem"`
	ExpiresIn     types.Int64  `tfsdk:"expires_in_seconds"`

	// Computed outputs.
	Token     types.String `tfsdk:"token"`
	IssuedAt  types.Int64  `tfsdk:"issued_at"`
	ExpiresAt types.Int64  `tfsdk:"expires_at"`
}

// NewJWTDataSource creates the data source.
func NewJWTDataSource() datasource.DataSource {
	return &JWTDataSource{}
}

// Metadata defines the Terraform data source name.
func (d *JWTDataSource) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_jwt"
}

// Schema defines the data source inputs and outputs.
func (d *JWTDataSource) Schema(
	ctx context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Generates a fresh RS256-signed JWT on every plan and apply. " +
			"Because it is a data source, the jti claim is always unique and the " +
			"token is never stale — no random_uuid keeper is required.",

		Attributes: map[string]schema.Attribute{
			"issuer": schema.StringAttribute{
				Description: "JWT issuer claim, such as https://demo.ibm.com.",
				Required:    true,
			},

			"subject": schema.StringAttribute{
				Description: "Cloud Directory username placed in the sub claim.",
				Required:    true,
			},

			"key_id": schema.StringAttribute{
				Description: "JWT kid header matching the IBM Verify certificate label.",
				Required:    true,
			},

			"private_key_pem": schema.StringAttribute{
				Description: "RSA private key used to sign the JWT.",
				Required:    true,
				Sensitive:   true,
			},

			"expires_in_seconds": schema.Int64Attribute{
				Description: "JWT lifetime in seconds.",
				Required:    true,
			},

			"token": schema.StringAttribute{
				Description: "Generated signed JWT.",
				Computed:    true,
				Sensitive:   true,
			},

			"issued_at": schema.Int64Attribute{
				Description: "JWT iat value as a Unix timestamp.",
				Computed:    true,
			},

			"expires_at": schema.Int64Attribute{
				Description: "JWT exp value as a Unix timestamp.",
				Computed:    true,
			},
		},
	}
}

// Read generates and signs a fresh JWT.
// This is called on every plan and apply so the token is always current.
func (d *JWTDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config JWTDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := providercrypto.GenerateSignedJWT(
		providercrypto.JWTRequest{
			Issuer:        config.Issuer.ValueString(),
			Subject:       config.Subject.ValueString(),
			KeyID:         config.KeyID.ValueString(),
			// Data sources generate a fresh jti on every read — no external
			// random_uuid resource is needed.
			JWTID:         generateJTI(),
			PrivateKeyPEM: config.PrivateKeyPEM.ValueString(),
			ExpiresIn: time.Duration(
				config.ExpiresIn.ValueInt64(),
			) * time.Second,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to generate JWT",
			err.Error(),
		)
		return
	}

	config.Token = types.StringValue(result.Token)
	config.IssuedAt = types.Int64Value(result.IssuedAt)
	config.ExpiresAt = types.Int64Value(result.ExpiresAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
