terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.4.5"
    }
  }
}

# Uploads a signer certificate to IBM Verify using the cert-manager API client.
# The resource obtains its own client credentials token internally — no
# access_token is needed in configuration.
resource "verify_signercert" "this" {
  tenant_url                 = var.tenant_url
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
  certificate_pem            = var.certificate_pem
  label                      = var.label
}
