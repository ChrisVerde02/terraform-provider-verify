package resources

import (
	"context"

	verifyclient "github.com/Christian-Verderame/terraform-provider-verify/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TokenExchangeResource{}

// TokenExchangeResource implements verify_token_exchange.
type TokenExchangeResource struct{}

// TokenExchangeResourceModel represents the Terraform configuration and state.
type TokenExchangeResourceModel struct {
	TenantURL        types.String `tfsdk:"tenant_url"`
	ClientID         types.String `tfsdk:"client_id"`
	ClientSecret     types.String `tfsdk:"client_secret"`
	SubjectToken     types.String `tfsdk:"subject_token"`
	// SubjectTokenType identifies the type of token being exchanged.
	// The RFC 8693 standard URN for a JWT is:
	//   urn:ietf:params:oauth:token-type:jwt
	// IBM Verify requires this exact value; non-standard URNs are rejected.
	SubjectTokenType types.String `tfsdk:"subject_token_type"`

	AccessToken     types.String `tfsdk:"access_token"`
	ExpiresIn       types.Int64  `tfsdk:"expires_in"`
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
		Description: "Exchanges a custom JWT for an IBM Verify access token.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant URL, such as https://example.verify.ibm.com.",
				Required:    true,
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

	// Use the caller-supplied subject_token_type if provided. IBM Verify STS
	// clients can be configured with either the standard RFC 8693 URN
	// (urn:ietf:params:oauth:token-type:jwt) or a custom URN defined in the
	// tenant's Token exchange settings. Always set subject_token_type
	// explicitly in main.tf to match your tenant's STS configuration.
	subjectTokenType := plan.SubjectTokenType.ValueString()
	if subjectTokenType == "" {
		subjectTokenType = "urn:demo:token-type:user-jwt"
	}

	plan.SubjectTokenType = types.StringValue(subjectTokenType)

	result, err := verifyclient.ExchangeToken(
		ctx,
		verifyclient.TokenExchangeRequest{
			TenantURL:        plan.TenantURL.ValueString(),
			ClientID:         plan.ClientID.ValueString(),
			ClientSecret:     plan.ClientSecret.ValueString(),
			SubjectToken:     plan.SubjectToken.ValueString(),
			SubjectTokenType: subjectTokenType,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to exchange JWT",
			err.Error(),
		)
		return
	}

	plan.AccessToken = types.StringValue(result.AccessToken)
	plan.ExpiresIn = types.Int64Value(result.ExpiresIn)
	plan.GrantID = types.StringValue(result.GrantID)
	plan.IssuedTokenType = types.StringValue(result.IssuedTokenType)
	plan.Scope = types.StringValue(result.Scope)
	plan.TokenType = types.StringValue(result.TokenType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read preserves the token-exchange result stored in Terraform state.
func (r *TokenExchangeResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
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
