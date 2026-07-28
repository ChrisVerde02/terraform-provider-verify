package resources

import (
	"context"
	"time"

	providercrypto "github.com/ChrisVerde02/ibmverify-go/crypto"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &JWTResource{}

// JWTResource implements the verify_jwt resource.
type JWTResource struct{}

// JWTResourceModel represents the Terraform configuration and state.
type JWTResourceModel struct {
	Issuer        types.String `tfsdk:"issuer"`
	Subject       types.String `tfsdk:"subject"`
	KeyID         types.String `tfsdk:"key_id"`
	JWTID         types.String `tfsdk:"jwt_id"`
	PrivateKeyPEM types.String `tfsdk:"private_key_pem"`
	ExpiresIn     types.Int64  `tfsdk:"expires_in_seconds"`

	Token     types.String `tfsdk:"token"`
	IssuedAt  types.Int64  `tfsdk:"issued_at"`
	ExpiresAt types.Int64  `tfsdk:"expires_at"`
}

// NewJWTResource creates a new JWT resource.
func NewJWTResource() resource.Resource {
	return &JWTResource{}
}

// Metadata defines the Terraform resource name.
func (r *JWTResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_jwt"
}

// Schema defines the JWT resource inputs and outputs.
func (r *JWTResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Generates an RS256-signed JWT using an RSA private key.",

		Attributes: map[string]schema.Attribute{
			"issuer": schema.StringAttribute{
				Description: "JWT issuer claim, such as https://demo.ibm.com.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"subject": schema.StringAttribute{
				Description: "Cloud Directory username placed in the sub claim.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"key_id": schema.StringAttribute{
				Description: "JWT kid header matching the IBM Verify certificate label.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"jwt_id": schema.StringAttribute{
				Description: "Unique value placed in the jti claim.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"private_key_pem": schema.StringAttribute{
				Description: "RSA private key used to sign the JWT.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"expires_in_seconds": schema.Int64Attribute{
				Description: "JWT lifetime in seconds.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
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

// Create generates and signs the JWT.
func (r *JWTResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan JWTResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := providercrypto.GenerateSignedJWT(
		providercrypto.JWTRequest{
			Issuer:        plan.Issuer.ValueString(),
			Subject:       plan.Subject.ValueString(),
			KeyID:         plan.KeyID.ValueString(),
			JWTID:         plan.JWTID.ValueString(),
			PrivateKeyPEM: plan.PrivateKeyPEM.ValueString(),
			ExpiresIn: time.Duration(
				plan.ExpiresIn.ValueInt64(),
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

	plan.Token = types.StringValue(result.Token)
	plan.IssuedAt = types.Int64Value(result.IssuedAt)
	plan.ExpiresAt = types.Int64Value(result.ExpiresAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read retains the locally generated JWT in Terraform state.
func (r *JWTResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
}

// Update is unused because all JWT inputs require replacement.
func (r *JWTResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the JWT resource from Terraform state.
func (r *JWTResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
}
