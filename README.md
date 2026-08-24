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

```hcl
terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "~> 0.4"
    }
  }
}
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

This client uploads and manages the signer certificate in IBM Verify.

In the IBM Verify console, go to **Security → API access → Add API client**:

| Field | Value |
|---|---|
| Name | `cert-manager` |
| Grant type | Client credentials |

Under **Entitlements**, enable:
- **Manage certificates**
- **Read certificates**

> **Important:** IBM Verify lowercases all signer certificate labels on storage. The `jwt_key_id` in your `terraform.tfvars` must use lowercase (e.g. `demotokensigner`) to match.

## Provider configuration

Credentials can be supplied either in the `provider` block or via environment variables. Environment variables take effect when the block attribute is omitted.

```hcl
provider "verify" {
  tenant_url = "https://YOUR-TENANT.verify.ibm.com"

  # STS client — used for token exchange and introspection
  sts_client_id     = var.sts_client_id
  sts_client_secret = var.sts_client_secret

  # cert-manager client — used for signer certificate management
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
}
```

### Environment variables

| Variable | Provider attribute |
|---|---|
| `VERIFY_TENANT_URL` | `tenant_url` |
| `VERIFY_STS_CLIENT_ID` | `sts_client_id` |
| `VERIFY_STS_CLIENT_SECRET` | `sts_client_secret` |
| `VERIFY_CERT_MANAGER_CLIENT_ID` | `cert_manager_client_id` |
| `VERIFY_CERT_MANAGER_CLIENT_SECRET` | `cert_manager_client_secret` |

When credentials are configured in the provider block, resources do not need their own `client_id` / `client_secret` inputs.

## Project structure

```text
terraform-provider-verify/
├── main.go
├── go.mod
├── go.sum
├── README.md
├── .gitignore
├── .goreleaser.yml
├── .github/
│   └── workflows/
│       ├── ci.yml        # lint, vet, test on every push/PR
│       └── release.yml   # goreleaser on v* tags
├── internal/
│   ├── provider/
│   │   └── provider.go
│   ├── resources/
│   │   ├── certificate_resource.go       # verify_certificate
│   │   ├── jwt_resource.go               # verify_jwt
│   │   ├── token_exchange_resource.go    # verify_token_exchange
│   │   ├── signercert_resource.go        # verify_signercert
│   │   ├── token_introspection_resource.go
│   │   ├── validators.go                 # shared URL / label / PEM regexps
│   │   └── tests/
│   │       ├── signercert_test.go
│   │       ├── signercert_http_test.go   # httptest-backed SDK tests
│   │       ├── jwt_resource_test.go
│   │       ├── token_exchange_resource_test.go
│   │       └── validators_test.go
│   └── datasources/
│       ├── data_verify_jwt.go
│       ├── data_verify_token_exchange.go
│       ├── data_verify_client_credentials_token.go
│       └── token_introspection_data_source.go
├── modules/
│   ├── certificate/     # Wraps verify_certificate
│   ├── signercert/      # Wraps verify_signercert
│   ├── jwt/             # Wraps verify_jwt
│   ├── token_exchange/  # Wraps verify_token_exchange
│   └── introspection/   # Wraps data.verify_token_introspection
└── examples2/           # Working end-to-end example (git-ignored, see below)
    ├── main.tf
    └── terraform.tfvars
```

> `examples2/` is git-ignored because it contains `terraform.tfvars` with real credentials. The full content is shown in this README.

## Modules

The `modules/` directory contains reusable Terraform modules that wrap the provider resources. Each module is a thin wrapper — it declares the resource and exposes its outputs so you can compose them in any root configuration without repeating boilerplate.

### `modules/certificate`

Wraps `verify_certificate`. Generates an RSA key pair and self-signed cert.

| Input | Default | Description |
|---|---|---|
| `common_name` | — | Certificate CN |
| `organization` | — | Certificate O |
| `country` | — | Two-letter ISO country code |
| `validity_days` | — | Certificate lifetime in days (≥ 1) |
| `key_size` | — | RSA key size: 2048, 3072, or 4096 |
| `certificate_output_path` | — | Path to write `cert.pem` |
| `private_key_output_path` | — | Path to write `key.pem` |

Outputs: `certificate_pem`, `private_key_pem`, `certificate_path`, `private_key_path`

### `modules/jwt`

Wraps `verify_jwt`. Signs an RS256 JWT using the private key from the certificate module.

| Input | Default | Description |
|---|---|---|
| `issuer` | — | JWT `iss` claim |
| `subject` | — | JWT `sub` claim (Cloud Directory username) |
| `key_id` | — | JWT `kid` header — must match the signer cert label |
| `private_key_pem` | — | RSA private key (from `modules/certificate`) |
| `expires_in_seconds` | `900` | JWT lifetime in seconds |
| `refresh_threshold` | `60` | Seconds before expiry at which the JWT is auto-refreshed |

Outputs: `token`, `issued_at`, `expires_at`

### `modules/token_exchange`

Wraps `verify_token_exchange`. Exchanges a signed JWT for an IBM Verify access token (RFC 8693).

| Input | Default | Description |
|---|---|---|
| `tenant_url` | — | IBM Verify tenant URL |
| `client_id` | — | STS client ID |
| `client_secret` | — | STS client secret |
| `subject_token` | — | Signed JWT to exchange |
| `subject_token_type` | — | RFC 8693 token-type URN |
| `refresh_threshold` | `60` | Seconds before expiry at which the token is auto-re-exchanged |

Outputs: `access_token`, `expires_in`, `expires_at`, `grant_id`, `issued_token_type`, `token_type`, `scope`

### `modules/signercert`

Wraps `verify_signercert`. Uploads the certificate to IBM Verify. Stale certs (mismatched key pair) are automatically replaced on the next apply.

| Input | Default | Description |
|---|---|---|
| `tenant_url` | — | IBM Verify tenant URL |
| `cert_manager_client_id` | — | cert-manager client ID |
| `cert_manager_client_secret` | — | cert-manager client secret |
| `certificate_pem` | — | PEM certificate to upload |
| `label` | — | Signer cert label (lowercase, 1–128 chars) |

Outputs: `label`

### `modules/introspection`

Wraps `data.verify_token_introspection`. Confirms the access token is live and returns user metadata. Re-evaluated on every plan/apply.

| Input | Description |
|---|---|
| `tenant_url` | IBM Verify tenant URL |
| `client_id` | STS client ID |
| `client_secret` | STS client secret |
| `token` | Access token to introspect |

Outputs: `active`, `subject`, `preferred_username`, `username`, `name`, `given_name`, `scope`, `expires_at`

## Quick start

### 1. Configure the provider

Create `examples/main.tf`:

```hcl
terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "~> 0.4"
    }
  }
}

provider "verify" {
  tenant_url                 = var.verify_tenant_url
  sts_client_id              = var.sts_client_id
  sts_client_secret          = var.sts_client_secret
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
}

# ---------------------------------------------------------------
# Module: certificate
# Generates an RSA key pair and self-signed X.509 certificate.
# Stored in Terraform state — reused until near expiry,
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
# Module: signercert
# Uploads the certificate to IBM Verify so it can validate JWT
# signatures during token exchange.
# ---------------------------------------------------------------
module "signercert" {
  source = "../modules/signercert"

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

  depends_on = [module.signercert]
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
```

### 2. Create `examples/terraform.tfvars`

```hcl
# IBM Verify tenant URL
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"

# STS API client credentials (token exchange)
sts_client_id     = "YOUR-STS-CLIENT-ID"
sts_client_secret = "YOUR-STS-CLIENT-SECRET"

# cert-manager API client credentials
cert_manager_client_id     = "YOUR-CERT-MANAGER-CLIENT-ID"
cert_manager_client_secret = "YOUR-CERT-MANAGER-CLIENT-SECRET"

# Certificate settings — IBM Verify lowercases cert labels, use lowercase
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

# Token type URN — must match your IBM Verify STS client configuration
subject_token_type = "urn:demo:token-type:user-jwt"
```

> **Never commit `terraform.tfvars`.** It is already git-ignored in this project.

### 3. Initialise and apply

```bash
cd examples
terraform init
terraform apply
```

On success you will see outputs including `token_active = true` and `introspected_preferred_username`.

### 4. Verify idempotency

```bash
terraform plan
# Should show: No changes. Your infrastructure matches the configuration.
```

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

# Tear everything down (removes cert from IBM Verify, clears all state)
terraform destroy
```

## Provider resources reference

### `verify_certificate`

Generates a self-signed RSA X.509 certificate and private key locally. Self-healing — automatically regenerates 24 hours before the `NotAfter` date.

```hcl
resource "verify_certificate" "example" {
  common_name   = "demotokensigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096  # 2048, 3072, or 4096
}
```

| Attribute | Type | Description |
|---|---|---|
| `common_name` | string (required) | Certificate CN |
| `organization` | string (required) | Certificate O |
| `country` | string (required) | Two-letter ISO country code |
| `validity_days` | number (required) | Lifetime in days (≥ 1) |
| `key_size` | number (required) | RSA key bits: 2048, 3072, or 4096 |
| `certificate_pem` | string (computed) | Generated X.509 certificate |
| `private_key_pem` | string (computed, sensitive) | RSA private key |
| `expires_at` | number (computed) | Certificate NotAfter as Unix timestamp |

> Import not supported — this resource generates keys locally. Use `data.verify_jwt` for stateless operations.

### `verify_signercert`

Uploads a PEM certificate to IBM Verify as a signer certificate. IBM Verify uses it to validate JWT signatures during token exchange. Stale certs (key-pair mismatch) are automatically replaced. Supports `terraform import`.

```hcl
resource "verify_signercert" "example" {
  tenant_url                 = "https://YOUR-TENANT.verify.ibm.com"
  cert_manager_client_id     = var.cert_manager_client_id     # optional if set in provider
  cert_manager_client_secret = var.cert_manager_client_secret # optional if set in provider
  certificate_pem            = verify_certificate.example.certificate_pem
  label                      = "demotokensigner"
}
```

```bash
# Import an existing cert into Terraform state
terraform import verify_signercert.example demotokensigner
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `cert_manager_client_id` | string (optional) | cert-manager client ID — falls back to provider block |
| `cert_manager_client_secret` | string (optional, sensitive) | cert-manager client secret — falls back to provider block |
| `certificate_pem` | string (required) | PEM certificate beginning with `-----BEGIN CERTIFICATE-----` |
| `label` | string (required) | Cert label in IBM Verify — must match JWT `kid` header. Letters, digits, dots, hyphens, underscores (1–128 chars) |

### `verify_jwt`

Generates an RS256-signed JWT using an RSA private key. Self-healing — automatically regenerates with a fresh `jti` when fewer than `refresh_threshold` seconds remain before expiry, preventing IBM Verify replay rejection (`CSIAQ5206E`).

```hcl
resource "verify_jwt" "example" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  private_key_pem    = verify_certificate.example.private_key_pem
  expires_in_seconds = 900
  refresh_threshold  = 60  # optional, default 60
}
```

| Attribute | Type | Description |
|---|---|---|
| `issuer` | string (required) | JWT `iss` claim |
| `subject` | string (required) | JWT `sub` claim (Cloud Directory username) |
| `key_id` | string (required) | JWT `kid` header — must match the signer cert label |
| `private_key_pem` | string (required, sensitive) | RSA private key |
| `expires_in_seconds` | number (required) | JWT lifetime in seconds (≥ 1) |
| `refresh_threshold` | number (optional) | Seconds before expiry to auto-refresh. Default: `60` |
| `token` | string (computed, sensitive) | Signed JWT |
| `issued_at` | number (computed) | `iat` claim as Unix timestamp |
| `expires_at` | number (computed) | `exp` claim as Unix timestamp |
| `jwt_id` | string (computed) | `jti` claim value |

> Import not supported — JWTs are signed locally. Use `data.verify_jwt` for stateless JWT generation.

### `verify_token_exchange`

Exchanges a custom JWT for an IBM Verify access token using RFC 8693. Self-healing — automatically re-exchanges when fewer than `refresh_threshold` seconds remain before the access token expires.

```hcl
resource "verify_token_exchange" "example" {
  tenant_url         = "https://YOUR-TENANT.verify.ibm.com"
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = verify_jwt.example.token
  subject_token_type = "urn:demo:token-type:user-jwt"
  refresh_threshold  = 60  # optional, default 60
}
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `client_id` | string (required) | STS client ID |
| `client_secret` | string (required, sensitive) | STS client secret |
| `subject_token` | string (required, sensitive) | Signed JWT to exchange |
| `subject_token_type` | string (optional) | RFC 8693 token-type URN. Default: `urn:demo:token-type:user-jwt` |
| `refresh_threshold` | number (optional) | Seconds before expiry to auto-re-exchange. Default: `60` |
| `access_token` | string (computed, sensitive) | Access token returned by IBM Verify |
| `expires_in` | number (computed) | Token lifetime in seconds |
| `expires_at` | number (computed) | Token expiry as Unix timestamp |
| `grant_id` | string (computed) | Grant ID |
| `issued_token_type` | string (computed) | Token type issued |
| `scope` | string (computed) | Granted scopes |
| `token_type` | string (computed) | Authorization type (normally `bearer`) |

**Refresh behaviour on failure:**
- HTTP 404 or `CSIAQ5212E` (cert deleted) → resource removed from state, recreated on next apply. A warning diagnostic is shown.
- All other errors (401, 5xx, network) → error diagnostic shown, state preserved so the problem is visible.

> Import not supported — access tokens are ephemeral. Use `data.verify_token_exchange` for stateless workflows.

### `data.verify_token_introspection`

Introspects an IBM Verify access token. Re-evaluated on every plan/apply.

```hcl
data "verify_token_introspection" "example" {
  tenant_url    = "https://YOUR-TENANT.verify.ibm.com"
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = verify_token_exchange.example.access_token
}
```

Outputs: `active`, `subject`, `preferred_username`, `username`, `name`, `given_name`, `scope`, `token_type`, `issuer`, `issued_at`, `expires_at`

### `data.verify_jwt`

Generates a fresh RS256-signed JWT on every plan/apply. No state — `jti` is always unique.

```hcl
data "verify_jwt" "example" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  private_key_pem    = verify_certificate.example.private_key_pem
  expires_in_seconds = 900
}
```

Outputs: `token`, `issued_at`, `expires_at`

### `data.verify_token_exchange`

Exchanges a JWT for an IBM Verify access token on every plan/apply. No state — always fresh.

```hcl
data "verify_token_exchange" "example" {
  tenant_url         = "https://YOUR-TENANT.verify.ibm.com"
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = data.verify_jwt.example.token
  subject_token_type = "urn:demo:token-type:user-jwt"
}
```

Outputs: `access_token`, `expires_in`, `grant_id`, `issued_token_type`, `scope`, `token_type`

### `data.verify_client_credentials_token`

Obtains an access token via the OAuth 2.0 client credentials grant. Re-evaluated on every plan/apply.

```hcl
data "verify_client_credentials_token" "example" {
  tenant_url    = "https://YOUR-TENANT.verify.ibm.com"
  client_id     = var.cert_manager_client_id
  client_secret = var.cert_manager_client_secret
}
```

Outputs: `access_token`, `expires_in`, `token_type`, `scope`

## Testing

```bash
# Run all tests (no live tenant required)
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage report
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

Tests use `net/http/httptest` to mock IBM Verify responses. The full status-code matrix (200/201/204/400/401/404/429/500) is covered for `Import`, `Get`, `Delete`, `Exchange`, and `ClientCredentials`. No credentials or network access are needed.

## CI/CD

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | Push / PR to any branch | `go vet`, `go build`, `go test ./...` |
| `release.yml` | Push a `v*` tag | Builds darwin/linux/windows (amd64+arm64), creates GitHub Release via goreleaser, signs SHA256SUMS with GPG |

## Common errors

### `CSIAQ5212E Unable to verify the integrity of the token`

The JWT signature does not match the certificate stored in IBM Verify. Causes:
- `jwt_key_id` does not exactly match the cert label in IBM Verify (IBM Verify lowercases labels — use lowercase in `terraform.tfvars`)
- The certificate in IBM Verify is stale (from a previous key pair)

Fix: run `terraform apply` — the provider detects the mismatch, deletes the stale cert, and re-uploads the correct one automatically.

### `CSIAO5405E The friendly name must be unique`

A certificate with that label already exists in IBM Verify but is not tracked in Terraform state (e.g. after `rm terraform.tfstate`).

Fix — import it instead of recreating:
```bash
terraform import verify_signercert.example demotokensigner
```

Or delete the cert in IBM Verify manually, then `terraform apply`.

### `CSIAK4300E You are not authorized`

The cert-manager API client lacks `manageCerts` entitlement. Fix: ensure **Manage certificates** and **Read certificates** are enabled under **Security → API access**.

### `invalid_client` (HTTP 401)

Wrong client ID or secret, or the client does not exist in the tenant. Check your `terraform.tfvars` values.

### `invalid_request: unsupported_token_type`

The `subject_token_type` does not match what is configured in the STS client's Token Exchange settings in IBM Verify.

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
