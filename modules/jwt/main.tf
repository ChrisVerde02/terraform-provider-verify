terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "~> 0.1"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}

# Generates a new UUID whenever any JWT input changes.
# Prevents IBM Verify from rejecting the request with a jti replay error
# (CSIAQ5206E) when the subject or other claims are updated.
resource "random_uuid" "jwt_id" {
  keepers = {
    issuer  = var.issuer
    subject = var.subject
    key_id  = var.key_id
  }
}

# Signs a fresh RS256 JWT using the stable private key.
# The JWT is short-lived (default 15 min) but the key pair never rotates,
# so IBM Verify can always verify the signature with the uploaded certificate.
resource "verify_jwt" "this" {
  issuer             = var.issuer
  subject            = var.subject
  key_id             = var.key_id
  jwt_id             = random_uuid.jwt_id.result
  private_key_pem    = var.private_key_pem
  expires_in_seconds = var.expires_in_seconds
}
