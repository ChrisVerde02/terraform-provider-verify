package resources

import (
	"context"

	providercrypto "github.com/Christian-Verderame/terraform-provider-verify/internal/crypto"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the resource satisfies the Terraform interface.
var _ resource.Resource = &CertificateResource{}

// CertificateResource implements the verify_certificate resource.
type CertificateResource struct{}

// CertificateResourceModel represents the Terraform state.
type CertificateResourceModel struct {
	CommonName   types.String `tfsdk:"common_name"`
	Organization types.String `tfsdk:"organization"`
	Country      types.String `tfsdk:"country"`
	ValidityDays types.Int64  `tfsdk:"validity_days"`
	KeySize      types.Int64  `tfsdk:"key_size"`

	CertificatePEM types.String `tfsdk:"certificate_pem"`
	PrivateKeyPEM  types.String `tfsdk:"private_key_pem"`
}

// NewCertificateResource creates the resource.
func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

// Metadata defines the Terraform resource name.
func (r *CertificateResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

// Schema defines the resource schema.
func (r *CertificateResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {

	resp.Schema = schema.Schema{
		Description: "Generates a self-signed X.509 certificate.",

		Attributes: map[string]schema.Attribute{

			"common_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"organization": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"country": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"validity_days": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"key_size": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"certificate_pem": schema.StringAttribute{
				Computed: true,
			},

			"private_key_pem": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

// Create generates the certificate.
func (r *CertificateResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {

	var plan CertificateResourceModel

	// Read Terraform configuration.
	resp.Diagnostics.Append(
		req.Plan.Get(ctx, &plan)...,
	)

	if resp.Diagnostics.HasError() {
		return
	}

	// Generate certificate.
	result, err := providercrypto.GenerateSelfSignedCertificate(
		providercrypto.CertificateRequest{
			CommonName:   plan.CommonName.ValueString(),
			Organization: plan.Organization.ValueString(),
			Country:      plan.Country.ValueString(),
			ValidityDays: int(plan.ValidityDays.ValueInt64()),
			KeySize:      int(plan.KeySize.ValueInt64()),
		},
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to generate certificate",
			err.Error(),
		)
		return
	}

	// Save generated values into Terraform state.
	plan.CertificatePEM = types.StringValue(result.CertificatePEM)
	plan.PrivateKeyPEM = types.StringValue(result.PrivateKeyPEM)

	resp.Diagnostics.Append(
		resp.State.Set(ctx, &plan)...,
	)
}

// Read refreshes the resource state.
func (r *CertificateResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	// Nothing to refresh.
}

// Update updates the resource.
func (r *CertificateResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	// Every configurable field RequiresReplace(),
	// so Terraform will recreate the resource.
}

// Delete removes the resource.
func (r *CertificateResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	// Nothing to delete.
}
