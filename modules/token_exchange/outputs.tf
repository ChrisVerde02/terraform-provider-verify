# access_token is passed to the introspection module.
output "access_token" {
  value       = verify_token_exchange.this.access_token
  description = "IBM Verify access token."
  sensitive   = true
}

output "expires_in" {
  value       = verify_token_exchange.this.expires_in
  description = "Access token lifetime in seconds."
}

output "grant_id" {
  value       = verify_token_exchange.this.grant_id
  description = "IBM Verify grant identifier."
}

output "issued_token_type" {
  value       = verify_token_exchange.this.issued_token_type
  description = "Type of token issued by IBM Verify."
}

output "token_type" {
  value       = verify_token_exchange.this.token_type
  description = "Access token type — normally bearer."
}

output "scope" {
  value       = verify_token_exchange.this.scope
  description = "Scopes granted with the access token."
}
