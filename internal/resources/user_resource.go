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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithConfigure = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

// UserResource implements the verify_user resource.
// It creates and manages an IBM Verify Cloud Directory user via the SCIM v2 API.
//
// The resource uses the UsersClient injected by the provider Configure().
type UserResource struct {
	usersClient *verifyclient.Client
}

// UserStateModel is stored in and read from Terraform state.
type UserStateModel struct {
	TenantURL  types.String `tfsdk:"tenant_url"`
	UserName   types.String `tfsdk:"username"`
	GivenName  types.String `tfsdk:"given_name"`
	FamilyName types.String `tfsdk:"family_name"`
	Email      types.String `tfsdk:"email"`
	Password   types.String `tfsdk:"password"`
	Active     types.Bool   `tfsdk:"active"`
	UserID     types.String `tfsdk:"user_id"`
	DisplayName types.String `tfsdk:"display_name"`
}

// NewUserResource creates the resource.
func NewUserResource() resource.Resource {
	return &UserResource{}
}

// Metadata defines the Terraform resource name.
func (r *UserResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema defines the resource inputs and outputs.
func (r *UserResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages an IBM Verify Cloud Directory user via the SCIM v2 API. " +
			"Deleting this resource permanently removes the user from IBM Verify.",

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

			"username": schema.StringAttribute{
				Description: "The unique username (userName) for the user in IBM Verify. " +
					"This is the identifier used to log in. Changing this forces a new user to be created.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"given_name": schema.StringAttribute{
				Description: "The user's first (given) name.",
				Optional:    true,
				Computed:    true,
			},

			"family_name": schema.StringAttribute{
				Description: "The user's last (family) name.",
				Optional:    true,
				Computed:    true,
			},

			"email": schema.StringAttribute{
				Description: "The user's primary work email address.",
				Optional:    true,
				Computed:    true,
			},

			"password": schema.StringAttribute{
				Description: "Initial password for the user. Sensitive — stored in state. " +
					"IBM Verify does not return the password on Read; changes to this field " +
					"are not detected after creation.",
				Optional:  true,
				Sensitive: true,
			},

			"active": schema.BoolAttribute{
				Description: "Whether the user account is active. Defaults to true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			"user_id": schema.StringAttribute{
				Description: "The unique user ID assigned by IBM Verify on creation (SCIM id field).",
				Computed:    true,
			},

			"display_name": schema.StringAttribute{
				Description: "The display name of the user as returned by IBM Verify.",
				Computed:    true,
			},
		},
	}
}

// Configure receives the ProviderData and extracts the UsersClient.
func (r *UserResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(interface{ GetUsersClient() *verifyclient.Client })
	if ok {
		r.usersClient = pd.GetUsersClient()
	}
}

// usersClientFor returns the SDK Client for user operations.
func (r *UserResource) usersClientFor(ctx context.Context) (*verifyclient.Client, error) {
	if r.usersClient != nil {
		return r.usersClient, nil
	}
	return nil, fmt.Errorf(
		"users client not configured: set sts_client_id and sts_client_secret " +
			"in the provider block",
	)
}

// strPtr is a convenience helper that returns a pointer to a string value.
func strPtr(s string) *string { return &s }

// Create creates a new user in IBM Verify via the SCIM v2 API.
// Idempotent: if a user with the same userName already exists it is adopted
// into state without creating a duplicate. Running terraform apply twice,
// or wiping state and re-applying, is always safe.
func (r *UserResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan UserStateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.usersClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build users client", err.Error())
		return
	}

	// --- Idempotency check ---
	// Use SCIM filter to find an existing user with the same userName.
	// If found, adopt it into state instead of creating a duplicate.
	filterVal := fmt.Sprintf(`userName eq "%s"`, plan.UserName.ValueString())
	existing, listErr := c.Users.List(ctx, &generated.GetUsersRequest{Filter: &filterVal})
	if listErr == nil && len(existing) > 0 {
		populateStateFromMap(&plan, existing[0])
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// No existing match — create the user.
	userV2 := &generated.UserV2{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		UserName: plan.UserName.ValueString(),
	}

	givenName := plan.GivenName.ValueString()
	familyName := plan.FamilyName.ValueString()
	if givenName != "" || familyName != "" {
		userV2.Name = &generated.Name{}
		if givenName != "" {
			userV2.Name.GivenName = strPtr(givenName)
		}
		if familyName != "" {
			userV2.Name.FamilyName = strPtr(familyName)
		}
	}

	if email := plan.Email.ValueString(); email != "" {
		userV2.Emails = []*generated.EmailAddress{
			{Value: email, Type: "work"},
		}
	}

	if pw := plan.Password.ValueString(); pw != "" {
		userV2.Password = strPtr(pw)
	}

	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		active := plan.Active.ValueBool()
		userV2.Active = &active
	}

	createReq := &generated.CreateUserRequest{Body: userV2}
	m, err := c.Users.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create user in IBM Verify", err.Error())
		return
	}

	populateStateFromMap(&plan, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the user state from IBM Verify.
// If the user was deleted outside Terraform, it is removed from state.
func (r *UserResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state UserStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.usersClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build users client for Read", err.Error())
		return
	}

	m, err := c.Users.Get(ctx, state.UserID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read user from IBM Verify", err.Error())
		return
	}

	populateStateFromMap(&state, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called — all mutable fields on the required set use RequiresReplace,
// and computed fields are refreshed on Read.
func (r *UserResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
}

// Delete removes the user from IBM Verify.
func (r *UserResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state UserStateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.usersClientFor(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build users client for Delete", err.Error())
		return
	}

	if err := c.Users.Delete(ctx, state.UserID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete user from IBM Verify", err.Error())
		return
	}
}

// ImportState brings an existing IBM Verify user under Terraform management.
// The import ID is the user's SCIM id (UUID), e.g.:
//
//	terraform import verify_user.example a1b2c3d4-5678-...
func (r *UserResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	userID := req.ID
	if userID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID must be the user's SCIM id (UUID), e.g.: "+
				"terraform import verify_user.this a1b2c3d4-5678-...",
		)
		return
	}

	if r.usersClient == nil {
		resp.Diagnostics.AddError(
			"Provider not configured for import",
			"sts_client_id / sts_client_secret must be set in the provider block "+
				"before running terraform import on verify_user.",
		)
		return
	}

	m, err := r.usersClient.Users.Get(ctx, userID)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			resp.Diagnostics.AddError(
				"User not found",
				fmt.Sprintf("No user with ID %q exists in IBM Verify. "+
					"Verify the ID and tenant URL in the provider block.", userID),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch user from IBM Verify", err.Error())
		return
	}

	state := UserStateModel{
		TenantURL: types.StringValue(r.usersClient.TenantURL()),
	}
	populateStateFromMap(&state, m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// populateStateFromMap fills a UserStateModel from a raw SCIM response map.
// IBM Verify returns the SCIM user as a flat JSON map — this extracts the
// common fields and handles missing/null values safely.
func populateStateFromMap(state *UserStateModel, m map[string]interface{}) {
	if id, ok := m["id"].(string); ok {
		state.UserID = types.StringValue(id)
	}
	if un, ok := m["userName"].(string); ok {
		state.UserName = types.StringValue(un)
	}
	if dn, ok := m["displayName"].(string); ok {
		state.DisplayName = types.StringValue(dn)
	} else {
		state.DisplayName = types.StringValue("")
	}
	if active, ok := m["active"].(bool); ok {
		state.Active = types.BoolValue(active)
	} else {
		state.Active = types.BoolValue(true)
	}

	// Extract name sub-object.
	if nameMap, ok := m["name"].(map[string]interface{}); ok {
		if gn, ok := nameMap["givenName"].(string); ok {
			state.GivenName = types.StringValue(gn)
		}
		if fn, ok := nameMap["familyName"].(string); ok {
			state.FamilyName = types.StringValue(fn)
		}
	}
	if state.GivenName.IsNull() || state.GivenName.IsUnknown() {
		state.GivenName = types.StringValue("")
	}
	if state.FamilyName.IsNull() || state.FamilyName.IsUnknown() {
		state.FamilyName = types.StringValue("")
	}

	// Extract first work email.
	if emails, ok := m["emails"].([]interface{}); ok {
		for _, e := range emails {
			if em, ok := e.(map[string]interface{}); ok {
				if val, ok := em["value"].(string); ok && val != "" {
					state.Email = types.StringValue(val)
					break
				}
			}
		}
	}
	if state.Email.IsNull() || state.Email.IsUnknown() {
		state.Email = types.StringValue("")
	}
}
