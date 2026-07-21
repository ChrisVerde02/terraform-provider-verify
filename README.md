# Terraform Provider for IBM Verify

A custom Terraform provider that automates parts of the IBM Verify external identity provider token-exchange workflow.

The provider currently supports:

- Generating a self-signed RSA X.509 certificate
- Generating an RS256-signed custom JWT
- Exchanging the custom JWT for an IBM Verify access token
- Writing the generated certificate and private key to local files through the HashiCorp `local` provider

Planned features:

- Introspecting the IBM Verify access token
- Creating an STS client
- Creating a custom token type
- Uploading the signer certificate to IBM Verify

## Workflow

```text
Terraform
    |
    v
Generate RSA private key and X.509 certificate
    |
    v
Generate custom RS256 JWT
    |
    v
POST JWT to IBM Verify /oauth2/token
    |
    v
Receive IBM Verify access token
    |
    v
Introspect access token
```

## Project structure

```text
terraform-provider-verify/
├── go.mod
├── go.sum
├── main.go
├── README.md
├── internal/
│   ├── client/
│   │   └── token_exchange.go
│   ├── crypto/
│   │   ├── certificate.go
│   │   └── jwt.go
│   ├── provider/
│   │   └── provider.go
│   └── resources/
│       ├── certificate_resource.go
│       ├── jwt_resource.go
│       └── token_exchange_resource.go
└── examples/
    ├── main.tf
    ├── terraform.tfvars
    └── .terraform.lock.hcl
```

## Requirements

- Go 1.26 or later
- Terraform CLI
- OpenSSL, for inspecting generated certificates
- An IBM Verify tenant
- A configured IBM Verify STS client
- A configured IBM Verify custom token type
- A signer certificate uploaded to IBM Verify

## Provider resources

### `verify_certificate`

Generates an RSA private key and a self-signed X.509 certificate.

Example:

```hcl
resource "verify_certificate" "example" {
  common_name   = "DemoTokenSigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}
```

Outputs:

- `certificate_pem`
- `private_key_pem`

The private key is marked sensitive, but it is still stored in Terraform state.

### `verify_jwt`

Generates an RS256-signed JWT using the private key created by `verify_certificate`.

Example:

```hcl
resource "random_uuid" "jwt_id" {}

resource "verify_jwt" "custom" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  jwt_id              = random_uuid.jwt_id.result
  private_key_pem     = verify_certificate.example.private_key_pem
  expires_in_seconds  = 900
}
```

The generated JWT includes:

```text
iss  JWT issuer
sub  IBM Verify Cloud Directory username
iat  Current Unix timestamp
exp  Expiration Unix timestamp
jti  Unique JWT identifier
```

The JWT header includes:

```text
alg = RS256
kid = certificate label in IBM Verify
```

The `key_id` value must exactly match the signer-certificate label configured in IBM Verify.

### `verify_token_exchange`

Exchanges the generated JWT for an IBM Verify access token.

Example:

```hcl
resource "verify_token_exchange" "example" {
  tenant_url    = var.verify_tenant_url
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  subject_token = verify_jwt.custom.token
}
```

The provider sends a form-encoded request equivalent to:

```bash
curl 'https://<tenant>.verify.ibm.com/oauth2/token' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'client_id=<sts_client_id>' \
  --data-urlencode 'client_secret=<sts_client_secret>' \
  --data-urlencode 'grant_type=urn:ietf:params:oauth:grant-type:token-exchange' \
  --data-urlencode 'subject_token=<jwt>' \
  --data-urlencode 'subject_token_type=urn:demo:token-type:user-jwt'
```

The `tenant_url` input must contain only the tenant base URL:

```hcl
verify_tenant_url = "https://example.verify.ibm.com"
```

Do not append `/oauth2/token`. The provider appends that endpoint automatically.

## Example Terraform configuration

```hcl
terraform {
  required_providers {
    verify = {
      source = "christian-verderame/verify"
    }

    local = {
      source = "hashicorp/local"
    }

    random = {
      source = "hashicorp/random"
    }
  }
}

provider "verify" {}

resource "verify_certificate" "example" {
  common_name   = "DemoTokenSigner"
  organization  = "IBM"
  country       = "US"
  validity_days = 365
  key_size      = 4096
}

resource "local_file" "certificate" {
  content         = verify_certificate.example.certificate_pem
  filename        = "${path.module}/cert.pem"
  file_permission = "0644"
}

resource "local_sensitive_file" "private_key" {
  content         = verify_certificate.example.private_key_pem
  filename        = "${path.module}/key.pem"
  file_permission = "0600"
}

resource "random_uuid" "jwt_id" {}

resource "verify_jwt" "custom" {
  issuer             = "https://demo.ibm.com"
  subject            = "bretton"
  key_id             = "demotokensigner"
  jwt_id              = random_uuid.jwt_id.result
  private_key_pem     = verify_certificate.example.private_key_pem
  expires_in_seconds  = 900
}

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

resource "verify_token_exchange" "example" {
  tenant_url    = var.verify_tenant_url
  client_id     = var.sts_client_id
  client_secret = var.sts_client_secret
  subject_token = verify_jwt.custom.token
}

output "certificate_path" {
  value = local_file.certificate.filename
}

output "private_key_path" {
  value = local_sensitive_file.private_key.filename
}

output "jwt" {
  value     = verify_jwt.custom.token
  sensitive = true
}

output "jwt_issued_at" {
  value = verify_jwt.custom.issued_at
}

output "jwt_expires_at" {
  value = verify_jwt.custom.expires_at
}

output "access_token" {
  value     = verify_token_exchange.example.access_token
  sensitive = true
}

output "access_token_expires_in" {
  value = verify_token_exchange.example.expires_in
}

output "issued_token_type" {
  value = verify_token_exchange.example.issued_token_type
}
```

## Terraform variables

Create `examples/terraform.tfvars`:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"
sts_client_id     = "YOUR-STS-CLIENT-ID"
sts_client_secret = "YOUR-STS-CLIENT-SECRET"
```

Never commit this file.

## Local provider development

Build the provider from the repository root:

```bash
go fmt ./...
go mod tidy
go build ./...
go build -o terraform-provider-verify
```

Configure Terraform to use the local provider binary in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/christian-verderame/verify" = "/Users/chris_main/Downloads/terraform-provider-verify"
  }

  direct {}
}
```

The address must match the provider address in `main.go`.

After changing Go code, rebuild the provider binary before running Terraform again:

```bash
go build -o terraform-provider-verify
```

You normally do not need to run `terraform init` again after editing provider code.

## Running the example


From the provider root:

```bash
go fmt ./...
go mod tidy
go build ./...
go build -o terraform-provider-verify
```

Then:

if you haven't uploaded or created a certification then:
```bash
cd examples/cert
terraform validate
terraform plan
terraform apply
```
this will create the cert.pem, and key.pem files in the examples folder which you can upload to IBM verify tenant

then you can:

```bash
cd examples
terraform validate
terraform plan
terraform apply
```

Enter:

```text
yes
```

View the generated JWT:

```bash
terraform output -raw jwt
```

View the access token:

```bash
terraform output -raw access_token
```

Check generated files:

```bash
ls -l cert.pem key.pem
```

Inspect the certificate:

```bash
openssl x509 -in cert.pem -text -noout
```

Destroy the Terraform-managed resources and local files:

```bash
terraform destroy
```

## Refreshing an expired JWT

The JWT is generated only when the `verify_jwt` resource is created. With:

```hcl
expires_in_seconds = 900
```

the JWT is valid for 15 minutes.

To force Terraform to generate a new JTI and JWT:

```bash
terraform apply \
  -replace=random_uuid.jwt_id \
  -replace=verify_jwt.custom
```

Confirm the timestamps:

```bash
terraform output jwt_issued_at
terraform output jwt_expires_at
```

On macOS:

```bash
date -r "$(terraform output -raw jwt_issued_at)"
date -r "$(terraform output -raw jwt_expires_at)"
```

## Common errors

### Provider does not support `verify_jwt`

Make sure `provider.go` registers the resource:

```go
func (p *VerifyProvider) Resources(
    ctx context.Context,
) []func() resource.Resource {
    return []func() resource.Resource{
        resources.NewCertificateResource,
        resources.NewJWTResource,
        resources.NewTokenExchangeResource,
    }
}
```

Then rebuild:

```bash
go build -o terraform-provider-verify
```

### `invalid_client`

Example:

```text
HTTP 401
invalid_client
The requested OAuth 2.0 Client could not be authenticated
```

Check:

- The credentials belong to an IBM Verify STS client
- The STS client exists in the same tenant
- The client secret has not been regenerated
- There are no leading or trailing spaces
- The tenant base URL is correct
- The client ID and secret belong to the same client

### JSON decode error beginning with `<`

Example:

```text
invalid character '<' looking for beginning of value
```

This usually means the provider received an HTML page because the endpoint URL was incorrect.

Use:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com"
```

Do not use:

```hcl
verify_tenant_url = "https://YOUR-TENANT.verify.ibm.com/oauth2/token"
```

The provider adds `/oauth2/token` automatically.

### JWT expired

Generate a fresh token:

```bash
terraform apply \
  -replace=random_uuid.jwt_id \
  -replace=verify_jwt.custom
```

## Security

Sensitive values are hidden from normal Terraform output, but they are still stored in Terraform state.

Protect:

- `terraform.tfstate`
- `terraform.tfvars`
- `key.pem`
- JWTs
- Access tokens
- STS client secrets

Recommended `.gitignore`:

```gitignore
.terraform/
*.tfstate
*.tfstate.*
terraform.tfvars
*.auto.tfvars
key.pem
cert.pem
terraform-provider-verify
```

Never commit a private key, STS secret, JWT, or access token.

If a client secret is exposed, rotate it immediately in IBM Verify.

## Current limitations

- JWTs are stored in Terraform state
- Access tokens are stored in Terraform state
- JWT resources do not automatically regenerate when they expire
- The certificate is currently self-signed
- Certificate upload to IBM Verify is not yet automated
- STS client creation is not yet automated
- Custom token type creation is not yet automated
- Token introspection is not yet implemented

## Next planned resource

The next resource will call IBM Verify’s introspection endpoint:

```text
verify_token_introspection
```

It will accept the access token returned by `verify_token_exchange` and report whether the token is active, its expiration, scope, subject, and client information.