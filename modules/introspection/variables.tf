variable "tenant_url" {
  type        = string
  description = "IBM Verify tenant base URL."
}

variable "client_id" {
  type        = string
  description = "IBM Verify STS API client ID."
}

variable "client_secret" {
  type        = string
  description = "IBM Verify STS API client secret."
  sensitive   = true
}

variable "token" {
  type        = string
  description = "Access token to introspect."
  sensitive   = true
}
