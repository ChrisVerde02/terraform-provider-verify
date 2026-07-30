terraform {
  required_providers {
    verify = {
      # Uses the published provider from the Terraform Registry.
      # No go build, no ~/.terraformrc override needed.
      source  = "ChrisVerde02/verify"
      version = "0.3.3"
    }
  }
}

provider "verify" {}

# ---------------------------------------------------------------
# Module: jwt
# Signs an RS256 JWT using the stable private key on disk.
# Reused until expired, then automatically regenerated.
# ---------------------------------------------------------------
module "jwt" {
  source = "../modules/jwt"

  issuer          = var.jwt_issuer
  subject         = var.jwt_subject
  key_id          = var.jwt_key_id
  private_key_pem = file(var.private_key_path)
}

# ---------------------------------------------------------------
# Module: token_exchange
# Exchanges the JWT for an IBM Verify access token.
# Reused until expired, then automatically re-exchanged.
# ---------------------------------------------------------------
module "token_exchange" {
  source = "../modules/token_exchange"

  tenant_url         = var.verify_tenant_url
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = module.jwt.token
  subject_token_type = var.subject_token_type
}

# ---------------------------------------------------------------
# Module: introspection
# Verifies the access token is active — re-evaluated every apply.
# ---------------------------------------------------------------
module "introspection" {
  source = "../modules/introspection"

  tenant_url    = var.verify_tenant_url
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = module.token_exchange.access_token
}

# ---------------------------------------------------------------
# Variables
# ---------------------------------------------------------------

variable "verify_tenant_url" {
  type        = string
  description = "IBM Verify tenant base URL."
}

variable "sts_client_id" {
  type        = string
  description = "IBM Verify STS client ID."
}

variable "sts_client_secret" {
  type        = string
  description = "IBM Verify STS client secret."
  sensitive   = true
}

variable "private_key_path" {
  type        = string
  description = "Path to the RSA private key PEM file on disk."
  default     = "../examples/certificates/key.pem"
}

variable "jwt_issuer" {
  type        = string
  description = "Value placed in the JWT iss claim."
}

variable "jwt_subject" {
  type        = string
  description = "IBM Verify Cloud Directory username."
}

variable "jwt_key_id" {
  type        = string
  description = "Certificate label in IBM Verify."
}

variable "subject_token_type" {
  type        = string
  description = "Token type URN configured in your IBM Verify STS client."
  default     = "urn:demo:token-type:user-jwt"
}

# ---------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------

output "access_token" {
  value     = module.token_exchange.access_token
  sensitive = true
}

output "access_token_expires_in" {
  value = module.token_exchange.expires_in
}

output "grant_id" {
  value = module.token_exchange.grant_id
}

output "issued_token_type" {
  value = module.token_exchange.issued_token_type
}

output "token_type" {
  value = module.token_exchange.token_type
}

output "token_active" {
  value = module.introspection.active
}

output "introspected_subject" {
  value = module.introspection.subject
}

output "introspected_preferred_username" {
  value = module.introspection.preferred_username
}

output "introspected_expires_at" {
  value = module.introspection.expires_at
}
