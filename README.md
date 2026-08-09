# Terraform Provider for IBM Verify

A custom Terraform provider that automates the full IBM Verify JWT token-exchange workflow — including certificate generation, automatic upload to IBM Verify, JWT signing, token exchange, and token introspection. Everything is fully managed and self-healing: certificates and tokens are automatically rotated when they expire.

## What it does

```text
Generate RSA key pair + self-signed X.509 certificate
        |
        v
Upload certificate to IBM Verify (as a signer cert)
        |
        v
Sign an RS256 JWT with the private key
        |
        v
Exchange JWT for an IBM Verify access token (RFC 8693 Token Exchange)
        |
        v
Introspect the access token — confirm it is active
```

All steps run automatically on `terraform apply`. On subsequent runs, Terraform reuses valid state (cert, JWT, access token) and only refreshes what has expired. `terraform plan` is fully idempotent — no changes are shown after a successful apply.

## Provider version

```
ChrisVerde02/verify ~> 0.4.1
```

Published at: https://registry.terraform.io/providers/ChrisVerde02/verify

## IBM Verify prerequisites

You need two API clients configured in your IBM Verify tenant before running Terraform.

### 1. STS client (token exchange)

This client performs the RFC 8693 JWT token exchange that produces the final access token.

In the IBM Verify console, go to **Applications → STS clients → Add**:

| Field | Value |
|---|---|
| Name | Any name, e.g. `DEMO JWT impersonation client` |
| Grant type | Token Exchange |
| Subject token type | Custom URN, e.g. `urn:demo:token-type:user-jwt` |
| JWT validation | Add the signer cert label (see below) |

Under **Custom scopes and API access**, no special entitlements are needed for this client — it performs user impersonation, not admin operations.

### 2. cert-manager API client (certificate management)

This client is used to upload and manage the signer certificate in IBM Verify.

In the IBM Verify console, go to **Security → API access → Add API client**:

| Field | Value |
|---|---|
| Name | `cert-manager` |
| Grant type | Client credentials |

Under **Entitlements**, enable:
- **Manage certificates**
- **Read certificates**

> **Important:** IBM Verify lowercases all signer certificate labels on storage. The `jwt_key_id` in your `terraform.tfvars` must use lowercase (e.g. `demotokensigner`) to match.

## Project structure

```text
terraform-provider-verify/
├── main.go
├── go.mod
├── go.sum
├── README.md
├── .gitignore
├── internal/
│   ├── provider/
│   │   └── provider.go
│   ├── resources/
│   │   ├── certificate_resource.go       # verify_certificate
│   │   ├── jwt_resource.go               # verify_jwt
│   │   ├── token_exchange_resource.go    # verify_token_exchange
│   │   ├── signercert_resource.go        # verify_signercert
│   │   └── token_introspection_resource.go
│   └── datasources/
│       ├── data_verify_jwt.go
│       ├── data_verify_token_exchange.go
│       ├── data_verify_client_credentials_token.go
│       └── token_introspection_data_source.go
├── modules/
│   ├── certificate/     # Wraps verify_certificate + local file writes
│   ├── jwt/             # Wraps verify_jwt
│   ├── token_exchange/  # Wraps verify_token_exchange
│   └── introspection/   # Wraps verify_token_introspection data source
└── examples2/           # Working end-to-end example (git-ignored, see below)
    ├── main.tf
    └── terraform.tfvars
```

> `examples2/` is git-ignored because it contains `terraform.tfvars` with real credentials. The full content is shown in this README.

## Quick start

### 1. Create the `examples2/` directory

```bash
mkdir examples2
```

### 2. Create `examples2/main.tf`

```hcl
terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.4.1"
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
# Resource: signercert
# Uploads the certificate to IBM Verify so it can validate JWT
# signatures during token exchange. Obtains its own client
# credentials token internally — no access_token in config.
# ---------------------------------------------------------------
resource "verify_signercert" "this" {
  tenant_url                 = var.verify_tenant_url
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
  certificate_pem            = module.certificate.certificate_pem
  label                      = var.jwt_key_id
}

# ---------------------------------------------------------------
# Module: jwt
# Signs an RS256 JWT using the private key from the certificate
# module. Depends on signercert so the cert is uploaded first.
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
# Exchanges the JWT for an IBM Verify access token (RFC 8693).
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
# Confirms the access token is active — re-evaluated every apply.
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
  description = "IBM Verify cert-manager API client ID."
}

variable "cert_manager_client_secret" {
  type        = string
  description = "IBM Verify cert-manager API client secret."
  sensitive   = true
}

variable "cert_common_name" {
  type    = string
  default = "demotokensigner"
}

variable "cert_organization" {
  type    = string
  default = "IBM"
}

variable "cert_country" {
  type    = string
  default = "US"
}

variable "cert_validity_days" {
  type    = number
  default = 365
}

variable "cert_key_size" {
  type    = number
  default = 4096
}

variable "certificate_output_path" {
  type    = string
  default = "../examples/certificates/cert.pem"
}

variable "private_key_output_path" {
  type    = string
  default = "../examples/certificates/key.pem"
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
  description = "Signer cert label in IBM Verify. IBM Verify lowercases labels — use lowercase here."
}

variable "subject_token_type" {
  type    = string
  default = "urn:demo:token-type:user-jwt"
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
```

### 3. Create `examples2/terraform.tfvars`

```hcl
# IBM Verify tenant URL
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"

# STS API client credentials (token exchange — impersonates user via JWT)
sts_client_id     = "YOUR-STS-CLIENT-ID"
sts_client_secret = "YOUR-STS-CLIENT-SECRET"

# cert-manager API client credentials (client credentials — uploads signer certs)
cert_manager_client_id     = "YOUR-CERT-MANAGER-CLIENT-ID"
cert_manager_client_secret = "YOUR-CERT-MANAGER-CLIENT-SECRET"

# Certificate settings
# IBM Verify lowercases cert labels — use lowercase here
cert_common_name   = "demotokensigner"
cert_organization  = "IBM"
cert_country       = "US"
cert_validity_days = 365
cert_key_size      = 4096

# Where to write cert and key files on disk
certificate_output_path = "../examples/certificates/cert.pem"
private_key_output_path = "../examples/certificates/key.pem"

# JWT claim values
jwt_issuer  = "https://demo.ibm.com"
jwt_subject = "YOUR-CLOUD-DIRECTORY-USERNAME"
jwt_key_id  = "demotokensigner"

# Token type URN configured in your IBM Verify STS client
subject_token_type = "urn:demo:token-type:user-jwt"
```

> **Never commit `terraform.tfvars`.** It is already git-ignored in this project.

### 4. Initialise and apply

```bash
cd examples2
terraform init
terraform apply
```

On success you will see outputs including `token_active = true` and `introspected_preferred_username`.

### 5. Verify idempotency

```bash
terraform plan
```

Should show: `No changes. Your infrastructure matches the configuration.`

## Useful commands

```bash
# View the access token
terraform output -raw access_token

# View the certificate
terraform output certificate_pem

# View when the access token expires
terraform output introspected_expires_at

# Inspect the certificate on disk
openssl x509 -in ../examples/certificates/cert.pem -noout -text

# Tear everything down (removes cert from IBM Verify, clears state)
terraform destroy
```

## Provider resources

### `verify_certificate`

Generates a self-signed RSA X.509 certificate and private key. Self-healing — automatically regenerates 24 hours before expiry.

```hcl
resource "verify_certificate" "example" {
  common_name   = "demotokensigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}
```

Outputs: `certificate_pem`, `private_key_pem`, `expires_at`

### `verify_signercert`

Uploads a signer certificate to IBM Verify. Obtains its own client credentials token internally — no `access_token` input needed. On `terraform destroy`, deletes the cert from IBM Verify. On the next `terraform apply`, re-uploads it.

```hcl
resource "verify_signercert" "example" {
  tenant_url                 = "https://YOUR-TENANT.verify.ibm.com"
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
  certificate_pem            = verify_certificate.example.certificate_pem
  label                      = "demotokensigner"
}
```

### `verify_jwt`

Generates an RS256-signed JWT. Self-healing — automatically regenerates with a fresh `jti` 60 seconds before expiry.

```hcl
resource "verify_jwt" "example" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  private_key_pem    = verify_certificate.example.private_key_pem
  expires_in_seconds = 900
}
```

JWT claims: `iss`, `sub`, `iat`, `exp`, `jti`. JWT header: `alg=RS256`, `kid=<label>`.

### `verify_token_exchange`

Exchanges the JWT for an IBM Verify access token using RFC 8693. Self-healing — re-exchanges automatically 60 seconds before the access token expires.

```hcl
resource "verify_token_exchange" "example" {
  tenant_url         = "https://YOUR-TENANT.verify.ibm.com"
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = verify_jwt.example.token
  subject_token_type = "urn:demo:token-type:user-jwt"
}
```

Outputs: `access_token`, `expires_in`, `expires_at`, `grant_id`, `issued_token_type`, `token_type`

### `data.verify_token_introspection`

Introspects the access token against IBM Verify. Re-evaluated on every plan/apply.

```hcl
data "verify_token_introspection" "example" {
  tenant_url    = "https://YOUR-TENANT.verify.ibm.com"
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = verify_token_exchange.example.access_token
}
```

Outputs: `active`, `subject`, `preferred_username`, `username`, `name`, `given_name`, `scope`, `expires_at`

### `data.verify_client_credentials_token`

Obtains an access token using the OAuth 2.0 client credentials grant. Re-evaluated on every plan/apply.

```hcl
data "verify_client_credentials_token" "example" {
  tenant_url    = "https://YOUR-TENANT.verify.ibm.com"
  client_id     = var.cert_manager_client_id
  client_secret = var.cert_manager_client_secret
}
```

## Common errors

### `CSIAQ5212E Unable to verify the integrity of the token`

The JWT signature does not match the certificate stored in IBM Verify. Causes:
- `jwt_key_id` does not exactly match the cert label in IBM Verify (IBM Verify lowercases labels — use lowercase in `terraform.tfvars`)
- The certificate in IBM Verify is stale (from a previous key pair)

Fix: delete the cert in IBM Verify, then run `terraform apply`.

### `CSIAO5405E The friendly name must be unique`

A certificate with that label already exists in IBM Verify but is not tracked in Terraform state (e.g. after `rm terraform.tfstate`).

Fix:
```bash
# Delete from IBM Verify manually, then apply
terraform apply
```

Or use `terraform destroy` before wiping state.

### `CSIAK4300E You are not authorized`

The access token used to upload the cert lacks `manageCerts` entitlement. Fix: ensure the cert-manager API client has **Manage certificates** and **Read certificates** enabled under **Security → API access**.

### `invalid_client` (HTTP 401)

Wrong client ID or secret, or the STS client does not exist in the tenant. Check your `terraform.tfvars` values.

### `invalid_request: unsupported_token_type`

The `subject_token_type` in `terraform.tfvars` does not match what is configured in the STS client's Token Exchange settings in IBM Verify.

## Security

The following are sensitive and must never be committed:

- `terraform.tfvars` — contains client secrets
- `terraform.tfstate` — contains the private key, JWTs, and access tokens
- `examples/certificates/key.pem` — RSA private key

All are already git-ignored in this project. If a secret is accidentally exposed, rotate it immediately in IBM Verify.

## Destroy and restart cleanly

```bash
cd examples2
terraform destroy       # removes cert from IBM Verify, clears all state
terraform apply         # rebuilds everything from scratch
```

Never wipe `terraform.tfstate` manually without running `terraform destroy` first — doing so leaves the signer certificate orphaned in IBM Verify, which causes `CSIAO5405E` on the next apply.
