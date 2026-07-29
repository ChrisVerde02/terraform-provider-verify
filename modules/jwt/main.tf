terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "~> 0.3"
    }
  }
}

# Signs a fresh RS256 JWT on every plan and apply.
# Because this is a data source, the jti claim is automatically unique on
# each run — no random_uuid keeper is needed.
data "verify_jwt" "this" {
  issuer             = var.issuer
  subject            = var.subject
  key_id             = var.key_id
  private_key_pem    = var.private_key_pem
  expires_in_seconds = var.expires_in_seconds
}
