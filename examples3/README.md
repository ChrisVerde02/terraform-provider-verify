# examples3 — New Resources Demo

Demonstrates the three new IBM Verify resources added in v0.6.0–v0.8.0:

| Resource | What it creates |
|---|---|
| `verify_api_client` | A Dynamic Client Registration (DCR) API client |
| `verify_user` | A Cloud Directory user via SCIM v2 |
| `verify_application` | An IBM Verify application from a template |

All three resources use a single STS client credential set —
no separate cert-manager or app-specific credentials are required.

---

## Prerequisites

1. An IBM Verify tenant
2. An API client with entitlements: `manageApiClients`, `manageUsers`, `manageApplications`
3. An application template ID (find it in IBM Verify: **Applications → Add application**)

---

## Quick start

```bash
cd examples3

# 1. Fill in your credentials
cp terraform.tfvars terraform.tfvars.local
# edit terraform.tfvars (already git-ignored)

# 2. Point Terraform at your local provider build
# Add to ~/.terraformrc:
# provider_installation {
#   dev_overrides { "ChrisVerde02/verify" = "/path/to/terraform-provider-verify" }
#   direct {}
# }

# 3. Apply
terraform init
terraform apply
```

---

## What gets created

```
verify_api_client.demo   → clientId + clientSecret stored in state
verify_user.demo         → user_id stored in state
verify_application.demo  → application_id stored in state
```

---

## Outputs

| Output | Description |
|---|---|
| `api_client_id` | Generated client ID |
| `api_client_secret` | Generated client secret (sensitive) |
| `api_client_enabled` | Whether the client is active |
| `user_id` | SCIM user ID |
| `user_display_name` | Display name returned by IBM Verify |
| `application_id` | Application UUID |
| `application_state` | `"true"` = active, `"false"` = draft |

---

## Teardown

```bash
terraform destroy
```

This permanently removes all three resources from IBM Verify.

---

## Security

`terraform.tfvars` and `terraform.tfstate` are git-ignored — they contain credentials and secrets.
Never commit either file.
