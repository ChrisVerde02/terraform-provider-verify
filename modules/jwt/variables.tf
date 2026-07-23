variable "issuer" {
  type        = string
  description = "Value placed in the JWT iss claim."
}

variable "subject" {
  type        = string
  description = "IBM Verify Cloud Directory username placed in the JWT sub claim."
}

variable "key_id" {
  type        = string
  description = "Certificate label in IBM Verify placed in the JWT kid header."
}

variable "private_key_pem" {
  type        = string
  description = "RSA private key used to sign the JWT."
  sensitive   = true
}

variable "expires_in_seconds" {
  type        = number
  description = "JWT lifetime in seconds."
  default     = 900
}
