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

// TokenExchangeRequest contains the values sent to IBM Verify.
type TokenExchangeRequest struct {
	TenantURL        string
	ClientID         string
	ClientSecret     string
	SubjectToken     string
	SubjectTokenType string
}

// TokenExchangeResponse represents the response from IBM Verify.
type TokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	ExpiresIn       int64  `json:"expires_in"`
	GrantID         string `json:"grant_id"`
	IssuedTokenType string `json:"issued_token_type"`
	Scope           string `json:"scope"`
	TokenType       string `json:"token_type"`
}

// TokenExchangeError represents an OAuth error response.
type TokenExchangeError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// ExchangeToken sends a JWT to IBM Verify and returns an access token.
func ExchangeToken(
	ctx context.Context,
	request TokenExchangeRequest,
) (*TokenExchangeResponse, error) {
	if strings.TrimSpace(request.TenantURL) == "" {
		return nil, errors.New("tenant URL cannot be empty")
	}

	if strings.TrimSpace(request.ClientID) == "" {
		return nil, errors.New("client ID cannot be empty")
	}

	if strings.TrimSpace(request.ClientSecret) == "" {
		return nil, errors.New("client secret cannot be empty")
	}

	if strings.TrimSpace(request.SubjectToken) == "" {
		return nil, errors.New("subject token cannot be empty")
	}

	if strings.TrimSpace(request.SubjectTokenType) == "" {
		return nil, errors.New("subject token type cannot be empty")
	}

	tokenEndpoint := strings.TrimRight(request.TenantURL, "/") + "/oauth2/token"

	form := url.Values{}
	form.Set("client_id", request.ClientID)
	form.Set("client_secret", request.ClientSecret)
	form.Set(
		"grant_type",
		"urn:ietf:params:oauth:grant-type:token-exchange",
	)
	form.Set("subject_token", request.SubjectToken)
	form.Set("subject_token_type", request.SubjectTokenType)

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenEndpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token exchange request: %w", err)
	}

	httpRequest.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	httpRequest.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("send token exchange request: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("read token exchange response: %w", err)
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		var oauthError TokenExchangeError

		if err := json.Unmarshal(responseBody, &oauthError); err == nil &&
			oauthError.Error != "" {
			return nil, fmt.Errorf(
				"IBM Verify token exchange failed with HTTP %d: %s: %s",
				httpResponse.StatusCode,
				oauthError.Error,
				oauthError.ErrorDescription,
			)
		}

		return nil, fmt.Errorf(
			"IBM Verify token exchange failed with HTTP %d: %s",
			httpResponse.StatusCode,
			string(responseBody),
		)
	}

	var tokenResponse TokenExchangeResponse

	if err := json.Unmarshal(responseBody, &tokenResponse); err != nil {
		return nil, fmt.Errorf(
			"decode token exchange response: %w",
			err,
		)
	}

	if tokenResponse.AccessToken == "" {
		return nil, errors.New(
			"IBM Verify response did not contain an access_token",
		)
	}

	return &tokenResponse, nil
}
