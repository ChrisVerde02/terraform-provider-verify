# token is passed to the token_exchange module as the subject_token.
output "token" {
  value       = data.verify_jwt.this.token
  description = "Signed RS256 JWT."
  sensitive   = true
}

output "issued_at" {
  value       = data.verify_jwt.this.issued_at
  description = "JWT issue time as a Unix timestamp."
}

output "expires_at" {
  value       = data.verify_jwt.this.expires_at
  description = "JWT expiry time as a Unix timestamp."
}
