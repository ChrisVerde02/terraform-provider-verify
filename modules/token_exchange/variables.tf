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

variable "subject_token" {
  type        = string
  description = "Signed JWT to exchange for an access token."
  sensitive   = true
}

variable "subject_token_type" {
  type        = string
  description = "Token type URN configured in the IBM Verify STS client."
  default     = "urn:demo:token-type:user-jwt"
}
