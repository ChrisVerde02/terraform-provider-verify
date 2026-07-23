package datasources

import (
	"crypto/rand"
	"fmt"
)

// generateJTI produces a random 128-bit hex string suitable for use as a
// JWT jti claim. It is called on every data source Read so each generated
// JWT has a unique identifier, preventing IBM Verify from rejecting the
// request with a jti replay error (CSIAQ5206E).
func generateJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand.Read never fails on supported platforms; panic is appropriate.
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%12x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:],
	)
}
