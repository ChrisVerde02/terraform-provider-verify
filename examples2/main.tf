terraform {
  required_providers {
    verify = {
      # Uses the published provider from the Terraform Registry.
      # No go build, no ~/.terraformrc override needed.
      source  = "ChrisVerde02/verify"
      version = "0.3.7"
    }
  }
}

provider "verify" {}

# ---------------------------------------------------------------
# Module: certificate
# Generates an RSA key pair and self-signed X.509 certificate.
# Stored in Terraform state — reused until expired (validity_days),
# then automatically regenerated.
# ---------------------------------------------------------------
module "certificate" {
  source = "../modules/certificate"

  common_name   = var.cert_common_name
  organization  = var.cert_organization
  country       = var.cert_country
  validity_days = var.cert_validity_days
  key_size      = var.cert_key_size

  certificate_output_path = var.certificate_output_path
  private_key_output_path = var.private_key_output_path
}

# ---------------------------------------------------------------
# Module: token_exchange (bootstrap)
# A first token exchange using the existing stable credentials
# is needed to get an access token so we can upload the cert.
# This uses file() for the private key so it doesn't depend on
# the certificate module — avoiding a circular dependency.
# ---------------------------------------------------------------
module "jwt_bootstrap" {
  source = "../modules/jwt"

  issuer          = var.jwt_issuer
  subject         = var.jwt_subject
  key_id          = var.jwt_key_id
  private_key_pem = file(var.private_key_path)
}

module "token_exchange_bootstrap" {
  source = "../modules/token_exchange"

  tenant_url         = var.verify_tenant_url
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = module.jwt_bootstrap.token
  subject_token_type = var.subject_token_type
}

# ---------------------------------------------------------------
# Resource: signercert
# Uploads the certificate from the certificate module to IBM Verify.
# IBM Verify uses this to validate JWT signatures during token exchange.
# The access_token comes from the cert-manager API client (client
# credentials grant) which has manageCerts entitlement directly —
# not via user impersonation, so no 403 permission intersection issue.
# ---------------------------------------------------------------
resource "verify_signercert" "this" {
  tenant_url      = var.verify_tenant_url
  access_token    = data.verify_client_credentials_token.cert_manager.access_token
  certificate_pem = module.certificate.certificate_pem
  label           = var.jwt_key_id
}

# ---------------------------------------------------------------
# Data source: cert_manager_token
# Obtains a fresh access token for the cert-manager API client on
# every plan/apply using the client credentials grant type.
# ---------------------------------------------------------------
data "verify_client_credentials_token" "cert_manager" {
  tenant_url    = var.verify_tenant_url
  client_id     = var.cert_manager_client_id
  client_secret = var.cert_manager_client_secret
}

# ---------------------------------------------------------------
# Module: jwt
# Signs an RS256 JWT using the private key from the certificate
# module. Depends on signercert so the cert is uploaded before
# the JWT is used for token exchange.
# ---------------------------------------------------------------
module "jwt" {
  source = "../modules/jwt"

  issuer          = var.jwt_issuer
  subject         = var.jwt_subject
  key_id          = var.jwt_key_id
  private_key_pem = module.certificate.private_key_pem

  depends_on = [verify_signercert.this]
}

# ---------------------------------------------------------------
# Module: token_exchange
# Exchanges the JWT (signed with the managed cert) for an access
# token. Reused until expired, then automatically re-exchanged.
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

variable "cert_manager_client_id" {
  type        = string
  description = "IBM Verify cert-manager API client ID — used for client credentials token to upload signer certs."
}

variable "cert_manager_client_secret" {
  type        = string
  description = "IBM Verify cert-manager API client secret."
  sensitive   = true
}

# Certificate variables
variable "cert_common_name" {
  type        = string
  description = "Certificate CN — must match the label in IBM Verify."
  default     = "DemoTokenSigner"
}

variable "cert_organization" {
  type        = string
  description = "Certificate O field."
  default     = "IBM"
}

variable "cert_country" {
  type        = string
  description = "Certificate C field — two-letter country code."
  default     = "US"
}

variable "cert_validity_days" {
  type        = number
  description = "Certificate validity in days."
  default     = 365
}

variable "cert_key_size" {
  type        = number
  description = "RSA key size — 2048, 3072, or 4096."
  default     = 4096
}

variable "certificate_output_path" {
  type        = string
  description = "Path where cert.pem will be written on disk."
  default     = "../examples/certificates/cert.pem"
}

variable "private_key_output_path" {
  type        = string
  description = "Path where key.pem will be written on disk. Never commit this file."
  default     = "../examples/certificates/key.pem"
}

# Bootstrap — existing stable key on disk used to get the first access token
variable "private_key_path" {
  type        = string
  description = "Path to the existing RSA private key on disk — used for bootstrap only."
  default     = "../examples/certificates/key.pem"
}

# JWT variables
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
  description = "Certificate label in IBM Verify — must match cert_common_name."
}

variable "subject_token_type" {
  type        = string
  description = "Token type URN configured in your IBM Verify STS client."
  default     = "urn:demo:token-type:user-jwt"
}

# ---------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------

output "certificate_pem" {
  value       = module.certificate.certificate_pem
  description = "Current certificate — automatically uploaded to IBM Verify."
}

output "certificate_path" {
  value       = module.certificate.certificate_path
  description = "Path to cert.pem on disk."
}

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
