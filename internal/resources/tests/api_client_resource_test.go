package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	generated "github.com/ChrisVerde02/ibmverify-go/generated"
	"github.com/hashicorp/terraform-plugin-framework/types"

	resources "github.com/Christian-Verderame/terraform-provider-verify/internal/resources"
)

const testAPIClientID = "c1b2c3d4-0000-0000-0000-000000000001"

// ---------------------------------------------------------------------------
// APIClients.Create — httptest matrix
// ---------------------------------------------------------------------------

func TestAPIClientsCreate_201_location_setsClientID(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)

	// POST /v1.0/apiclients → 201 + Location header (empty body — real IBM behaviour)
	mux.HandleFunc("/v1.0/apiclients", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Location", "/v1.0/apiclients/"+testAPIClientID)
		w.WriteHeader(http.StatusCreated)
	})

	// GET /v1.0/apiclients/<id> — follow-up after 201 (SDK does this internally)
	mux.HandleFunc("/v1.0/apiclients/"+testAPIClientID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET follow-up, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"clientId":     testAPIClientID,
			"clientName":   "my-tf-client",
			"clientSecret": "super-secret-value",
			"enabled":      true,
			"entitlements": []string{"manageCerts"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	req := &generated.APIClientConfigRequest{
		ClientName:   "my-tf-client",
		Entitlements: []string{"manageCerts"},
		Enabled:      true,
	}
	m, err := c.APIClients.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if m["clientId"] != testAPIClientID {
		t.Errorf("clientId: want %q, got %q", testAPIClientID, m["clientId"])
	}
	if m["clientSecret"] != "super-secret-value" {
		t.Errorf("clientSecret: want %q, got %q", "super-secret-value", m["clientSecret"])
	}
}

func TestAPIClientsCreate_400_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/apiclients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error": "clientName already exists",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.APIClients.Create(context.Background(), &generated.APIClientConfigRequest{
		ClientName:   "duplicate",
		Entitlements: []string{},
	})
	if err == nil {
		t.Fatal("expected error for HTTP 400, got nil")
	}
}

// ---------------------------------------------------------------------------
// APIClients.Get — httptest matrix
// ---------------------------------------------------------------------------

func TestAPIClientsGet_200_found(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/apiclients/"+testAPIClientID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"clientId":     testAPIClientID,
			"clientName":   "my-tf-client",
			"enabled":      true,
			"entitlements": []string{"manageCerts"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	m, err := c.APIClients.Get(context.Background(), testAPIClientID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if m["clientId"] != testAPIClientID {
		t.Errorf("clientId: want %q, got %q", testAPIClientID, m["clientId"])
	}
}

func TestAPIClientsGet_404_returnsHTTP404Error(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/apiclients/missing-client", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.APIClients.Get(context.Background(), "missing-client")
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected error to contain %q, got: %v", "HTTP 404", err)
	}
}

// ---------------------------------------------------------------------------
// APIClients.Delete — httptest matrix
// ---------------------------------------------------------------------------

func TestAPIClientsDelete_204_ok(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/apiclients/"+testAPIClientID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.APIClients.Delete(context.Background(), testAPIClientID); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
}

func TestAPIClientsDelete_404_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/apiclients/missing-client", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.APIClients.Delete(context.Background(), "missing-client")
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

// ---------------------------------------------------------------------------
// apiClientStateFromMap — adoption path: secret not in map → empty string, not unknown
// ---------------------------------------------------------------------------

func TestAPIClientStateFromMap_adoptionPath_secretIsEmptyNotUnknown(t *testing.T) {
	// Simulate a GET response (no clientSecret key) — happens during adoption and Read.
	m := map[string]interface{}{
		"clientId":     testAPIClientID,
		"clientName":   "my-tf-client",
		"enabled":      true,
		"entitlements": []interface{}{"manageCerts"},
	}

	state := resources.APIClientStateModel{
		// ClientSecret starts as unknown (zero value) — same as adoption path.
		ClientSecret: types.StringUnknown(),
	}
	resources.APIClientStateFromMap(context.Background(), &state, m)

	if state.ClientSecret.IsNull() {
		t.Error("ClientSecret must not be null after adoption — Terraform requires all Computed fields to be known")
	}
	if state.ClientSecret.IsUnknown() {
		t.Error("ClientSecret must not be unknown after adoption — Terraform requires all Computed fields to be known after apply")
	}
	// Secret should be empty string (not the real secret, which IBM never returns on GET).
	if got := state.ClientSecret.ValueString(); got != "" {
		t.Errorf("ClientSecret: want empty string on adoption, got %q", got)
	}
}

func TestAPIClientStateFromMap_adoptionPath_preservesExistingSecret(t *testing.T) {
	// Simulate Read refreshing state when an existing secret is already stored.
	m := map[string]interface{}{
		"clientId":   testAPIClientID,
		"clientName": "my-tf-client",
		"enabled":    true,
	}

	state := resources.APIClientStateModel{
		// Pre-existing secret from initial Create.
		ClientSecret: types.StringValue("previously-stored-secret"),
	}
	resources.APIClientStateFromMap(context.Background(), &state, m)

	if got := state.ClientSecret.ValueString(); got != "previously-stored-secret" {
		t.Errorf("ClientSecret: want %q preserved, got %q", "previously-stored-secret", got)
	}
}
