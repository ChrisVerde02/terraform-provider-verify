variable "common_name" {
  type        = string
  description = "Certificate CN field — used as the display name in IBM Verify."
  default     = "DemoTokenSigner"
}

variable "organization" {
  type        = string
  description = "Certificate O field."
  default     = "IBM"
}

variable "country" {
  type        = string
  description = "Certificate C field — two-letter country code."
  default     = "US"
}

variable "validity_days" {
  type        = number
  description = "Certificate validity period in days."
  default     = 365
}

variable "key_size" {
  type        = number
  description = "RSA key size — 2048, 3072, or 4096."
  default     = 4096
}

variable "certificate_output_path" {
  type        = string
  description = "Path where cert.pem will be written on disk."
  default     = "./certificates/cert.pem"
}

variable "private_key_output_path" {
  type        = string
  description = "Path where key.pem will be written on disk. Never commit this file."
  default     = "./certificates/key.pem"
}
