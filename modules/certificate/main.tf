terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.4.5"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

# Generates a new RSA private key and self-signed X.509 certificate.
# This module must only be used in its own Terraform root with its own
# state file — never inside the same root as jwt or token_exchange.
# Reason: every terraform destroy would regenerate the key pair, which
# would break IBM Verify's trust relationship until the new cert.pem is
# re-uploaded to the STS client.
resource "verify_certificate" "this" {
  common_name   = var.common_name
  organization  = var.organization
  country       = var.country
  validity_days = var.validity_days
  key_size      = var.key_size
}

# Write the public certificate to disk so it can be uploaded to IBM Verify.
# The output directory should be excluded from git via .gitignore.
resource "local_file" "certificate" {
  content         = verify_certificate.this.certificate_pem
  filename        = var.certificate_output_path
  file_permission = "0644"
}

# Write the private key to disk so the jwt module can read it with file().
# This file must never be committed — exclude it via .gitignore.
resource "local_sensitive_file" "private_key" {
  content         = verify_certificate.this.private_key_pem
  filename        = var.private_key_output_path
  file_permission = "0600"
}
