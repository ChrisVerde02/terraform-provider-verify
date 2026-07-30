# IBM Verify tenant URL
verify_tenant_url = "https://<tenant>.verify.ibm.com"

# STS API client credentials (token exchange — impersonates user via JWT)
sts_client_id     = ""
sts_client_secret = ""

# cert-manager API client credentials (client credentials — uploads signer certs)
cert_manager_client_id     = ""
cert_manager_client_secret = ""

# Certificate settings
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
jwt_subject = ""
# IBM Verify lowercases all cert labels on storage — kid must match exactly
jwt_key_id  = "demotokensigner"

# Token type URN configured in your IBM Verify STS client
subject_token_type = "urn:demo:token-type:user-jwt"