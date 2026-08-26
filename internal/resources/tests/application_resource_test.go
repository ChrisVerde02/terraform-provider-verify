package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Apps.Create — httptest matrix
// ---------------------------------------------------------------------------

func TestAppsCreate_201_setsApplicationID(t *testing.T) {
	const wantID = "a1b2c3d4-0000-0000-0000-000000000001"

	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"_links": map[string]interface{}{
				"self": map[string]interface{}{
					"href": "/v1.0/applications/" + wantID,
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	result, err := c.Apps.Create(context.Background(), nil)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if result.GetLinks() == nil || result.GetLinks().GetSelf() == nil {
		t.Fatal("expected non-nil _links.self in response")
	}

	href := result.GetLinks().GetSelf().GetHref()
	href = strings.TrimRight(href, "/")
	gotID := href[strings.LastIndex(href, "/")+1:]
	if gotID != wantID {
		t.Errorf("extracted application ID: want %q, got %q", wantID, gotID)
	}
}

func TestAppsCreate_500_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Apps.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

// ---------------------------------------------------------------------------
// Apps.Get — httptest matrix
// ---------------------------------------------------------------------------

func TestAppsGet_200_found(t *testing.T) {
	const appID = "a1b2c3d4-0000-0000-0000-000000000001"

	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications/"+appID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"id":               appID,
			"name":             "My Test App",
			"templateId":       "tmpl-001",
			"applicationState": true,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	m, err := c.Apps.Get(context.Background(), appID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if m["name"] != "My Test App" {
		t.Errorf("name: want %q, got %q", "My Test App", m["name"])
	}
	// applicationState is a bool in IBM's response — comes through as interface{}.
	// fmt.Sprintf("%v", m["applicationState"]) must yield "true".
	stateStr := func() string {
		if v := m["applicationState"]; v != nil {
			return func(v interface{}) string {
				if b, ok := v.(bool); ok {
					if b {
						return "true"
					}
					return "false"
				}
				return ""
			}(v)
		}
		return ""
	}()
	if stateStr != "true" {
		t.Errorf("applicationState: want %q, got %q", "true", stateStr)
	}
}

func TestAppsGet_404_returnsHTTP404Error(t *testing.T) {
	const appID = "missing-app-id"

	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications/"+appID, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Apps.Get(context.Background(), appID)
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected error to contain %q, got: %v", "HTTP 404", err)
	}
}

// ---------------------------------------------------------------------------
// Apps.Delete — httptest matrix
// ---------------------------------------------------------------------------

func TestAppsDelete_204_ok(t *testing.T) {
	const appID = "a1b2c3d4-0000-0000-0000-000000000001"

	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications/"+appID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Apps.Delete(context.Background(), appID); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
}

func TestAppsDelete_500_returnsError(t *testing.T) {
	const appID = "a1b2c3d4-0000-0000-0000-000000000001"

	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/applications/"+appID, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Apps.Delete(context.Background(), appID)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}
