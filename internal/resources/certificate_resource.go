package resources

import (
	"context"
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

// Ensure the resource satisfies the Terraform interface.
var _ resource.Resource = &CertificateResource{}
var _ resource.ResourceWithImportState = &CertificateResource{}

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
	// ExpiresAt is the Unix timestamp when the certificate's NotAfter date
	// is reached. Read() uses this to decide whether to regenerate.
	ExpiresAt types.Int64 `tfsdk:"expires_at"`
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
		Description: "Generates a self-signed X.509 certificate. " +
			"The certificate is reused across plans until it expires, " +
			"then automatically regenerated.",

		Attributes: map[string]schema.Attribute{

			"common_name": schema.StringAttribute{
				Description: "Certificate Common Name (CN) — also used as the IBM Verify signer cert label.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"organization": schema.StringAttribute{
				Description: "Certificate Organization (O) field.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"country": schema.StringAttribute{
				Description: "Certificate Country (C) field — must be a two-letter ISO 3166-1 country code.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(2, 2),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"validity_days": schema.Int64Attribute{
				Description: "Certificate validity period in days. Must be at least 1.",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"key_size": schema.Int64Attribute{
				Description: "RSA key size in bits. Must be 2048, 3072, or 4096.",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.OneOf(2048, 3072, 4096),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"certificate_pem": schema.StringAttribute{
				Description: "Generated X.509 certificate in PEM format.",
				Computed:    true,
			},

			"private_key_pem": schema.StringAttribute{
				Description: "RSA private key in PEM format.",
				Computed:    true,
				Sensitive:   true,
			},

			"expires_at": schema.Int64Attribute{
				Description: "Certificate expiry as a Unix timestamp (NotAfter). " +
					"The resource regenerates the certificate automatically when " +
					"this timestamp is within 24 hours of the current time.",
				Computed: true,
			},
		},
	}
}

// ImportState is intentionally unsupported for verify_certificate.
// This resource generates its key pair and certificate locally — there is
// no remote counterpart to import. Use data.verify_jwt or
// data.verify_token_exchange for stateless operations.
func (r *CertificateResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.AddError(
		"Import not supported for verify_certificate",
		"verify_certificate generates its RSA key pair and certificate locally — "+
			"there is no remote resource to import. "+
			"Use the data.verify_jwt or data.verify_token_exchange data sources "+
			"for stateless operations.",
	)
}


// Create generates the certificate and stores expiry in state.
func (r *CertificateResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan CertificateResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	plan.CertificatePEM = types.StringValue(result.CertificatePEM)
	plan.PrivateKeyPEM = types.StringValue(result.PrivateKeyPEM)
	// Store expiry as Unix timestamp so Read() can compare against time.Now().
	plan.ExpiresAt = types.Int64Value(
		time.Now().UTC().AddDate(0, 0, int(plan.ValidityDays.ValueInt64())).Unix(),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read checks whether the certificate has expired.
// If it is still valid (more than 24 hours from expiry) the existing state
// is kept unchanged — no new certificate is generated.
// If it has expired (or is within 24 hours of expiry) a new certificate and
// key pair are generated and stored, making the resource self-healing.
func (r *CertificateResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state CertificateResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 24-hour buffer — regenerate before the cert actually expires.
	const bufferSeconds = 24 * 60 * 60

	if time.Now().Unix() < state.ExpiresAt.ValueInt64()-bufferSeconds {
		// Certificate is still valid — keep existing state as-is.
		return
	}

	// Certificate has expired or is about to — regenerate.
	result, err := providercrypto.GenerateSelfSignedCertificate(
		providercrypto.CertificateRequest{
			CommonName:   state.CommonName.ValueString(),
			Organization: state.Organization.ValueString(),
			Country:      state.Country.ValueString(),
			ValidityDays: int(state.ValidityDays.ValueInt64()),
			KeySize:      int(state.KeySize.ValueInt64()),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to regenerate expired certificate",
			err.Error(),
		)
		return
	}

	state.CertificatePEM = types.StringValue(result.CertificatePEM)
	state.PrivateKeyPEM = types.StringValue(result.PrivateKeyPEM)
	state.ExpiresAt = types.Int64Value(
		time.Now().UTC().AddDate(0, 0, int(state.ValidityDays.ValueInt64())).Unix(),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unused — all configurable fields use RequiresReplace().
func (r *CertificateResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the resource from Terraform state.
func (r *CertificateResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
}
