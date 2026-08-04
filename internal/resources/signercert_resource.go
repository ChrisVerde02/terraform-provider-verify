package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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
// The resource obtains its own client credentials token internally so
// access_token never needs to be stored in state or diffed.
type SignerCertResource struct{}

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
				Description: "IBM Verify tenant base URL.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"cert_manager_client_id": schema.StringAttribute{
				Description: "Client ID of the IBM Verify API client with manageCerts entitlement. " +
					"Used to obtain a client credentials token for all cert operations.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"cert_manager_client_secret": schema.StringAttribute{
				Description: "Client secret of the cert-manager API client.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

// getToken obtains a fresh client credentials access token from IBM Verify.
func (r *SignerCertResource) getToken(ctx context.Context, tenantURL, clientID, clientSecret string) (string, error) {
	endpoint := strings.TrimRight(tenantURL, "/") + "/v1.0/endpoint/default/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("token response contained no access_token")
	}
	return result.AccessToken, nil
}

// Create uploads the certificate to IBM Verify.
// If a certificate with the same label already exists (e.g. state was wiped
// without running terraform destroy), it is adopted into state rather than
// failing — so you never need to manually delete the cert from IBM Verify.
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

	token, err := r.getToken(ctx,
		plan.TenantURL.ValueString(),
		plan.CertManagerClientID.ValueString(),
		plan.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to get cert-manager token", err.Error())
		return
	}

	// Check whether the cert already exists in IBM Verify before uploading.
	// This handles the case where terraform.tfstate was wiped without running
	// terraform destroy — the cert is already there, so we just adopt it.
	existing, err := verifyclient.GetSignerCert(ctx,
		plan.TenantURL.ValueString(),
		plan.Label.ValueString(),
		token,
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to check existing signer certificate", err.Error())
		return
	}

	if existing == nil {
		// Cert does not exist — upload it normally.
		err = verifyclient.ImportSignerCert(ctx, verifyclient.SignerCertRequest{
			TenantURL:      plan.TenantURL.ValueString(),
			AccessToken:    token,
			CertificatePEM: plan.CertificatePEM.ValueString(),
			Label:          plan.Label.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to upload signer certificate to IBM Verify", err.Error())
			return
		}
	}
	// If existing != nil, the cert is already in IBM Verify — adopt it into
	// state silently. No upload needed.

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

	token, err := r.getToken(ctx,
		state.TenantURL.ValueString(),
		state.CertManagerClientID.ValueString(),
		state.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to get cert-manager token for Read", err.Error())
		return
	}

	result, err := verifyclient.GetSignerCert(ctx, state.TenantURL.ValueString(), state.Label.ValueString(), token)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read signer certificate from IBM Verify", err.Error())
		return
	}

	if result == nil {
		// Deleted outside Terraform — remove from state so it gets re-uploaded.
		resp.State.RemoveResource(ctx)
		return
	}
	// Certificate still exists — keep state as-is.
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

	token, err := r.getToken(ctx,
		state.TenantURL.ValueString(),
		state.CertManagerClientID.ValueString(),
		state.CertManagerClientSecret.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to get cert-manager token for Delete", err.Error())
		return
	}

	err = verifyclient.DeleteSignerCert(ctx, state.TenantURL.ValueString(), state.Label.ValueString(), token)
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete signer certificate from IBM Verify", err.Error())
		return
	}
}
