# terraform-provider-verify

Terraform provider for [IBM Verify](https://www.ibm.com/products/verify-identity). Manages the full JWT token-exchange flow as Terraform-managed resources — certificates auto-renew, JWTs auto-refresh, and access tokens auto-re-exchange when they expire.

Published on the [Terraform Registry](https://registry.terraform.io/providers/ChrisVerde02/verify/latest).

Built on top of [`ibmverify-go`](https://github.com/ChrisVerde02/ibmverify-go).

## Usage

```hcl
terraform {
  required_providers {
    verify = {
      source  = "ChrisVerde02/verify"
      version = "0.4.3"
    }
  }
}

provider "verify" {}
```

## Resources

### `verify_certificate`

Generates a self-signed RSA certificate and private key. Stored in state and automatically regenerated 24 hours before expiry.

```hcl
resource "verify_certificate" "this" {
  common_name   = "DemoTokenSigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}
# verify_certificate.this.certificate_pem
# verify_certificate.this.private_key_pem
```

### `verify_signercert`

Uploads a PEM certificate to IBM Verify as a signer certificate. Obtains its own client credentials token internally — no `access_token` needed in config. Idempotent: adopts an existing cert if state was lost.

```hcl
resource "verify_signercert" "this" {
  tenant_url                 = "https://example.verify.ibm.com"
  cert_manager_client_id     = var.cert_manager_client_id
  cert_manager_client_secret = var.cert_manager_client_secret
  certificate_pem            = verify_certificate.this.certificate_pem
  label                      = "DemoTokenSigner"
}
```

### `verify_jwt`

Signs an RS256 JWT. Stored in state and automatically regenerated with a fresh `jti` 60 seconds before expiry to prevent IBM Verify replay rejection.

```hcl
resource "verify_jwt" "this" {
  issuer             = "https://demo.ibm.com"
  subject            = "myusername"
  key_id             = "DemoTokenSigner"
  private_key_pem    = verify_certificate.this.private_key_pem
  expires_in_seconds = 900

  depends_on = [verify_signercert.this]
}
# verify_jwt.this.token
```

### `verify_token_exchange`

Exchanges a JWT for an IBM Verify access token (RFC 8693). Stored in state and automatically re-exchanged 60 seconds before expiry.

```hcl
resource "verify_token_exchange" "this" {
  tenant_url         = "https://example.verify.ibm.com"
  client_id          = var.sts_client_id
  client_secret      = var.sts_client_secret
  subject_token      = verify_jwt.this.token
  subject_token_type = "urn:ietf:params:oauth:token-type:jwt"
}
# verify_token_exchange.this.access_token
# verify_token_exchange.this.expires_in
```

### `verify_token_introspection` (resource)

Introspects a token once at create time and stores the result in state.

```hcl
resource "verify_token_introspection" "this" {
  tenant_url    = "https://example.verify.ibm.com"
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = verify_token_exchange.this.access_token
}
```

## Data Sources

### `verify_client_credentials_token`

Fetches a fresh client credentials access token on every plan and apply.

```hcl
data "verify_client_credentials_token" "this" {
  tenant_url    = "https://example.verify.ibm.com"
  client_id     = var.client_id
  client_secret = var.client_secret
}
# data.verify_client_credentials_token.this.access_token
```

### `verify_token_introspection` (data source)

Introspects a token on every plan and apply — result always reflects live token status.

```hcl
data "verify_token_introspection" "this" {
  tenant_url    = "https://example.verify.ibm.com"
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  token         = var.access_token
}
# data.verify_token_introspection.this.active
# data.verify_token_introspection.this.subject
# data.verify_token_introspection.this.expires_at
```

## Modules

Reusable modules are in the `modules/` directory:

| Module | Wraps |
|---|---|
| `modules/certificate` | `verify_certificate` + writes PEM files to disk |
| `modules/jwt` | `verify_jwt` |
| `modules/token_exchange` | `verify_token_exchange` |
| `modules/introspection` | `data.verify_token_introspection` |

## IBM Verify setup

Two API clients are required in your IBM Verify tenant:

| Client | Required entitlement | Used for |
|---|---|---|
| STS client | Token exchange | Exchanging a JWT for an access token, introspection |
| Cert-manager client | `manageCerts` | Uploading signer certificates via `verify_signercert` |

## Requirements

- Terraform 1.0+
- IBM Verify tenant
