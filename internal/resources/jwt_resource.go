package resources

import (
	"context"
	"crypto/rand"
	"fmt"
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
		Description: "Generates an RS256-signed JWT using an RSA private key. " +
			"The JWT is reused across plans until it expires, then automatically " +
			"regenerated with a fresh jti claim.",

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
				Description: "JWT exp value as a Unix timestamp. " +
					"The resource regenerates the JWT automatically when " +
					"this timestamp is within 60 seconds of the current time.",
				Computed: true,
			},
		},
	}
}

// generateJWTID produces a random UUID for use as a jti claim.
func generateJWTID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%12x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:],
	)
}

// signJWT is a shared helper used by both Create and Read.
func signJWT(state *JWTResourceModel) error {
	jwtID := generateJWTID()

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
