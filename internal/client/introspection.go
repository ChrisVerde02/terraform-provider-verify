package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// IntrospectionRequest contains the values sent to IBM Verify.
type IntrospectionRequest struct {
	TenantURL    string
	ClientID     string
	ClientSecret string
	Token        string
}

// IntrospectionResponse represents token metadata returned by IBM Verify.
type IntrospectionResponse struct {
	Active            bool   `json:"active"`
	ClientID          string `json:"client_id"`
	Username          string `json:"username"`           // IBM Verify Cloud Directory username (e.g. "Bretton")
	PreferredUsername string `json:"preferred_username"` // OIDC preferred_username claim
	Name              string `json:"name"`               // full display name (e.g. "Jessica")
	GivenName         string `json:"given_name"`         // first name
	Subject           string `json:"sub"`
	Scope             string `json:"scope"`
	TokenType         string `json:"token_type"`
	Issuer            string `json:"iss"`
	IssuedAt          int64  `json:"iat"`
	ExpiresAt         int64  `json:"exp"`
}

// IntrospectionError represents an OAuth error response.
type IntrospectionError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// IntrospectToken asks IBM Verify for metadata about an access token.
func IntrospectToken(
	ctx context.Context,
	request IntrospectionRequest,
) (*IntrospectionResponse, error) {
	if strings.TrimSpace(request.TenantURL) == "" {
		return nil, errors.New("tenant URL cannot be empty")
	}

	if strings.TrimSpace(request.ClientID) == "" {
		return nil, errors.New("client ID cannot be empty")
	}

	if strings.TrimSpace(request.ClientSecret) == "" {
		return nil, errors.New("client secret cannot be empty")
	}

	if strings.TrimSpace(request.Token) == "" {
		return nil, errors.New("token cannot be empty")
	}

	endpoint := strings.TrimRight(request.TenantURL, "/") +
		"/oauth2/introspect"

	form := url.Values{}
	form.Set("client_id", request.ClientID)
	form.Set("client_secret", request.ClientSecret)
	form.Set("token", request.Token)
	form.Set("token_type_hint", "access_token")

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create token introspection request: %w",
			err,
		)
	}

	httpRequest.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf(
			"send token introspection request: %w",
			err,
		)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"read token introspection response: %w",
			err,
		)
	}

	if httpResponse.StatusCode < 200 ||
		httpResponse.StatusCode >= 300 {
		var oauthError IntrospectionError

		if err := json.Unmarshal(
			responseBody,
			&oauthError,
		); err == nil && oauthError.Error != "" {
			return nil, fmt.Errorf(
				"IBM Verify introspection failed with HTTP %d: %s: %s",
				httpResponse.StatusCode,
				oauthError.Error,
				oauthError.ErrorDescription,
			)
		}

		return nil, fmt.Errorf(
			"IBM Verify introspection failed with HTTP %d: %s",
			httpResponse.StatusCode,
			string(responseBody),
		)
	}

	var result IntrospectionResponse

	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf(
			"decode introspection response: %w",
			err,
		)
	}

	return &result, nil
}
