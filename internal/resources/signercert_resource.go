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

var _ resource.Resource = &SignerCertResource{}

// SignerCertResource implements the verify_signercert resource.
// It uploads a PEM certificate to IBM Verify as a signer certificate,
// which IBM Verify uses to validate JWT signatures during token exchange.
type SignerCertResource struct{}

// SignerCertResourceModel represents the Terraform state.
type SignerCertResourceModel struct {
	TenantURL      types.String `tfsdk:"tenant_url"`
	AccessToken    types.String `tfsdk:"access_token"`
	CertificatePEM types.String `tfsdk:"certificate_pem"`
	// Label is the friendly name in IBM Verify — must match the kid in the JWT.
	Label types.String `tfsdk:"label"`
}

// NewSignerCertResource creates the resource.
func NewSignerCertResource() resource.Resource {
	return &SignerCertResource{}
}

// Metadata defines the Terraform resource name.
func (r *SignerCertResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_signercert"
}

// Schema defines the resource inputs and outputs.
func (r *SignerCertResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Uploads a signer certificate to IBM Verify. " +
			"IBM Verify uses this certificate to validate the signature of " +
			"custom JWTs during token exchange. The label must match the " +
			"kid header in the JWT (key_id in verify_jwt).",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"access_token": schema.StringAttribute{
				Description: "IBM Verify access token with manageCerts entitlement.",
				Required:    true,
				Sensitive:   true,
				// No RequiresReplace — the access token is a credential used to
				// make the API call, not part of the certificate identity. A new
				// token on each plan does not mean the cert needs to be re-uploaded.
			},

			"certificate_pem": schema.StringAttribute{
				Description: "PEM-encoded X.509 certificate to upload.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"label": schema.StringAttribute{
				Description: "Friendly name / alias for the certificate in IBM Verify. " +
					"Must match the kid header used in the JWT.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Create uploads the certificate to IBM Verify.
func (r *SignerCertResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan SignerCertResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := verifyclient.ImportSignerCert(
		ctx,
		verifyclient.SignerCertRequest{
			TenantURL:      plan.TenantURL.ValueString(),
			AccessToken:    plan.AccessToken.ValueString(),
			CertificatePEM: plan.CertificatePEM.ValueString(),
			Label:          plan.Label.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upload signer certificate to IBM Verify",
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read verifies the certificate still exists in IBM Verify.
// If it has been deleted externally, the resource is removed from state
// so Terraform will re-upload it on the next apply.
func (r *SignerCertResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state SignerCertResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := verifyclient.GetSignerCert(
		ctx,
		state.TenantURL.ValueString(),
		state.Label.ValueString(),
		state.AccessToken.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read signer certificate from IBM Verify",
			err.Error(),
		)
		return
	}

	if result == nil {
		// Certificate was deleted outside of Terraform — remove from state
		// so it gets re-uploaded on next apply.
		resp.State.RemoveResource(ctx)
		return
	}

	// Certificate still exists — keep state as-is.
}

// Update is unused — all inputs require replacement.
func (r *SignerCertResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the certificate from IBM Verify.
func (r *SignerCertResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state SignerCertResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := verifyclient.DeleteSignerCert(
		ctx,
		state.TenantURL.ValueString(),
		state.Label.ValueString(),
		state.AccessToken.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete signer certificate from IBM Verify",
			err.Error(),
		)
		return
	}
}
