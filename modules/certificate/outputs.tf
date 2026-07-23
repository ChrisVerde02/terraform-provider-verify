# certificate_pem should be uploaded to IBM Verify under
# Security → API clients → your STS client → JWT validation / Certificate.
output "certificate_pem" {
  value       = verify_certificate.this.certificate_pem
  description = "PEM-encoded X.509 certificate — upload this to IBM Verify."
}

# certificate_path is the location of cert.pem on disk.
output "certificate_path" {
  value       = local_file.certificate.filename
  description = "Path to cert.pem on disk."
}

# private_key_path is passed to the jwt module as private_key_pem via file().
output "private_key_path" {
  value       = local_sensitive_file.private_key.filename
  description = "Path to key.pem on disk — never commit this file."
  sensitive   = true
}

# private_key_pem can be passed directly to the jwt module instead of
# using file() — useful when both modules run in the same root.
output "private_key_pem" {
  value       = verify_certificate.this.private_key_pem
  description = "PEM-encoded RSA private key — sensitive."
  sensitive   = true
}
