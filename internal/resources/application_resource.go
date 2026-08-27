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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ApplicationResource{}
var _ resource.ResourceWithConfigure = &ApplicationResource{}
var _ resource.ResourceWithImportState = &ApplicationResource{}

// ApplicationResource implements the verify_application resource.
// It creates and manages an IBM Verify application, which acts as the
// integration point for token exchange and SSO configuration.
//
// The resource uses the AppsClient injected by the provider Configure().
type ApplicationResource struct {
	// appsClient is injected by the provider — nil if provider block omits app creds.
	appsClient *verifyclient.Client
}

// ApplicationStateModel is stored in and read from Terraform state.
type ApplicationStateModel struct {
	TenantURL        types.String `tfsdk:"tenant_url"`
	Name             types.String `tfsdk:"name"`
	TemplateID       types.String `tfsdk:"template_id"`
	ApplicationID    types.String `tfsdk:"application_id"`
	ApplicationState types.String `tfsdk:"application_state"`
}

// NewApplicationResource creates the resource.
func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

// Metadata defines the Terraform resource name.
func (r *ApplicationResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// Schema defines the resource inputs and outputs.
func (r *ApplicationResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages an IBM Verify application. " +
			"Applications are the integration point for SSO, token exchange, " +
			"and identity federation in IBM Verify. " +
			"Deleting this resource permanently removes the application from IBM Verify.",

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

			"name": schema.StringAttribute{
				Description: "Display name of the application in IBM Verify.",
				Required:    true,
			},

			"template_id": schema.StringAttribute{
				Description: "IBM Verify template ID that defines the application type " +
					"(e.g. SAML, OIDC). Changing this forces a new application to be created.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"application_id": schema.StringAttribute{
				Description: "Application ID assigned by IBM Verify on creation.",
				Computed:    true,
			},

			"application_state": schema.StringAttribute{
				Description: `Application state as reported by IBM Verify — "true" means active, "false" means draft.`,
				Computed:    true,
			},
		},
	}
}

// Configure receives the ProviderData and extracts the AppsClient.
func (r *ApplicationResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(interface{ GetAppsClient() *verifyclient.Client })
	if ok {
		r.appsClient = pd.GetAppsClient()
	}
}

// appsClientFor returns the SDK Client to use for application operations.
// The provider-injected client is always required for applications —
// per-resource credential fallback is not supported for this resource type.
func (r *ApplicationResource) appsClientFor(
	ctx context.Context,
	tenantURL string,
) (*verifyclient.Client, error) {
	if r.appsClient != nil {
		return r.appsClient, nil
	}
	return nil, fmt.Errorf(
		"apps client not configured: set app_client_id and app_client_secret " +
			"(or sts_client_id and sts_client_secret) in the provider block",
	)
}

// appIDFromHref extracts the application UUID from an IBM Verify self-link href.
// IBM Verify returns hrefs like: https://tenant.verify.ibm.com/v1.0/applications/<uuid>
func appIDFromHref(href string) string {
	href = strings.TrimRight(href, "/")
	return href[strings.LastIndex(href, "/")+1:]
}

// Create registers a new application in IBM Verify.
// Idempotent: if an application with the same name and templateId already exists
// it is adopted into state without creating a duplicate. This means running
// terraform apply twice, or wiping state and re-applying, is always safe.
func (r *ApplicationResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan ApplicationStateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.appsClientFor(ctx, plan.TenantURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to build apps client", err.Error())
		return
	}

	// --- Idempotency check ---
	// List all existing applications and look for one matching name + templateId.
	// If found, adopt it into state instead of creating a duplicate.
	existing, listErr := c.Apps.List(ctx, nil)
	if listErr == nil {
		wantName := plan.Name.ValueString()
		wantTemplate := plan.TemplateID.ValueString()
		for _, app := range existing {
			name, _ := app["name"].(string)
			tmpl, _ := app["templateId"].(string)
			if name == wantName && tmpl == wantTemplate {
				// Found a matching application — adopt it.
				existingID := appIDFromHref(
					func() string {
						if links, ok := app["_links"].(map[string]interface{}); ok {
							if self, ok := links["self"].(map[string]interface{}); ok {
								if href, ok := self["href"].(string); ok {
									return href
								}
							}
						}
						return ""
					}(),
				)
				if existingID != "" {
					plan.ApplicationID = types.StringValue(existingID)
					plan.ApplicationState = types.StringValue(fmt.Sprintf("%v", app["applicationState"]))
					resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
					return
				}
			}
		}
	}

	// No existing match — create a new application.
	result, err := c.Apps.Create(ctx, &generated.ApplicationRequestBean{
		Name:       plan.Name.ValueString(),
		TemplateID: plan.TemplateID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create application in IBM Verify", err.Error())
		return
	}

	// IBM Verify returns the application ID in the self-link href.
	if result.GetLinks() == nil || result.GetLinks().GetSelf() == nil {
		resp.Diagnostics.AddError(
			"Unexpected response from IBM Verify",
			"Create application response did not include a self-link href. "+
				"Cannot determine the application ID.",
		)
		return
	}
	appID := appIDFromHref(result.GetLinks().GetSelf().GetHref())
	if appID == "" {
		resp.Diagnostics.AddError(
			"Unable to extract application ID",
			fmt.Sprintf("Self-link href was %q — could not parse application ID from path.",
				result.GetLinks().GetSelf().GetHref()),
		)
		return
	}

	plan.ApplicationID = types.StringValue(appID)

	// Fetch the full record to populate computed fields.
	m, err := c.Apps.Get(ctx, appID)
	if err != nil {
		plan.ApplicationState = types.StringValue("")
	} else {
		plan.ApplicationState = types.StringValue(fmt.Sprintf("%v", m["applicationState"]))
		if nameVal, ok := m["name"].(string); ok && nameVal != "" {
			plan.Name = types.StringValue(nameVal)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the application state from IBM Verify.
// If the application was deleted outside Terraform, it is removed from state.
func (r *ApplicationResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state ApplicationStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.appsClientFor(ctx, state.TenantURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to build apps client for Read", err.Error())
		return
	}

	m, err := c.Apps.Get(ctx, state.ApplicationID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			// Application was deleted outside Terraform — remove from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read application from IBM Verify", err.Error())
		return
	}

	if nameVal, ok := m["name"].(string); ok && nameVal != "" {
		state.Name = types.StringValue(nameVal)
	}
	state.ApplicationState = types.StringValue(fmt.Sprintf("%v", m["applicationState"]))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called — all mutable fields use RequiresReplace or are Computed.
func (r *ApplicationResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the application from IBM Verify.
func (r *ApplicationResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state ApplicationStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.appsClientFor(ctx, state.TenantURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to build apps client for Delete", err.Error())
		return
	}

	if err := c.Apps.Delete(ctx, state.ApplicationID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete application from IBM Verify", err.Error())
		return
	}
}

// ImportState brings an existing IBM Verify application under Terraform management.
// The import ID is the application UUID, e.g.:
//
//	terraform import verify_application.example a1b2c3d4-5678-...
func (r *ApplicationResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	appID := req.ID
	if appID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID must be the application UUID, e.g.: "+
				"terraform import verify_application.this a1b2c3d4-5678-...",
		)
		return
	}

	if r.appsClient == nil {
		resp.Diagnostics.AddError(
			"Provider not configured for import",
			"app_client_id / app_client_secret (or sts_client_id / sts_client_secret) "+
				"must be set in the provider block before running terraform import on verify_application.",
		)
		return
	}

	m, err := r.appsClient.Apps.Get(ctx, appID)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			resp.Diagnostics.AddError(
				"Application not found",
				fmt.Sprintf("No application with ID %q exists in IBM Verify. "+
					"Verify the ID and tenant URL in the provider block.", appID),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch application from IBM Verify", err.Error())
		return
	}

	name, _ := m["name"].(string)
	templateID, _ := m["templateId"].(string)

	state := ApplicationStateModel{
		TenantURL:        types.StringValue(r.appsClient.TenantURL()),
		Name:             types.StringValue(name),
		TemplateID:       types.StringValue(templateID),
		ApplicationID:    types.StringValue(appID),
		ApplicationState: types.StringValue(fmt.Sprintf("%v", m["applicationState"])),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
