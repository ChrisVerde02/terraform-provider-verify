package resources

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// certDER decodes a PEM certificate string and returns the raw DER bytes.
// Returns nil if the PEM is empty or cannot be decoded.
func certDER(pemStr string) []byte {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil
	}
	// Validate it parses as a certificate (catches garbage data).
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil
	}
	return block.Bytes
}

// certPEMsMatch returns true when both PEM strings encode the same certificate.
func certPEMsMatch(a, b string) bool {
	da := certDER(a)
	db := certDER(b)
	if da == nil || db == nil {
		return false
	}
	return bytes.Equal(da, db)
}

var _ resource.Resource = &SignerCertResource{}
var _ resource.ResourceWithConfigure = &SignerCertResource{}
var _ resource.ResourceWithImportState = &SignerCertResource{}

// SignerCertResource implements the verify_signercert resource.
// It uploads a PEM certificate to IBM Verify as a signer certificate,
// which IBM Verify uses to validate JWT signatures during token exchange.
//
// The resource prefers the CertClient injected by the provider Configure().
// If no provider client is available it falls back to the per-resource
// cert_manager_client_id / cert_manager_client_secret fields.
type SignerCertResource struct {
	// certClient is injected by the provider — nil if provider block omits cert creds.
	certClient *verifyclient.Client
}

// SignerCertStateModel is stored in and read from Terraform state.
type SignerCertStateModel struct {
	TenantURL               types.String `tfsdk:"tenant_url"`
	CertManagerClientID     types.String `tfsdk:"cert_manager_client_id"`
	CertManagerClientSecret types.String `tfsdk:"cert_manager_client_secret"`
	CertificatePEM          types.String `tfsdk:"certificate_pem"`
	Label                   types.String `tfsdk:"label"`
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
			"kid header in the JWT (key_id in verify_jwt). " +
			"The resource obtains its own client credentials token internally " +
			"using cert_manager_client_id and cert_manager_client_secret.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL, e.g. https://example.verify.ibm.com.",
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

			"cert_manager_client_id": schema.StringAttribute{
				Description: "Client ID of the IBM Verify API client with manageCerts entitlement. " +
					"Optional when cert_manager_client_id is configured in the provider block.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"cert_manager_client_secret": schema.StringAttribute{
				Description: "Client secret of the cert-manager API client. " +
					"Optional when cert_manager_client_secret is configured in the provider block.",
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"certificate_pem": schema.StringAttribute{
				Description: "PEM-encoded X.509 certificate to upload.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						pemCertRegexp,
						`must be a PEM certificate beginning with "-----BEGIN CERTIFICATE-----"`,
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"label": schema.StringAttribute{
				Description: "Signer certificate label in IBM Verify. Must match the JWT kid header. " +
					"IBM Verify lowercases labels on storage — use lowercase. " +
					"Allowed characters: letters, digits, dots, hyphens, underscores (1–128 chars).",
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						labelRegexp,
						"must contain only letters, digits, dots, hyphens, or underscores (1–128 characters)",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure receives the ProviderData and extracts the CertClient.
func (r *SignerCertResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(interface{ GetCertClient() *verifyclient.Client })
	if ok {
		r.certClient = pd.GetCertClient()
	}
}

// certClientFor returns the SDK Client to use for cert operations.
// Priority: provider-injected client > per-resource credentials.
func (r *SignerCertResource) certClientFor(
	ctx context.Context,
	tenantURL, clientID, clientSecret string,
) (*verifyclient.Client, error) {
	if r.certClient != nil {
		return r.certClient, nil
	}
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf(
			"cert-manager credentials missing: set cert_manager_client_id / " +
				"cert_manager_client_secret in the resource, or configure " +
				"cert_manager_client_id / cert_manager_client_secret in the provider block")
	}
	return verifyclient.New(tenantURL,
		verifyclient.WithClientCredentials(clientID, clientSecret),
	)
}

// Create uploads the certificate to IBM Verify.
// If a certificate with the same label already exists and its content matches
// the desired cert (e.g. state was wiped without terraform destroy), it is
// adopted into state — no upload needed.
// If a cert with the same label exists but has DIFFERENT content (stale cert
// from a previous key pair), it is deleted and replaced with the correct one.
// This prevents CSIAQ5212E token integrity failures caused by cert/key mismatches.
func (r *SignerCertResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan SignerCertStateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.certClientFor(ctx,
		plan.TenantURL.ValueString(),
		plan.CertManagerClientID.ValueString(),
		plan.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build cert-manager client", err.Error())
		return
	}

	existing, err := c.Certs.Get(ctx, plan.Label.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to check existing signer certificate", err.Error())
		return
	}

	if existing != nil && certPEMsMatch(existing.Cert, plan.CertificatePEM.ValueString()) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	if existing != nil {
		if err = c.Certs.Delete(ctx, plan.Label.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to replace stale signer certificate in IBM Verify", err.Error())
			return
		}
	}

	if err = c.Certs.Import(ctx, plan.Label.ValueString(), plan.CertificatePEM.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to upload signer certificate to IBM Verify", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read verifies the certificate still exists in IBM Verify.
// If deleted externally, removes from state so Terraform re-uploads on next apply.
func (r *SignerCertResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state SignerCertStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.certClientFor(ctx,
		state.TenantURL.ValueString(),
		state.CertManagerClientID.ValueString(),
		state.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build cert-manager client for Read", err.Error())
		return
	}

	result, err := c.Certs.Get(ctx, state.Label.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read signer certificate from IBM Verify", err.Error())
		return
	}

	if result == nil {
		// Deleted outside Terraform — remove from state so it gets re-uploaded.
		resp.State.RemoveResource(ctx)
		return
	}

	if !certPEMsMatch(result.Cert, state.CertificatePEM.ValueString()) {
		// Cert exists in IBM Verify but has different content (stale key pair).
		// Remove from state so Terraform recreates it on the next apply,
		// which will trigger Create → delete stale + upload correct cert.
		resp.State.RemoveResource(ctx)
		return
	}
	// Certificate exists and matches — keep state as-is.
}

// ImportState brings an existing IBM Verify signer certificate under
// Terraform management. The import ID is the certificate label, e.g.:
//
//	terraform import module.signercert.verify_signercert.this demotokensigner
//
// The cert is fetched from IBM Verify and its PEM is stored in state.
// tenant_url is sourced from the provider-configured CertClient.
// cert_manager_client_id and cert_manager_client_secret are left empty —
// they will be populated from the provider block on the next plan/apply.
func (r *SignerCertResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	label := req.ID
	if label == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID must be the signer certificate label, e.g.: "+
				"terraform import verify_signercert.this demotokensigner",
		)
		return
	}

	if r.certClient == nil {
		resp.Diagnostics.AddError(
			"Provider not configured for import",
			"cert_manager_client_id and cert_manager_client_secret must be set in "+
				"the provider block before running terraform import on verify_signercert.",
		)
		return
	}

	result, err := r.certClient.Certs.Get(ctx, label)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to fetch signer certificate from IBM Verify",
			err.Error(),
		)
		return
	}
	if result == nil {
		resp.Diagnostics.AddError(
			"Signer certificate not found",
			fmt.Sprintf("No signer certificate with label %q exists in IBM Verify. "+
				"Verify the label and tenant URL in the provider block.", label),
		)
		return
	}

	// Populate state from the fetched cert.
	// tenant_url comes from the client's configured base URL.
	state := SignerCertStateModel{
		TenantURL:               types.StringValue(r.certClient.TenantURL()),
		CertManagerClientID:     types.StringValue(""),
		CertManagerClientSecret: types.StringValue(""),
		CertificatePEM:          types.StringValue(result.Cert),
		Label:                   types.StringValue(result.Label),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called — all fields use RequiresReplace.
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
	var state SignerCertStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.certClientFor(ctx,
		state.TenantURL.ValueString(),
		state.CertManagerClientID.ValueString(),
		state.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build cert-manager client for Delete", err.Error())
		return
	}

	if err = c.Certs.Delete(ctx, state.Label.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete signer certificate from IBM Verify", err.Error())
		return
	}
}
