terraform {
  required_providers {
    verify = {
      # Uses the published provider from the Terraform Registry.
      # No go build, no ~/.terraformrc override needed.
      source  = "ChrisVerde02/verify"
      version = "~> 0.1"
    }

    random = {
      source = "hashicorp/random"
    }
  }
}

provider "verify" {}

# Read the private key from the path supplied in terraform.tfvars.
# The key was generated once by examples/cert/ and never changes.
locals {
  private_key_pem = file(var.private_key_path)
}

variable "private_key_path" {
  type        = string
  description = "Path to the RSA private key PEM file."
  default     = "../examples/certificates/key.pem"
}

# Generates a new UUID whenever any JWT input changes.
# Prevents IBM Verify from rejecting the token with a jti replay error.
resource "random_uuid" "jwt_id" {
  keepers = {
    issuer  = var.jwt_issuer
    subject = var.jwt_subject
    key_id  = var.jwt_key_id
  }
}

resource "verify_jwt" "custom" {
  issuer             = var.jwt_issuer
  subject            = var.jwt_subject
  key_id             = var.jwt_key_id
  jwt_id             = random_uuid.jwt_id.result
  private_key_pem    = local.private_key_pem
  expires_in_seconds = 900
}

resource "verify_token_exchange" "example" {
  tenant_url         = var.verify_tenant_url
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = verify_jwt.custom.token
  subject_token_type = var.subject_token_type
}

data "verify_token_introspection" "example" {
  tenant_url    = var.verify_tenant_url
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = verify_token_exchange.example.access_token
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

variable "jwt_issuer" {
  type        = string
  description = "Value placed in the JWT iss claim."
}

variable "jwt_subject" {
  type        = string
  description = "IBM Verify Cloud Directory username placed in the JWT sub claim."
}

variable "jwt_key_id" {
  type        = string
  description = "Certificate label in IBM Verify — must match the label set when uploading cert.pem."
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
  value     = verify_token_exchange.example.access_token
  sensitive = true
}

output "access_token_expires_in" {
  value = verify_token_exchange.example.expires_in
}

output "grant_id" {
  value = verify_token_exchange.example.grant_id
}

output "issued_token_type" {
  value = verify_token_exchange.example.issued_token_type
}

output "token_type" {
  value = verify_token_exchange.example.token_type
}

output "token_active" {
  value = data.verify_token_introspection.example.active
}

output "introspected_subject" {
  value = data.verify_token_introspection.example.subject
}

output "introspected_preferred_username" {
  value = data.verify_token_introspection.example.preferred_username
}

output "introspected_expires_at" {
  value = data.verify_token_introspection.example.expires_at
}
