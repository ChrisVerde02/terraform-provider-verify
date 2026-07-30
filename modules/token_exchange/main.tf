terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.3.4"
    }
  }
}

# Exchanges the signed JWT for an IBM Verify access token using
# OAuth 2.0 Token Exchange (RFC 8693).
# Because this is a resource with a smart Read(), the access token is reused
# across plans until it expires, then automatically re-exchanged.
resource "verify_token_exchange" "this" {
  tenant_url         = var.tenant_url
  client_id          = var.client_id
  client_secret      = var.client_secret
  subject_token      = var.subject_token
  subject_token_type = var.subject_token_type
}
