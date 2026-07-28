package resources

import (
	"context"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TokenIntrospectionResource{}

// TokenIntrospectionResource implements verify_token_introspection.
type TokenIntrospectionResource struct{}

// TokenIntrospectionResourceModel represents configuration and state.
type TokenIntrospectionResourceModel struct {
	TenantURL    types.String `tfsdk:"tenant_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Token        types.String `tfsdk:"token"`

	Active    types.Bool   `tfsdk:"active"`
	Username  types.String `tfsdk:"username"`
	Subject   types.String `tfsdk:"subject"`
	Scope     types.String `tfsdk:"scope"`
	TokenType types.String `tfsdk:"token_type"`
	Issuer    types.String `tfsdk:"issuer"`
	IssuedAt  types.Int64  `tfsdk:"issued_at"`
	ExpiresAt types.Int64  `tfsdk:"expires_at"`
}

// NewTokenIntrospectionResource creates the resource.
func NewTokenIntrospectionResource() resource.Resource {
	return &TokenIntrospectionResource{}
}

// Metadata defines the Terraform resource name.
func (r *TokenIntrospectionResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_token_introspection"
}

// Schema defines the resource inputs and outputs.
func (r *TokenIntrospectionResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Introspects an IBM Verify access token.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"client_id": schema.StringAttribute{
				Description: "STS client ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"client_secret": schema.StringAttribute{
				Description: "STS client secret.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"token": schema.StringAttribute{
				Description: "Access token to introspect.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"active": schema.BoolAttribute{
				Description: "Whether IBM Verify considers the token active.",
				Computed:    true,
			},

			"username": schema.StringAttribute{
				Description: "Username associated with the token.",
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

// Create introspects the access token.
func (r *TokenIntrospectionResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan TokenIntrospectionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := verifyclient.IntrospectToken(
		ctx,
		verifyclient.IntrospectionRequest{
			TenantURL:    plan.TenantURL.ValueString(),
			ClientID:     plan.ClientID.ValueString(),
			ClientSecret: plan.ClientSecret.ValueString(),
			Token:        plan.Token.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to introspect access token",
			err.Error(),
		)
		return
	}

	plan.Active = types.BoolValue(result.Active)
	plan.Username = types.StringValue(result.Username)
	plan.Subject = types.StringValue(result.Subject)
	plan.Scope = types.StringValue(result.Scope)
	plan.TokenType = types.StringValue(result.TokenType)
	plan.Issuer = types.StringValue(result.Issuer)
	plan.IssuedAt = types.Int64Value(result.IssuedAt)
	plan.ExpiresAt = types.Int64Value(result.ExpiresAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read preserves the introspection response in state.
func (r *TokenIntrospectionResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
}

// Update is unused because all inputs require replacement.
func (r *TokenIntrospectionResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the introspection result from state.
func (r *TokenIntrospectionResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
}
