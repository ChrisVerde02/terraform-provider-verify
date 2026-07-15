package crypto

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTRequest contains the values needed to create a signed JWT.
type JWTRequest struct {
	Issuer        string
	Subject       string
	KeyID         string
	JWTID         string
	PrivateKeyPEM string
	ExpiresIn     time.Duration
}

// JWTResult contains the generated JWT and timestamps.
type JWTResult struct {
	Token     string
	IssuedAt  int64
	ExpiresAt int64
}

// GenerateSignedJWT creates an RS256-signed JWT.
func GenerateSignedJWT(request JWTRequest) (*JWTResult, error) {
	if request.Issuer == "" {
		return nil, errors.New("issuer cannot be empty")
	}

	if request.Subject == "" {
		return nil, errors.New("subject cannot be empty")
	}

	if request.KeyID == "" {
		return nil, errors.New("key ID cannot be empty")
	}

	if request.JWTID == "" {
		return nil, errors.New("JWT ID cannot be empty")
	}

	if request.PrivateKeyPEM == "" {
		return nil, errors.New("private key cannot be empty")
	}

	if request.ExpiresIn <= 0 {
		return nil, errors.New("expiration duration must be greater than zero")
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(
		[]byte(request.PrivateKeyPEM),
	)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(request.ExpiresIn)

	claims := jwt.MapClaims{
		"iss": request.Issuer,
		"sub": request.Subject,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
		"jti": request.JWTID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	token.Header["kid"] = request.KeyID

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign JWT: %w", err)
	}

	return &JWTResult{
		Token:     signedToken,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}, nil
}
