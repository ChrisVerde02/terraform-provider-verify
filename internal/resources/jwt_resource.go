package resources

import (
	"context"
	"fmt"
	"time"

	providercrypto "github.com/ChrisVerde02/ibmverify-go/crypto"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &JWTResource{}
var _ resource.ResourceWithImportState = &JWTResource{}

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
		Description: "Generates an RS256-signed JWT using an RSA private key. " +
			"The JWT is stored in state and reused across plans. " +
			"Refresh policy: on each plan/apply the resource checks the token expiry. " +
			"If fewer than 60 seconds remain, a new JWT is signed with a fresh jti claim " +
			"automatically — preventing IBM Verify replay rejection (CSIAQ5206E). " +
			"The threshold is not configurable; use data.verify_jwt for a fresh JWT " +
			"on every apply.",

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
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"jwt_id": schema.StringAttribute{
				Description: "Unique value placed in the jti claim. " +
					"Automatically refreshed when the JWT expires.",
				Computed: true,
				Optional: true,
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
				Description: "JWT lifetime in seconds. Must be at least 1.",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
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
				Description: "JWT exp value as a Unix timestamp. " +
					"Refresh policy: the resource regenerates the JWT with a fresh jti " +
					"automatically when fewer than 60 seconds remain before this timestamp. " +
					"The threshold is not configurable.",
				Computed: true,
			},
		},
	}
}

// signJWT is a shared helper used by both Create and Read.
func signJWT(state *JWTResourceModel) error {
	jwtID, err := providercrypto.GenerateJTI()
	if err != nil {
		return fmt.Errorf("generate JWT ID: %w", err)
	}

	result, err := providercrypto.GenerateSignedJWT(
		providercrypto.JWTRequest{
			Issuer:        state.Issuer.ValueString(),
			Subject:       state.Subject.ValueString(),
			KeyID:         state.KeyID.ValueString(),
			JWTID:         jwtID,
			PrivateKeyPEM: state.PrivateKeyPEM.ValueString(),
			ExpiresIn: time.Duration(
				state.ExpiresIn.ValueInt64(),
			) * time.Second,
		},
	)
	if err != nil {
		return err
	}

	state.JWTID = types.StringValue(jwtID)
	state.Token = types.StringValue(result.Token)
	state.IssuedAt = types.Int64Value(result.IssuedAt)
	state.ExpiresAt = types.Int64Value(result.ExpiresAt)
	return nil
}

// ImportState is intentionally unsupported for verify_jwt.
// JWTs are signed locally from a private key — there is no remote resource
// to import. Use the data.verify_jwt data source for a fresh JWT on every
// plan/apply without any state management.
func (r *JWTResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.AddError(
		"Import not supported for verify_jwt",
		"verify_jwt signs a JWT locally — there is no remote resource to import. "+
			"Use the data.verify_jwt data source instead for stateless JWT generation.",
	)
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

	if err := signJWT(&plan); err != nil {
		resp.Diagnostics.AddError("Unable to generate JWT", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read checks whether the JWT has expired.
// If it is still valid (more than 60 seconds from expiry) the existing state
// is kept unchanged — no new JWT is generated and no API call is made.
// If it has expired (or is within 60 seconds of expiry) a fresh JWT is signed
// with a new jti claim, preventing IBM Verify replay rejection (CSIAQ5206E).
func (r *JWTResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state JWTResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 60-second buffer — regenerate before IBM Verify rejects the token.
	const bufferSeconds = 60

	if time.Now().Unix() < state.ExpiresAt.ValueInt64()-bufferSeconds {
		// JWT is still valid — keep existing state as-is.
		return
	}

	// JWT has expired or is about to — regenerate with a fresh jti.
	if err := signJWT(&state); err != nil {
		resp.Diagnostics.AddError("Unable to regenerate expired JWT", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
