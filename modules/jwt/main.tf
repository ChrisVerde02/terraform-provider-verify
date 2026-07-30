terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.3.8"
    }
  }
}

# Signs an RS256 JWT and stores it in Terraform state.
# Because this is a resource with a smart Read(), the JWT is reused across
# plans until it expires, then automatically regenerated with a fresh jti.
resource "verify_jwt" "this" {
  issuer             = var.issuer
  subject            = var.subject
  key_id             = var.key_id
  private_key_pem    = var.private_key_pem
  expires_in_seconds = var.expires_in_seconds
}
