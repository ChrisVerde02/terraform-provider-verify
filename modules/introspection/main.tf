terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.4.4"
    }
  }
}

# Introspects the access token against IBM Verify.
# This is a data source so it is re-evaluated on every plan and apply —
# the result always reflects the live token status, never cached state.
data "verify_token_introspection" "this" {
  tenant_url    = var.tenant_url
  client_id     = var.client_id
  client_secret = var.client_secret
  token         = var.token
}
