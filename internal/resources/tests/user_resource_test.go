package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	generated "github.com/ChrisVerde02/ibmverify-go/generated"
)

const testUserID = "u1b2c3d4-0000-0000-0000-000000000001"

// ---------------------------------------------------------------------------
// Users.Create — httptest matrix
// ---------------------------------------------------------------------------

func TestUsersCreate_201_setsUserID(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/scim+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"id":          testUserID,
			"userName":    "jdoe",
			"displayName": "John Doe",
			"active":      true,
			"name": map[string]interface{}{
				"givenName":  "John",
				"familyName": "Doe",
			},
			"emails": []interface{}{
				map[string]interface{}{"value": "jdoe@example.com", "type": "work"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	req := &generated.CreateUserRequest{Body: &generated.UserV2{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		UserName: "jdoe",
	}}
	m, err := c.Users.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if m["id"] != testUserID {
		t.Errorf("id: want %q, got %q", testUserID, m["id"])
	}
	if m["userName"] != "jdoe" {
		t.Errorf("userName: want %q, got %q", "jdoe", m["userName"])
	}
}

func TestUsersCreate_400_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/scim+json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"detail": "userName already exists",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	req := &generated.CreateUserRequest{Body: &generated.UserV2{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		UserName: "duplicate-user",
	}}
	_, err := c.Users.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for HTTP 400, got nil")
	}
}

// ---------------------------------------------------------------------------
// Users.Get — httptest matrix
// ---------------------------------------------------------------------------

func TestUsersGet_200_found(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users/"+testUserID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/scim+json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"id":       testUserID,
			"userName": "jdoe",
			"active":   true,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	m, err := c.Users.Get(context.Background(), testUserID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if m["id"] != testUserID {
		t.Errorf("id: want %q, got %q", testUserID, m["id"])
	}
}

func TestUsersGet_404_returnsHTTP404Error(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users/missing-user", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Users.Get(context.Background(), "missing-user")
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected error to contain %q, got: %v", "HTTP 404", err)
	}
}

// ---------------------------------------------------------------------------
// Users.Delete — httptest matrix
// ---------------------------------------------------------------------------

func TestUsersDelete_204_ok(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users/"+testUserID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Users.Delete(context.Background(), testUserID); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
}

func TestUsersDelete_404_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users/missing-user", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Users.Delete(context.Background(), "missing-user")
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

// ---------------------------------------------------------------------------
// populateStateFromMap — unit tests (no HTTP needed)
// ---------------------------------------------------------------------------

func TestUsersGet_activeField_boolHandling(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v2.0/Users/"+testUserID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/scim+json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"id":       testUserID,
			"userName": "jdoe",
			"active":   false, // explicitly inactive
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	m, err := c.Users.Get(context.Background(), testUserID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	// active comes through as bool via JSON unmarshal into interface{}
	active, ok := m["active"].(bool)
	if !ok {
		t.Fatalf("expected active to be bool, got %T", m["active"])
	}
	if active {
		t.Error("expected active=false, got true")
	}
}
