terraform {
  required_providers {
    verify = {
      source = "christian-verderame/verify"
    }

    local = {
      source = "hashicorp/local"
    }
  }
}

provider "verify" {}

# -------------------------------------------------------------------
# verify_certificate is in its own Terraform root so it has its own
# state file and is never touched by the main examples/ apply.
# Run this once:
#
#   cd examples/cert
#   terraform init && terraform apply -auto-approve
#
# Then upload cert.pem to your IBM Verify STS client, and from that
# point on only run terraform apply in examples/ — the key pair here
# will never be regenerated unless you explicitly destroy this root.
# -------------------------------------------------------------------
resource "verify_certificate" "this" {
  common_name   = "DemoTokenSigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}

# Write the public certificate to disk so it can be uploaded to
# IBM Verify and so the main config can reference its path.
resource "local_file" "certificate" {
  content         = verify_certificate.this.certificate_pem
  filename        = "${path.module}/../cert.pem"
  file_permission = "0644"
}

# Write the private key to disk so the main config can read it with
# file() — it stays here permanently and is never regenerated.
resource "local_sensitive_file" "private_key" {
  content         = verify_certificate.this.private_key_pem
  filename        = "${path.module}/../key.pem"
  file_permission = "0600"
}

output "certificate_pem" {
  value       = verify_certificate.this.certificate_pem
  description = "Upload this certificate to IBM Verify STS client."
}

output "certificate_path" {
  value       = local_file.certificate.filename
  description = "Path to cert.pem on disk."
}

output "private_key_path" {
  value       = local_sensitive_file.private_key.filename
  description = "Path to key.pem on disk."
  sensitive   = true
}
