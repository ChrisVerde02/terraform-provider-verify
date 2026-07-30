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
type SignerCertResource struct{}

// SignerCertStateModel is stored in and read from Terraform state.
// cert_manager_client_id and cert_manager_client_secret are stored so that
// Read() and Delete() can obtain a fresh access token without needing it
// passed in (access_token itself is write-only and never stored).
type SignerCertStateModel struct {
	TenantURL              types.String `tfsdk:"tenant_url"`
	AccessToken            types.String `tfsdk:"access_token"`
	CertManagerClientID    types.String `tfsdk:"cert_manager_client_id"`
	CertManagerClientSecret types.String `tfsdk:"cert_manager_client_secret"`
	CertificatePEM         types.String `tfsdk:"certificate_pem"`
	Label                  types.String `tfsdk:"label"`
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

			// access_token is WriteOnly — never stored in state, never diffed.
			// The cert-manager client credentials are stored instead so that
			// Read() and Delete() can obtain a fresh token independently.
			"access_token": schema.StringAttribute{
				Description: "IBM Verify access token with manageCerts entitlement. " +
					"Write-only — never stored in state.",
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
			},

			// cert_manager_client_id and cert_manager_client_secret are stored
			// in state so Read() and Delete() can get a fresh token when needed.
			"cert_manager_client_id": schema.StringAttribute{
				Description: "Client ID of the cert-manager API client. " +
					"Stored in state so Read and Delete can obtain a fresh token.",
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

// freshToken obtains a new client credentials access token from IBM Verify.
func freshToken(ctx context.Context, tenantURL, clientID, clientSecret string) (string, error) {
	endpoint := strings.TrimRight(tenantURL, "/") + "/v1.0/endpoint/default/token"
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
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
		return "", err
	}
	return result.AccessToken, nil
}

// Create uploads the certificate to IBM Verify.
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
		resp.Diagnostics.AddError("Unable to upload signer certificate to IBM Verify", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read verifies the certificate still exists in IBM Verify.
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

	token, err := freshToken(ctx,
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
		resp.State.RemoveResource(ctx)
		return
	}
	// Certificate still exists — keep state as-is.
}

// Update is never called — all stored fields use RequiresReplace.
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

	token, err := freshToken(ctx,
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
