package resources

import (
	"context"
	"fmt"
	"strings"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"
	generated "github.com/ChrisVerde02/ibmverify-go/generated"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &APIClientResource{}
var _ resource.ResourceWithConfigure = &APIClientResource{}
var _ resource.ResourceWithImportState = &APIClientResource{}

// APIClientResource implements the verify_api_client resource.
// It creates and manages an IBM Verify Dynamic Client Registration (DCR) API client.
// The resource uses the APIClientsClient injected by the provider Configure().
type APIClientResource struct {
	apiClientsClient *verifyclient.Client
}

// APIClientStateModel is stored in and read from Terraform state.
type APIClientStateModel struct {
	TenantURL    types.String `tfsdk:"tenant_url"`
	ClientName   types.String `tfsdk:"client_name"`
	Entitlements types.List   `tfsdk:"entitlements"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Description  types.String `tfsdk:"description"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

// NewAPIClientResource creates the resource.
func NewAPIClientResource() resource.Resource {
	return &APIClientResource{}
}

// Metadata defines the Terraform resource name.
func (r *APIClientResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_api_client"
}

// Schema defines the resource inputs and outputs.
func (r *APIClientResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages an IBM Verify API client via Dynamic Client Registration (DCR). " +
			"The generated client_id and client_secret are stored in state and can be used " +
			"as credentials for other IBM Verify resources. " +
			"Deleting this resource permanently removes the API client from IBM Verify.",

		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Description: "IBM Verify tenant base URL, e.g. https://example.verify.ibm.com.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						URLRegexp,
						"must be a valid HTTPS URL, e.g. https://example.verify.ibm.com",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"client_name": schema.StringAttribute{
				Description: "Friendly display name for the API client in IBM Verify.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"entitlements": schema.ListAttribute{
				Description: "List of entitlements granted to this API client " +
					"(e.g. [\"manageApiClients\", \"manageCerts\"]). " +
					"Changing this forces a new API client to be created.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},

			"enabled": schema.BoolAttribute{
				Description: "Whether the API client is enabled and can generate tokens. Defaults to true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			"description": schema.StringAttribute{
				Description: "Optional description of the API client.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"client_id": schema.StringAttribute{
				Description: "The generated client ID assigned by IBM Verify. " +
					"Use this as a credential in other resources.",
				Computed: true,
			},

			"client_secret": schema.StringAttribute{
				Description: "The generated client secret assigned by IBM Verify. " +
					"Sensitive — stored in state.",
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

// Configure receives the ProviderData and extracts the APIClientsClient.
func (r *APIClientResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(interface{ GetAPIClientsClient() *verifyclient.Client })
	if ok {
		r.apiClientsClient = pd.GetAPIClientsClient()
	}
}

// apiClientsClientFor returns the SDK Client for DCR operations.
func (r *APIClientResource) apiClientsClientFor(ctx context.Context) (*verifyclient.Client, error) {
	if r.apiClientsClient != nil {
		return r.apiClientsClient, nil
	}
	return nil, fmt.Errorf(
		"api clients client not configured: set sts_client_id and sts_client_secret " +
			"in the provider block",
	)
}

// Create registers a new API client in IBM Verify via DCR.
// Idempotent: if an API client with the same clientName already exists it is
// adopted into state without creating a duplicate. Running terraform apply
// twice, or wiping state and re-applying, is always safe.
func (r *APIClientResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan APIClientStateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.apiClientsClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build API clients client", err.Error())
		return
	}

	// --- Idempotency check ---
	// List all existing API clients and look for one matching clientName.
	// If found, adopt it into state instead of creating a duplicate.
	existingList, listErr := c.APIClients.List(ctx, nil)
	if listErr == nil {
		wantName := plan.ClientName.ValueString()
		for _, client := range existingList {
			if name, ok := client["clientName"].(string); ok && name == wantName {
				APIClientStateFromMap(ctx, &plan, client)
				resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
				return
			}
		}
	}

	// No existing match — create a new API client.
	var entitlements []string
	resp.Diagnostics.Append(plan.Entitlements.ElementsAs(ctx, &entitlements, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := true
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		enabled = plan.Enabled.ValueBool()
	}

	createReq := &generated.APIClientConfigRequest{
		ClientName:   plan.ClientName.ValueString(),
		Entitlements: entitlements,
		Enabled:      enabled,
	}
	if desc := plan.Description.ValueString(); desc != "" {
		createReq.Description = &desc
	}

	m, err := c.APIClients.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create API client in IBM Verify", err.Error())
		return
	}

	APIClientStateFromMap(ctx, &plan, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the API client state from IBM Verify.
// If the client was deleted outside Terraform, it is removed from state.
func (r *APIClientResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state APIClientStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.apiClientsClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build API clients client for Read", err.Error())
		return
	}

	m, err := c.APIClients.Get(ctx, state.ClientID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read API client from IBM Verify", err.Error())
		return
	}

	APIClientStateFromMap(ctx, &state, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called — all fields use RequiresReplace or UseStateForUnknown.
func (r *APIClientResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the API client from IBM Verify.
func (r *APIClientResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state APIClientStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.apiClientsClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build API clients client for Delete", err.Error())
		return
	}

	if err := c.APIClients.Delete(ctx, state.ClientID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete API client from IBM Verify", err.Error())
		return
	}
}

// ImportState brings an existing IBM Verify API client under Terraform management.
// The import ID is the client_id (UUID), e.g.:
//
//	terraform import verify_api_client.example a1b2c3d4-5678-...
func (r *APIClientResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	clientID := req.ID
	if clientID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID must be the API client_id (UUID), e.g.: "+
				"terraform import verify_api_client.this a1b2c3d4-5678-...",
		)
		return
	}

	if r.apiClientsClient == nil {
		resp.Diagnostics.AddError(
			"Provider not configured for import",
			"sts_client_id / sts_client_secret must be set in the provider block "+
				"before running terraform import on verify_api_client.",
		)
		return
	}

	m, err := r.apiClientsClient.APIClients.Get(ctx, clientID)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			resp.Diagnostics.AddError(
				"API client not found",
				fmt.Sprintf("No API client with ID %q exists in IBM Verify. "+
					"Verify the ID and tenant URL in the provider block.", clientID),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch API client from IBM Verify", err.Error())
		return
	}

	state := APIClientStateModel{
		TenantURL: types.StringValue(r.apiClientsClient.TenantURL()),
	}
	APIClientStateFromMap(ctx, &state, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// APIClientStateFromMap fills an APIClientStateModel from a raw DCR response map.
// client_secret is only present in the Create response; GET responses never include it.
// When the secret is absent from the map, the existing state value is preserved so that
// Terraform never sees an unknown/null for a Computed field after apply.
// Exported for use in tests.
func APIClientStateFromMap(ctx context.Context, state *APIClientStateModel, m map[string]interface{}) {
	if id, ok := m["clientId"].(string); ok {
		state.ClientID = types.StringValue(id)
	}
	if name, ok := m["clientName"].(string); ok {
		state.ClientName = types.StringValue(name)
	}
	if secret, ok := m["clientSecret"].(string); ok && secret != "" {
		state.ClientSecret = types.StringValue(secret)
	} else if state.ClientSecret.IsNull() || state.ClientSecret.IsUnknown() {
		// Secret not returned by IBM Verify (GET path or adoption path).
		// Store empty string so the field is always known in state.
		state.ClientSecret = types.StringValue("")
	}
	// else: preserve the existing non-empty secret already in state.
	if desc, ok := m["description"].(string); ok {
		state.Description = types.StringValue(desc)
	} else if state.Description.IsNull() || state.Description.IsUnknown() {
		state.Description = types.StringValue("")
	}
	if enabled, ok := m["enabled"].(bool); ok {
		state.Enabled = types.BoolValue(enabled)
	} else {
		state.Enabled = types.BoolValue(true)
	}

	// Entitlements — raw JSON array of strings.
	if raw, ok := m["entitlements"].([]interface{}); ok {
		strs := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		list, d := types.ListValueFrom(ctx, types.StringType, strs)
		if d.HasError() {
			return
		}
		state.Entitlements = list
	} else if state.Entitlements.IsNull() || state.Entitlements.IsUnknown() {
		list, _ := types.ListValueFrom(ctx, types.StringType, []string{})
		state.Entitlements = list
	}
}
