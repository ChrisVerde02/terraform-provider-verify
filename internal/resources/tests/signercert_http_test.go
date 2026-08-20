// Package tests contains black-box tests for the resources package.
// These tests use net/http/httptest to exercise the SDK against a mock IBM Verify
// server — no live tenant or credentials required.
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	verifyclient "github.com/ChrisVerde02/ibmverify-go/client"
)

// newTestClient builds a verifyclient.Client pointed at the given httptest server.
func newTestClient(t *testing.T, srv *httptest.Server) *verifyclient.Client {
	t.Helper()
	c, err := verifyclient.New(
		srv.URL,
		verifyclient.WithClientCredentials("test-id", "test-secret"),
		verifyclient.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	return c
}

// registerTokenEndpoint adds a client-credentials handler that returns a
// minimal valid token so downstream cert/token calls can proceed.
func registerTokenEndpoint(mux *http.ServeMux) {
	mux.HandleFunc("/v1.0/endpoint/default/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"access_token": "mock-bearer-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
			"scope":        "",
		})
	})
}

// ---------------------------------------------------------------------------
// CertsClient.Import — httptest matrix
// ---------------------------------------------------------------------------

func TestCertsImport_201_created(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Certs.Import(context.Background(), "demotokensigner", realCertPEM); err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}
}

func TestCertsImport_500_returnsRetryableAPIError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error":             "server_error",
			"error_description": "internal failure",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Certs.Import(context.Background(), "demotokensigner", realCertPEM)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode: want 500, got %d", apiErr.StatusCode)
	}
	if !apiErr.IsServer() {
		t.Error("IsServer() should be true for 500")
	}
	if !apiErr.IsRetryable() {
		t.Error("IsRetryable() should be true for 500")
	}
}

func TestCertsImport_429_returnsRateLimitError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Certs.Import(context.Background(), "demotokensigner", realCertPEM)
	if err == nil {
		t.Fatal("expected error for HTTP 429, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsRateLimit() {
		t.Error("IsRateLimit() should be true for 429")
	}
}

func TestCertsImport_401_returnsAuthError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error":             "invalid_client",
			"error_description": "client authentication failed",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Certs.Import(context.Background(), "demotokensigner", realCertPEM)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsAuth() {
		t.Errorf("IsAuth() should be true for 401 invalid_client, code=%q", apiErr.Code)
	}
}

// ---------------------------------------------------------------------------
// CertsClient.Get — httptest matrix
// ---------------------------------------------------------------------------

func TestCertsGet_200_found(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert/demotokensigner", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"label": "demotokensigner",
			"cert":  realCertPEM,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	result, err := c.Certs.Get(context.Background(), "demotokensigner")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if result.Label != "demotokensigner" {
		t.Errorf("Label: want demotokensigner, got %q", result.Label)
	}
}

func TestCertsGet_404_returnsErrNotFound(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	result, err := c.Certs.Get(context.Background(), "missing")
	if result != nil {
		t.Error("expected nil result for 404")
	}
	if !errors.Is(err, verifyclient.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CertsClient.Delete — httptest matrix
// ---------------------------------------------------------------------------

func TestCertsDelete_204_ok(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert/demotokensigner", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Certs.Delete(context.Background(), "demotokensigner"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
}

func TestCertsDelete_500_returnsError(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	mux.HandleFunc("/v1.0/signercert/demotokensigner", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.Certs.Delete(context.Background(), "demotokensigner")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsServer() {
		t.Error("IsServer() should be true for 500")
	}
}

// ---------------------------------------------------------------------------
// Path-traversal — label validation rejects before any HTTP call
// ---------------------------------------------------------------------------

func TestCertsGet_traversalLabel_rejectedClientSide(t *testing.T) {
	httpCallMade := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpCallMade = true
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Certs.Get(context.Background(), "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal label, got nil")
	}
	if httpCallMade {
		t.Error("SDK should reject traversal labels before any HTTP call")
	}
}

// ---------------------------------------------------------------------------
// Token.Exchange — httptest matrix
// ---------------------------------------------------------------------------

func TestTokenExchange_200_ok(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"access_token":      "exchanged-token",
			"expires_in":        7200,
			"grant_id":          "grant-abc",
			"issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
			"scope":             "openid",
			"token_type":        "Bearer",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	result, err := c.Token.Exchange(context.Background(),
		"subject.jwt.here",
		"urn:ietf:params:oauth:token-type:jwt",
	)
	if err != nil {
		t.Fatalf("Exchange: unexpected error: %v", err)
	}
	if result.AccessToken != "exchanged-token" {
		t.Errorf("AccessToken: want exchanged-token, got %q", result.AccessToken)
	}
	if result.ExpiresIn != 7200 {
		t.Errorf("ExpiresIn: want 7200, got %d", result.ExpiresIn)
	}
}

func TestTokenExchange_404_returnsTypedNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Token.Exchange(context.Background(), "jwt", "urn:ietf:params:oauth:token-type:jwt")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("IsNotFound() should be true, StatusCode=%d", apiErr.StatusCode)
	}
}

func TestTokenExchange_400_CSIAQ5212E_matchedByCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"messageId":          "CSIAQ5212E",
			"messageDescription": "Unable to verify the integrity of the token.",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Token.Exchange(context.Background(), "jwt", "urn:ietf:params:oauth:token-type:jwt")
	if err == nil {
		t.Fatal("expected error for CSIAQ5212E, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "CSIAQ5212E" {
		t.Errorf("Code: want CSIAQ5212E, got %q", apiErr.Code)
	}
}

// ---------------------------------------------------------------------------
// Token.ClientCredentials — httptest matrix
// ---------------------------------------------------------------------------

func TestClientCredentials_200_ok(t *testing.T) {
	mux := http.NewServeMux()
	registerTokenEndpoint(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	result, err := c.Token.ClientCredentials(context.Background())
	if err != nil {
		t.Fatalf("ClientCredentials: unexpected error: %v", err)
	}
	if result.AccessToken != "mock-bearer-token" {
		t.Errorf("AccessToken: want mock-bearer-token, got %q", result.AccessToken)
	}
}

func TestClientCredentials_401_returnsAuthError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/endpoint/default/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error":             "invalid_client",
			"error_description": "OIDC client authentication failed",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Token.ClientCredentials(context.Background())
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	var apiErr *verifyclient.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsAuth() {
		t.Errorf("IsAuth() should be true for invalid_client, code=%q", apiErr.Code)
	}
}
