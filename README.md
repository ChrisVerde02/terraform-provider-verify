# Terraform Provider for IBM Verify

A custom Terraform provider that automates IBM Verify administration — including certificate generation, JWT signing, token exchange, user management, application management, and dynamic client registration. Everything is fully managed and self-healing: certificates and tokens rotate automatically when they expire, and all resources are idempotent — running `terraform apply` twice is always safe.

## What it does

**JWT token-exchange workflow** (original):
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

**IBM Verify resource management** (new in v0.8.x):
```text
Create an IBM Verify application (from a template)
Create a Cloud Directory user (SCIM v2)
Register a Dynamic Client (DCR) — get client_id + client_secret
```

All steps run automatically on `terraform apply`. On subsequent runs, Terraform reuses valid state and only refreshes what has expired or changed.

## Provider version

```hcl
terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "~> 0.8"
    }
  }
}
```

Published at: https://registry.terraform.io/providers/ChrisVerde02/verify

## IBM Verify prerequisites

### For the JWT token-exchange workflow

You need two API clients configured in your IBM Verify tenant.

#### STS client (token exchange)

In the IBM Verify console, go to **Applications → STS clients → Add**:

| Field | Value |
|---|---|
| Name | Any name, e.g. `DEMO JWT impersonation client` |
| Grant type | Token Exchange |
| Subject token type | Custom URN, e.g. `urn:demo:token-type:user-jwt` |
| JWT validation | Add the signer cert label (see below) |

#### cert-manager API client (certificate management)

In the IBM Verify console, go to **Security → API access → Add API client**:

| Field | Value |
|---|---|
| Name | `cert-manager` |
| Grant type | Client credentials |
| Entitlements | **Manage certificates**, **Read certificates** |

> IBM Verify lowercases all signer certificate labels on storage. The `jwt_key_id` in your `terraform.tfvars` must use lowercase (e.g. `demotokensigner`).

### For resource management (applications, users, API clients)

A single API client with the relevant entitlements covers all three new resources:

| Resource | Required entitlement |
|---|---|
| `verify_application` | `manageApplications` |
| `verify_user` | `manageUsers` |
| `verify_api_client` | `manageApiClients` |

The same client can hold all three entitlements. Configure it under **Security → API access → Add API client** with **Client credentials** grant type.

## Provider configuration

```hcl
provider "verify" {
  tenant_url = "https://YOUR-TENANT.verify.ibm.com"

  # STS client — used for token exchange, introspection, users, applications, api_clients
  sts_client_id     = var.sts_client_id
  sts_client_secret = var.sts_client_secret

  # cert-manager client — used for signer certificate management
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret

  # Optional: dedicated app client credentials (falls back to sts_client if omitted)
  # app_client_id     = var.app_client_id
  # app_client_secret = var.app_client_secret
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
| `VERIFY_APP_CLIENT_ID` | `app_client_id` |
| `VERIFY_APP_CLIENT_SECRET` | `app_client_secret` |

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
│   │   ├── application_resource.go       # verify_application  (v0.8.x)
│   │   ├── user_resource.go              # verify_user         (v0.8.x)
│   │   ├── api_client_resource.go        # verify_api_client   (v0.8.x)
│   │   ├── validators.go                 # shared URL / label / PEM regexps
│   │   └── tests/
│   │       ├── signercert_http_test.go
│   │       ├── application_resource_test.go
│   │       ├── user_resource_test.go
│   │       ├── api_client_resource_test.go
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
├── examples2/           # JWT token-exchange workflow demo (git-ignored)
└── examples3/           # New resources demo: application, user, api_client (git-ignored)
```

## Quick start — JWT token-exchange workflow

### 1. Configure `examples/terraform.tfvars`

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"

sts_client_id     = "YOUR-STS-CLIENT-ID"
sts_client_secret = "YOUR-STS-CLIENT-SECRET"

cert_manager_client_id     = "YOUR-CERT-MANAGER-CLIENT-ID"
cert_manager_client_secret = "YOUR-CERT-MANAGER-CLIENT-SECRET"

cert_common_name   = "demotokensigner"
cert_organization  = "IBM"
cert_country       = "US"
cert_validity_days = 365
cert_key_size      = 4096

certificate_output_path = "../examples/certificates/cert.pem"
private_key_output_path = "../examples/certificates/key.pem"

jwt_issuer  = "https://demo.ibm.com"
jwt_subject = "YOUR-CLOUD-DIRECTORY-USERNAME"
jwt_key_id  = "demotokensigner"

subject_token_type = "urn:demo:token-type:user-jwt"
```

### 2. Apply

```bash
cd examples2
terraform init
terraform apply
```

### 3. Verify idempotency

```bash
terraform plan
# Should show: No changes. Your infrastructure matches the configuration.
```

## Quick start — resource management (applications, users, API clients)

### 1. Configure `examples3/terraform.tfvars`

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"

# One API client covering all three resources
sts_client_id     = "YOUR-CLIENT-ID"
sts_client_secret = "YOUR-CLIENT-SECRET"

api_client_name         = "terraform-demo-client"
api_client_entitlements = ["readApiClients"]

user_username    = "demo.user@example.com"
user_given_name  = "Demo"
user_family_name = "User"
user_email       = "demo.user@example.com"

app_name        = "terraform-demo-app"
app_template_id = "YOUR-TEMPLATE-ID"   # from IBM Verify: Applications → Add application
```

### 2. Apply

```bash
cd examples3
terraform init
terraform apply
```

Running `terraform apply` again produces **no changes** — all three resources are idempotent.

## Useful commands

```bash
# View the access token
terraform output -raw access_token

# View when the token expires
terraform output introspected_expires_at

# View the generated API client ID
terraform output api_client_id

# Show the API client secret (sensitive)
terraform output -raw api_client_secret

# Tear everything down
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

> Import not supported — this resource generates keys locally.

---

### `verify_signercert`

Uploads a PEM certificate to IBM Verify as a signer certificate. IBM Verify uses it to validate JWT signatures during token exchange. Idempotent — if the same cert already exists it is adopted into state. Stale certs (key-pair mismatch) are automatically replaced. Supports `terraform import`.

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
terraform import verify_signercert.example demotokensigner
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `cert_manager_client_id` | string (optional) | Falls back to provider block |
| `cert_manager_client_secret` | string (optional, sensitive) | Falls back to provider block |
| `certificate_pem` | string (required) | PEM certificate |
| `label` | string (required) | Cert label — must match JWT `kid`. Letters, digits, dots, hyphens, underscores (1–128 chars) |

---

### `verify_jwt`

Generates an RS256-signed JWT. Self-healing — automatically regenerates with a fresh `jti` when fewer than `refresh_threshold` seconds remain before expiry, preventing IBM Verify replay rejection (`CSIAQ5206E`).

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
| `subject` | string (required) | JWT `sub` claim |
| `key_id` | string (required) | JWT `kid` header — must match signer cert label |
| `private_key_pem` | string (required, sensitive) | RSA private key |
| `expires_in_seconds` | number (required) | JWT lifetime in seconds (≥ 1) |
| `refresh_threshold` | number (optional) | Seconds before expiry to auto-refresh. Default: `60` |
| `token` | string (computed, sensitive) | Signed JWT |
| `issued_at` | number (computed) | `iat` claim as Unix timestamp |
| `expires_at` | number (computed) | `exp` claim as Unix timestamp |
| `jwt_id` | string (computed) | `jti` claim value |

---

### `verify_token_exchange`

Exchanges a custom JWT for an IBM Verify access token (RFC 8693). Self-healing — automatically re-exchanges when fewer than `refresh_threshold` seconds remain before expiry.

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
| `subject_token_type` | string (optional) | RFC 8693 URN. Default: `urn:demo:token-type:user-jwt` |
| `refresh_threshold` | number (optional) | Seconds before expiry to auto-re-exchange. Default: `60` |
| `access_token` | string (computed, sensitive) | IBM Verify access token |
| `expires_in` | number (computed) | Token lifetime in seconds |
| `expires_at` | number (computed) | Token expiry as Unix timestamp |
| `grant_id` | string (computed) | Grant ID |
| `issued_token_type` | string (computed) | Token type issued |
| `scope` | string (computed) | Granted scopes |
| `token_type` | string (computed) | Authorization type (normally `bearer`) |

**Refresh failure behaviour:**
- HTTP 404 or `CSIAQ5212E` → resource removed from state, recreated on next apply (warning shown)
- All other errors (401, 5xx, network) → error shown, state preserved

---

### `verify_application` *(v0.8.x)*

Creates and manages an IBM Verify application. **Idempotent** — if an application with the same `name` and `template_id` already exists it is adopted into state without creating a duplicate.

```hcl
resource "verify_application" "example" {
  tenant_url  = "https://YOUR-TENANT.verify.ibm.com"
  name        = "my-app"
  template_id = "1"   # from IBM Verify: Applications → Add application
}
```

```bash
terraform import verify_application.example 4946788872749535294
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `name` | string (required) | Application display name |
| `template_id` | string (required) | IBM Verify template ID — determines app type (SAML, OIDC, etc.) |
| `application_id` | string (computed) | Application ID assigned by IBM Verify |
| `application_state` | string (computed) | `"true"` = active, `"false"` = draft |

> **Idempotency key:** `name` + `template_id`. Running `terraform apply` twice, or wiping state and re-applying, will find the existing application and adopt it — no duplicate is created.

---

### `verify_user` *(v0.8.x)*

Creates and manages a Cloud Directory user in IBM Verify via the SCIM v2 API. **Idempotent** — if a user with the same `username` already exists it is adopted into state.

```hcl
resource "verify_user" "example" {
  tenant_url   = "https://YOUR-TENANT.verify.ibm.com"
  username     = "demo.user@example.com"
  given_name   = "Demo"
  family_name  = "User"
  email        = "demo.user@example.com"
  active       = true
}
```

```bash
terraform import verify_user.example 645009B7RQ
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `username` | string (required) | Unique Cloud Directory username (SCIM `userName`) |
| `given_name` | string (optional) | First name |
| `family_name` | string (optional) | Last name |
| `email` | string (optional) | Work email address |
| `password` | string (optional, sensitive) | Initial password — not tracked after creation |
| `active` | bool (optional) | Whether the account is active. Default: `true` |
| `user_id` | string (computed) | SCIM `id` assigned by IBM Verify |
| `display_name` | string (computed) | Display name returned by IBM Verify |

> **Idempotency key:** `username`. IBM Verify enforces uniqueness on `userName` — if the user exists, it is adopted rather than duplicated.

---

### `verify_api_client` *(v0.8.x)*

Creates and manages an IBM Verify API client via Dynamic Client Registration (DCR). The generated `client_id` and `client_secret` are stored in state and can be used as credentials for other resources. **Idempotent** — if a client with the same `client_name` already exists it is adopted into state.

```hcl
resource "verify_api_client" "example" {
  tenant_url   = "https://YOUR-TENANT.verify.ibm.com"
  client_name  = "terraform-managed-client"
  entitlements = ["manageCerts", "readApiClients"]
  enabled      = true
  description  = "Terraform-managed API client"
}
```

```bash
terraform import verify_api_client.example 3347abcb-3128-461a-96a9-da5361b7b317
```

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string (required) | IBM Verify tenant URL |
| `client_name` | string (required) | Display name for the API client |
| `entitlements` | list(string) (required) | Entitlements granted to the client |
| `enabled` | bool (optional) | Whether the client can generate tokens. Default: `true` |
| `description` | string (optional) | Description of the client |
| `client_id` | string (computed) | Generated client ID — use as a credential elsewhere |
| `client_secret` | string (computed, sensitive) | Generated client secret |

> **Idempotency key:** `client_name`. Running `terraform apply` twice finds the existing client by name and adopts it — no duplicate is created.
> **Note:** `client_secret` is only returned by IBM Verify at creation time. If you adopt an existing client (idempotency path), the secret will be empty in state. Destroy and re-create if you need a new secret.

---

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

---

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

---

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

---

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

---

## Idempotency

All resources in this provider are safe to apply multiple times. The table below summarises how each resource handles an already-existing counterpart in IBM Verify:

| Resource | Idempotency key | Behaviour on duplicate |
|---|---|---|
| `verify_certificate` | Local only (no remote) | Regenerates key pair locally — no IBM Verify call |
| `verify_signercert` | `label` | Adopts if cert content matches; replaces if stale |
| `verify_jwt` | Local only | Regenerates with fresh `jti` if near expiry |
| `verify_token_exchange` | Local only | Re-exchanges if near expiry; removes from state on 404 |
| `verify_application` | `name` + `template_id` | Adopts existing application into state |
| `verify_user` | `username` | Adopts existing user into state |
| `verify_api_client` | `client_name` | Adopts existing client into state |

> If state is wiped (`terraform.tfstate` deleted) and you run `terraform apply` again, all resources are found by their idempotency key and adopted — no duplicates, no manual `terraform import` needed.

## Testing

```bash
# Run all tests (no live tenant required)
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

Tests use `net/http/httptest` to mock IBM Verify responses. The full HTTP status-code matrix is covered for all domain clients (certs, tokens, apps, users, api clients). No credentials or network access needed.

## CI/CD

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | Push / PR to any branch | `go vet`, `go build`, `go test ./...` |
| `release.yml` | Push a `v*` tag | Builds darwin/linux/windows (amd64+arm64), creates GitHub Release via goreleaser, signs SHA256SUMS with GPG |

## Common errors

### `CSIAQ5212E Unable to verify the integrity of the token`

The JWT signature does not match the certificate in IBM Verify. Causes:
- `jwt_key_id` does not match the cert label (IBM Verify lowercases labels — use lowercase)
- The certificate in IBM Verify is stale (from a previous key pair)

Fix: run `terraform apply` — the provider detects the mismatch and replaces the cert automatically.

### `CSIAO5405E The friendly name must be unique`

A certificate with that label already exists in IBM Verify but is not tracked in Terraform state.

Fix: run `terraform apply` — `verify_signercert` automatically detects the existing cert and adopts it (or replaces it if stale). No manual import needed.

### `CSIAK4300E You are not authorized`

The API client lacks a required entitlement. Check:
- cert-manager client needs **Manage certificates** + **Read certificates**
- App/user management client needs **manageApplications** / **manageUsers** / **manageApiClients**

### `invalid_client` (HTTP 401)

Wrong client ID or secret, or the client does not exist in the tenant. Check your `terraform.tfvars`.

### `invalid_request: unsupported_token_type`

The `subject_token_type` does not match what is configured in the STS client's Token Exchange settings in IBM Verify.

## Security

The following are sensitive and must never be committed:

- `terraform.tfvars` — contains client secrets
- `terraform.tfstate` — contains the private key, JWTs, access tokens, and API client secrets
- `examples/certificates/key.pem` — RSA private key

All are already git-ignored in this project. If a secret is accidentally exposed, rotate it immediately in IBM Verify.

## Destroy and restart cleanly

```bash
# JWT workflow
cd examples2
terraform destroy    # removes signer cert from IBM Verify, clears state
terraform apply      # rebuilds everything from scratch

# Resource management
cd examples3
terraform destroy    # removes application, user, and api_client from IBM Verify
terraform apply      # recreates all three — idempotent, no duplicates
```

> Never wipe `terraform.tfstate` manually without running `terraform destroy` first. If you do, run `terraform apply` — all resources are idempotent and will be found and adopted automatically without creating duplicates.
