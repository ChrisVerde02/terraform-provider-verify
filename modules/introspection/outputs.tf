output "active" {
  value       = data.verify_token_introspection.this.active
  description = "Whether IBM Verify considers the token currently valid."
}

output "subject" {
  value       = data.verify_token_introspection.this.subject
  description = "IBM Verify internal user ID."
}

output "preferred_username" {
  value       = data.verify_token_introspection.this.preferred_username
  description = "OIDC login name of the user."
}

output "username" {
  value       = data.verify_token_introspection.this.username
  description = "Cloud Directory username."
}

output "name" {
  value       = data.verify_token_introspection.this.name
  description = "Full display name of the user."
}

output "given_name" {
  value       = data.verify_token_introspection.this.given_name
  description = "First name of the user."
}

output "scope" {
  value       = data.verify_token_introspection.this.scope
  description = "Scopes associated with the token."
}

output "expires_at" {
  value       = data.verify_token_introspection.this.expires_at
  description = "Token expiry time as a Unix timestamp."
}
