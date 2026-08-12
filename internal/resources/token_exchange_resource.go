package resources

import (
	"context"
	"time"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TokenExchangeResource{}
var _ resource.ResourceWithImportState = &TokenExchangeResource{}

// TokenExchangeResource implements verify_token_exchange.
type TokenExchangeResource struct{}

// TokenExchangeResourceModel represents the Terraform configuration and state.
type TokenExchangeResourceModel struct {
	TenantURL    types.String `tfsdk:"tenant_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	SubjectToken types.String `tfsdk:"subject_token"`
	// SubjectTokenType identifies the type of token being exchanged.
	// The RFC 8693 standard URN for a JWT is:
	//   urn:ietf:params:oauth:token-type:jwt
	// IBM Verify requires this exact value; non-standard URNs are rejected.
	SubjectTokenType types.String `tfsdk:"subject_token_type"`

	AccessToken     types.String `tfsdk:"access_token"`
	ExpiresIn       types.Int64  `tfsdk:"expires_in"`
	// ExpiresAt is the absolute Unix timestamp when the access token expires.
	// Computed from time.Now() + ExpiresIn at Create/Read time.
	// Read() uses this to decide whether to re-exchange or reuse.
	ExpiresAt       types.Int64  `tfsdk:"expires_at"`
	GrantID         types.String `tfsdk:"grant_id"`
	IssuedTokenType types.String `tfsdk:"issued_token_type"`
	Scope           types.String `tfsdk:"scope"`
	TokenType       types.String `tfsdk:"token_type"`
}

// NewTokenExchangeResource creates the Terraform resource.
func NewTokenExchangeResource() resource.Resource {
	return &TokenExchangeResource{}
}

// Metadata sets the Terraform resource name.
func (r *TokenExchangeResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_token_exchange"
}

// Schema defines the resource inputs and outputs.
func (r *TokenExchangeResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Exchanges a custom JWT for an IBM Verify access token. " +
			"The access token is reused across plans until it expires, then " +
			"automatically re-exchanged.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant URL, such as https://example.verify.ibm.com.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						urlRegexp,
						"must be a valid HTTPS URL, e.g. https://example.verify.ibm.com",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"client_id": schema.StringAttribute{
				Description: "Client ID belonging to the configured STS client.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"client_secret": schema.StringAttribute{
				Description: "Client secret belonging to the configured STS client.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"subject_token": schema.StringAttribute{
				Description: "Signed custom JWT to exchange.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// subject_token_type tells IBM Verify what kind of token is being
			// exchanged. RFC 8693 defines the correct URN for a JWT as
			// urn:ietf:params:oauth:token-type:jwt. Using a non-standard value
			// (e.g. urn:demo:token-type:user-jwt) causes IBM Verify to return
			// an unsupported_token_type error and reject the exchange.
			"subject_token_type": schema.StringAttribute{
				Description: "RFC 8693 token-type URN for the subject token. " +
					"Use urn:ietf:params:oauth:token-type:jwt for a signed JWT.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

			"expires_at": schema.Int64Attribute{
				Description: "Access-token expiry as a Unix timestamp. " +
					"The resource re-exchanges automatically when this timestamp " +
					"is within 60 seconds of the current time.",
				Computed: true,
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

// exchange calls IBM Verify and populates the token fields on state.
// It is shared between Create and Read.
func exchange(
	ctx context.Context,
	state *TokenExchangeResourceModel,
) error {
	subjectTokenType := state.SubjectTokenType.ValueString()
	if subjectTokenType == "" {
		subjectTokenType = "urn:demo:token-type:user-jwt"
	}

	state.SubjectTokenType = types.StringValue(subjectTokenType)

	result, err := verifyclient.ExchangeToken(
		ctx,
		verifyclient.TokenExchangeRequest{
			TenantURL:        state.TenantURL.ValueString(),
			ClientID:         state.ClientID.ValueString(),
			ClientSecret:     state.ClientSecret.ValueString(),
			SubjectToken:     state.SubjectToken.ValueString(),
			SubjectTokenType: subjectTokenType,
		},
	)
	if err != nil {
		return err
	}

	state.AccessToken = types.StringValue(result.AccessToken)
	state.ExpiresIn = types.Int64Value(result.ExpiresIn)
	// Compute absolute expiry from current time + lifetime returned by IBM Verify.
	state.ExpiresAt = types.Int64Value(time.Now().Unix() + result.ExpiresIn)
	state.GrantID = types.StringValue(result.GrantID)
	state.IssuedTokenType = types.StringValue(result.IssuedTokenType)
	state.Scope = types.StringValue(result.Scope)
	state.TokenType = types.StringValue(result.TokenType)
	return nil
}

// ImportState is intentionally unsupported for verify_token_exchange.
// Token exchange results are ephemeral — the access token expires and is
// re-exchanged automatically. There is no remote resource to import.
// Use the data.verify_token_exchange data source for a fresh token on every
// plan/apply without any state management.
func (r *TokenExchangeResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.AddError(
		"Import not supported for verify_token_exchange",
		"verify_token_exchange produces an ephemeral access token — there is no "+
			"remote resource to import. Use the data.verify_token_exchange data source "+
			"instead for a fresh token on every plan/apply.",
	)
}

// Create exchanges the JWT for an IBM Verify access token.
func (r *TokenExchangeResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan TokenExchangeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := exchange(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Unable to exchange JWT", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read checks whether the stored access token has expired.
// If it is still valid (more than 60 seconds from expiry) the existing state
// is kept unchanged — no API call is made to IBM Verify.
// If it has expired (or is within 60 seconds of expiry) a fresh token is
// obtained by re-exchanging with IBM Verify.
func (r *TokenExchangeResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state TokenExchangeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 60-second buffer — re-exchange before IBM Verify rejects the token.
	const bufferSeconds = 60

	if time.Now().Unix() < state.ExpiresAt.ValueInt64()-bufferSeconds {
		// Token is still valid — keep existing state as-is.
		return
	}

	// Token has expired or is about to — re-exchange for a fresh one.
	// If re-exchange fails (e.g. the subject JWT has also expired), remove
	// this resource from state so Terraform recreates it cleanly on the next
	// apply rather than surfacing a confusing mid-plan error.
	if err := exchange(ctx, &state); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unused because all inputs require replacement.
func (r *TokenExchangeResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the token exchange result from Terraform state.
func (r *TokenExchangeResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
}
