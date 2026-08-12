package datasources

import (
	providercrypto "github.com/ChrisVerde02/ibmverify-go/crypto"
)

// generateJTI produces a random UUID-shaped string for use as a JWT jti
// claim. Returns an error instead of panicking if the system random source
// fails — the caller surfaces it via AddError.
func generateJTI() (string, error) {
	return providercrypto.GenerateJTI()
}
