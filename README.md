# Terraform Provider for IBM Verify

A custom Terraform provider that automates the IBM Verify external identity provider token-exchange workflow.

The provider currently supports:

- Generating a self-signed RSA X.509 certificate (`verify_certificate`)
- Generating an RS256-signed custom JWT (`verify_jwt`)
- Exchanging the custom JWT for an IBM Verify access token (`verify_token_exchange`)
- Introspecting the IBM Verify access token (`data.verify_token_introspection`)

## Workflow

```text
examples/cert/ (run once)
    |
    v
Generate RSA private key and X.509 certificate
    |
    v
Upload cert.pem to IBM Verify STS client (manual step)
    |
    v
examples/ (run on every apply)
    |
    v
Read stable private key from key.pem on disk
    |
    v
Generate fresh RS256 JWT (valid 15 minutes)
    |
    v
POST JWT to IBM Verify /oauth2/token  →  receive access token
    |
    v
POST access token to IBM Verify /oauth2/introspect  →  confirm active = true
```

## Project structure

```text
terraform-provider-verify/
├── .gitignore
├── go.mod
├── go.sum
├── main.go
├── README.md
├── internal/
│   ├── client/
│   │   ├── introspection.go
│   │   └── token_exchange.go
│   ├── crypto/
│   │   ├── certificate.go
│   │   └── jwt.go
│   ├── datasources/
│   │   └── token_introspection_data_source.go
│   ├── provider/
│   │   └── provider.go
│   └── resources/
│       ├── certificate_resource.go
│       ├── jwt_resource.go
│       └── token_exchange_resource.go
└── examples/
    ├── cert/
    │   └── main.tf          ← run once to generate the certificate
    ├── main.tf
    └── terraform.tfvars
```

## Requirements

- Go 1.22 or later
- Terraform CLI
- An IBM Verify tenant
- A configured IBM Verify STS client with token exchange enabled
- A signer certificate uploaded to the STS client in IBM Verify

## Provider resources

### `verify_certificate`

Generates an RSA private key and a self-signed X.509 certificate.

> **Important:** This resource generates a new key pair every time it is created.
> It must live in its own Terraform root (`examples/cert/`) so it is never accidentally
> destroyed and regenerated. See [Running the example](#running-the-example).

Example:

```hcl
resource "verify_certificate" "this" {
  common_name   = "DemoTokenSigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}
```

Inputs:

| Attribute | Type | Description |
|---|---|---|
| `common_name` | string | Certificate CN field |
| `organization` | string | Certificate O field |
| `country` | string | Certificate C field (two-letter code) |
| `validity_days` | number | Certificate validity period in days |
| `key_size` | number | RSA key size — 2048, 3072, or 4096 |

Outputs:

| Attribute | Description |
|---|---|
| `certificate_pem` | PEM-encoded X.509 certificate — upload this to IBM Verify |
| `private_key_pem` | PEM-encoded RSA private key (sensitive) |

---

### `verify_jwt`

Generates an RS256-signed JWT using a stable RSA private key read from disk.

Example:

```hcl
resource "random_uuid" "jwt_id" {}

resource "verify_jwt" "custom" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  jwt_id             = random_uuid.jwt_id.result
  private_key_pem    = local.private_key_pem
  expires_in_seconds = 900
}
```

The generated JWT includes these claims:

| Claim | Description |
|---|---|
| `iss` | JWT issuer |
| `sub` | IBM Verify Cloud Directory username |
| `iat` | Issued-at Unix timestamp |
| `exp` | Expiration Unix timestamp |
| `jti` | Unique JWT identifier (prevents replay attacks) |

The JWT header includes:

| Header | Value |
|---|---|
| `alg` | `RS256` |
| `kid` | Certificate label in IBM Verify — must match exactly |

Inputs:

| Attribute | Type | Description |
|---|---|---|
| `issuer` | string | Value placed in the `iss` claim |
| `subject` | string | IBM Verify username placed in the `sub` claim |
| `key_id` | string | Certificate label in IBM Verify, placed in the `kid` header |
| `jwt_id` | string | Unique value placed in the `jti` claim |
| `private_key_pem` | string (sensitive) | RSA private key used to sign the JWT |
| `expires_in_seconds` | number | JWT lifetime in seconds (900 = 15 minutes) |

Outputs:

| Attribute | Description |
|---|---|
| `token` | Signed JWT string (sensitive) |
| `issued_at` | Unix timestamp of signing time |
| `expires_at` | Unix timestamp of expiry |

---

### `verify_token_exchange`

Exchanges the signed JWT for an IBM Verify access token using OAuth 2.0 Token Exchange (RFC 8693).

Example:

```hcl
resource "verify_token_exchange" "example" {
  tenant_url         = var.verify_tenant_url
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = verify_jwt.custom.token
  subject_token_type = "urn:demo:token-type:user-jwt"
}
```

The `subject_token_type` value must match the custom token type configured in your IBM Verify
STS client under **Security → API clients → Token exchange settings**. The default is
`urn:demo:token-type:user-jwt`. Other tenants may use the RFC 8693 standard value
`urn:ietf:params:oauth:token-type:jwt`.

The provider sends a request equivalent to:

```bash
curl 'https://<tenant>.verify.ibm.com/oauth2/token' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'client_id=<sts_client_id>' \
  --data-urlencode 'client_secret=<sts_client_secret>' \
  --data-urlencode 'grant_type=urn:ietf:params:oauth:grant-type:token-exchange' \
  --data-urlencode 'subject_token=<jwt>' \
  --data-urlencode 'subject_token_type=urn:demo:token-type:user-jwt'
```

Inputs:

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string | IBM Verify tenant base URL, e.g. `https://example.verify.ibm.com` |
| `client_id` | string | UUID of the IBM Verify STS API client |
| `client_secret` | string (sensitive) | Secret for the STS API client |
| `subject_token` | string (sensitive) | The signed JWT to exchange |
| `subject_token_type` | string (optional) | Token type URN — defaults to `urn:demo:token-type:user-jwt` |

Outputs:

| Attribute | Description |
|---|---|
| `access_token` | IBM Verify access token (sensitive) |
| `expires_in` | Token lifetime in seconds |
| `grant_id` | IBM Verify grant identifier |
| `issued_token_type` | `urn:ietf:params:oauth:token-type:access_token` |
| `scope` | Granted scopes (empty if none configured on the STS client) |
| `token_type` | `bearer` |

> **Note:** Do not append `/oauth2/token` to `tenant_url`. The provider appends the
> endpoint path automatically.

---

### `data.verify_token_introspection`

Introspects an IBM Verify access token. This is a **data source**, not a resource —
it is re-evaluated on every `terraform plan` and `terraform apply` so the result always
reflects the live token status.

Example:

```hcl
data "verify_token_introspection" "example" {
  tenant_url    = var.verify_tenant_url
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = verify_token_exchange.example.access_token
}
```

The provider sends a request equivalent to:

```bash
curl 'https://<tenant>.verify.ibm.com/oauth2/introspect' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'client_id=<sts_client_id>' \
  --data-urlencode 'client_secret=<sts_client_secret>' \
  --data-urlencode 'token=<access_token>'
```

Inputs:

| Attribute | Type | Description |
|---|---|---|
| `tenant_url` | string | IBM Verify tenant base URL |
| `client_id` | string | UUID of the IBM Verify STS API client |
| `client_secret` | string (sensitive) | Secret for the STS API client |
| `token` | string (sensitive) | Access token to introspect |

Outputs:

| Attribute | Description |
|---|---|
| `active` | Whether IBM Verify considers the token valid right now |
| `subject` | IBM Verify internal user ID (e.g. `6430083CO1`) |
| `preferred_username` | OIDC login name (e.g. `Bretton`) |
| `username` | Cloud Directory username — only if configured in STS response attributes |
| `name` | Full display name — only if configured in STS response attributes |
| `given_name` | First name — only if configured in STS response attributes |
| `scope` | Granted scopes |
| `token_type` | Token type |
| `issuer` | Token issuer |
| `issued_at` | Unix timestamp of token issue time |
| `expires_at` | Unix timestamp of token expiry |

> **Note:** Fields such as `username`, `name`, and `given_name` are only populated if the
> IBM Verify STS client is configured to include those attributes in the token response.
> Configure them under **Security → API clients → your STS client → Token exchange → Attributes**.

---

## Terraform variables

Create `examples/terraform.tfvars`:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"
sts_client_id     = "YOUR-STS-CLIENT-UUID"
sts_client_secret = "YOUR-STS-CLIENT-SECRET"
```

Never commit this file. It is listed in `.gitignore`.

## Local provider development

Build the provider binary from the repository root:

```bash
go fmt ./...
go mod tidy
go build -o terraform-provider-verify .
```

Configure Terraform to use the local binary by adding this to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "christian-verderame/verify" = "/Users/chris_main/Downloads/terraform-provider-verify"
  }

  direct {}
}
```

The path must point to the directory containing the `terraform-provider-verify` binary.
The address must match the provider address in `main.go`.

After changing any Go code, rebuild before running Terraform:

```bash
go build -o terraform-provider-verify .
```

You do not need to run `terraform init` again after rebuilding.

## Running the example

### Step 1 — Generate the certificate (once only)

```bash
cd examples/cert
terraform init
terraform apply -auto-approve
```

This writes `cert.pem` and `key.pem` into `examples/`.

### Step 2 — Upload the certificate to IBM Verify (once only)

1. Go to **Security → API clients** in your IBM Verify tenant
2. Open your STS client
3. Under **JWT validation / Certificate**, upload the contents of `cert.pem`
4. Set the certificate label to match the `key_id` in `main.tf` (e.g. `demotokensigner`)
5. Save

### Step 3 — Apply the main configuration

```bash
cd examples
terraform apply -auto-approve
```

View the access token:

```bash
terraform output -raw access_token
```

View the signed JWT:

```bash
terraform output -raw jwt
```

Check whether the token is active:

```bash
terraform output token_active
```

Inspect the certificate on disk:

```bash
openssl x509 -in cert.pem -text -noout
```

### Destroying and reapplying

When you run `terraform destroy` in `examples/`, the certificate in `examples/cert/` is
**not** affected. The key pair on disk remains stable and IBM Verify's trust relationship
is preserved. A fresh `terraform apply` will generate a new JWT and exchange it without
needing to re-upload the certificate.

Only destroy `examples/cert/` if you intentionally want to rotate the certificate — in
that case you must also re-upload the new `cert.pem` to IBM Verify before the next apply.

## Refreshing an expired JWT

The JWT is valid for 15 minutes (`expires_in_seconds = 900`). Because all JWT inputs are
unchanged between runs, Terraform will not recreate it automatically. To force a fresh JWT
and token exchange:

```bash
cd examples
terraform destroy -target=verify_token_exchange.example -auto-approve
terraform destroy -target=verify_jwt.custom -auto-approve
terraform destroy -target=random_uuid.jwt_id -auto-approve
terraform apply -auto-approve
```

Or use `-replace` to do it in a single apply:

```bash
terraform apply \
  -replace=random_uuid.jwt_id \
  -replace=verify_jwt.custom \
  -replace=verify_token_exchange.example \
  -auto-approve
```

Confirm the new timestamps:

```bash
terraform output jwt_issued_at
terraform output jwt_expires_at
```

On macOS, convert to human-readable time:

```bash
date -r "$(terraform output -raw jwt_issued_at)"
date -r "$(terraform output -raw jwt_expires_at)"
```

## Common errors

### Provider does not support resource type

```text
The provider does not support the resource type "verify_jwt".
```

The provider binary is out of date. Rebuild it:

```bash
go build -o terraform-provider-verify .
```

### `invalid_client` (HTTP 401)

```text
IBM Verify token exchange failed with HTTP 401: invalid_client:
CSIAQ0155E The requested OAuth 2.0 Client could not be authenticated.
```

Check:

- The `client_id` is the exact UUID from the IBM Verify STS application (no extra characters)
- The `client_secret` matches exactly (no leading or trailing characters)
- The credentials belong to an STS client, not a regular API client
- The tenant URL is correct

### `unsupported_token_type` (HTTP 400)

```text
IBM Verify token exchange failed with HTTP 400: invalid_request:
"urn:ietf:params:oauth:token-type:jwt" token type is not supported as a "subject_token_type".
```

Your STS client uses a custom token type URN. Set `subject_token_type` explicitly in `main.tf`:

```hcl
resource "verify_token_exchange" "example" {
  ...
  subject_token_type = "urn:demo:token-type:user-jwt"
}
```

The exact value must match what is configured in your IBM Verify STS client's token exchange settings.

### `CSIAQ5212E` — Unable to verify token integrity (HTTP 400)

```text
IBM Verify token exchange failed with HTTP 400: invalid_request:
CSIAQ5212E Unable to verify the integrity of the token.
```

IBM Verify cannot verify the JWT signature. This means the certificate uploaded to the STS
client does not match the private key used to sign the JWT. This happens when:

- `examples/cert/` was destroyed and recreated, generating a new key pair
- The new `cert.pem` was not re-uploaded to IBM Verify

Fix: re-upload `examples/cert.pem` to your IBM Verify STS client.

### JSON decode error beginning with `<`

```text
invalid character '<' looking for beginning of value
```

The provider received an HTML error page instead of a JSON response. The tenant URL is
incorrect or includes the endpoint path. Use:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"
```

Not:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com/oauth2/token"
```

### `token_active = false` after apply

The JWT in state is expired. Terraform did not regenerate it because no inputs changed.
Run the refresh commands in [Refreshing an expired JWT](#refreshing-an-expired-jwt).

## Security

Sensitive values are hidden from Terraform plan and apply output but are still stored in
Terraform state in plaintext. Protect the following files:

- `terraform.tfstate` and all backup copies
- `terraform.tfvars`
- `examples/key.pem`
- Any file containing a JWT, access token, or STS client secret

The `.gitignore` in this repository excludes all of the above. Never commit them.

If a client secret is exposed, rotate it immediately in IBM Verify under
**Security → API clients → your STS client → Regenerate secret**.

## Current limitations

- JWTs and access tokens are stored in Terraform state
- JWT resources do not automatically regenerate when they expire — manual destroy/apply required
- The certificate is self-signed — IBM Verify trusts it only because it was explicitly uploaded
- Certificate upload to IBM Verify is not yet automated
- STS client creation is not yet automated
- Custom token type creation is not yet automated
- Token revocation is not yet implemented
