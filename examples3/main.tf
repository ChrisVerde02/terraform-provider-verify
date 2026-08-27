terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.8.1"
    }
  }
}

provider "verify" {
  tenant_url        = var.verify_tenant_url
  sts_client_id     = var.sts_client_id
  sts_client_secret = var.sts_client_secret
}

# ---------------------------------------------------------------
# Resource: verify_api_client
# Registers a new API client in IBM Verify via Dynamic Client
# Registration (DCR). IBM Verify generates and returns the
# client_id and client_secret — stored in Terraform state.
# ---------------------------------------------------------------
resource "verify_api_client" "demo" {
  tenant_url  = var.verify_tenant_url
  client_name = var.api_client_name
  entitlements = var.api_client_entitlements
  enabled     = true
  description = "Terraform-managed demo API client"
}

# ---------------------------------------------------------------
# Resource: verify_user
# Creates a Cloud Directory user in IBM Verify via the SCIM v2
# API. The user_id is assigned by IBM Verify on creation.
# ---------------------------------------------------------------
resource "verify_user" "demo" {
  tenant_url  = var.verify_tenant_url
  username    = var.user_username
  given_name  = var.user_given_name
  family_name = var.user_family_name
  email       = var.user_email
  active      = true
}

# ---------------------------------------------------------------
# Resource: verify_application
# Creates an IBM Verify application from a template.
# The application_id is assigned by IBM Verify on creation.
# ---------------------------------------------------------------
resource "verify_application" "demo" {
  tenant_url  = var.verify_tenant_url
  name        = var.app_name
  template_id = var.app_template_id
}

# ---------------------------------------------------------------
# Variables
# ---------------------------------------------------------------

variable "verify_tenant_url" {
  type        = string
  description = "IBM Verify tenant base URL, e.g. https://example.verify.ibm.com."
}

variable "sts_client_id" {
  type        = string
  description = "IBM Verify STS client ID — used for all three new resources."
}

variable "sts_client_secret" {
  type        = string
  description = "IBM Verify STS client secret."
  sensitive   = true
}

variable "api_client_name" {
  type        = string
  description = "Display name for the new API client in IBM Verify."
  default     = "terraform-demo-client"
}

variable "api_client_entitlements" {
  type        = list(string)
  description = "List of entitlements to grant the API client."
  default     = ["readApiClients"]
}

variable "user_username" {
  type        = string
  description = "Unique username (userName) for the new Cloud Directory user."
}

variable "user_given_name" {
  type        = string
  description = "First name of the new user."
  default     = "Demo"
}

variable "user_family_name" {
  type        = string
  description = "Last name of the new user."
  default     = "User"
}

variable "user_email" {
  type        = string
  description = "Work email address for the new user."
}

variable "app_name" {
  type        = string
  description = "Display name for the new application in IBM Verify."
  default     = "terraform-demo-app"
}

variable "app_template_id" {
  type        = string
  description = "IBM Verify template ID for the application type (e.g. SAML, OIDC)."
}

# ---------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------

output "api_client_id" {
  value       = verify_api_client.demo.client_id
  description = "The generated client_id for the new API client."
}

output "api_client_secret" {
  value       = verify_api_client.demo.client_secret
  description = "The generated client_secret for the new API client."
  sensitive   = true
}

output "api_client_enabled" {
  value       = verify_api_client.demo.enabled
  description = "Whether the API client is enabled."
}

output "user_id" {
  value       = verify_user.demo.user_id
  description = "The SCIM id assigned to the new user by IBM Verify."
}

output "user_display_name" {
  value       = verify_user.demo.display_name
  description = "The display name of the created user."
}

output "application_id" {
  value       = verify_application.demo.application_id
  description = "The application ID assigned by IBM Verify."
}

output "application_state" {
  value       = verify_application.demo.application_state
  description = "The application state as reported by IBM Verify (true = active)."
}
