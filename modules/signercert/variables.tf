variable "tenant_url" {
  type        = string
  description = "IBM Verify tenant base URL."
}

variable "cert_manager_client_id" {
  type        = string
  description = "Client ID of the IBM Verify API client with manageCerts entitlement. Optional when cert_manager_client_id is set in the provider block."
  default     = ""
}

variable "cert_manager_client_secret" {
  type        = string
  description = "Client secret of the cert-manager API client. Optional when cert_manager_client_secret is set in the provider block."
  sensitive   = true
  default     = ""
}

variable "certificate_pem" {
  type        = string
  description = "PEM-encoded X.509 certificate to upload."
}

variable "label" {
  type        = string
  description = "Signer certificate label in IBM Verify. Must match the JWT kid. IBM Verify lowercases labels — use lowercase."
}
