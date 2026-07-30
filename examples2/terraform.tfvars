# IBM Verify tenant URL
verify_tenant_url = "https://ChrisVerde02.verify.ibm.com"

# STS API client credentials (token exchange — impersonates user via JWT)
sts_client_id     = "9ada0c66-b6e9-4fdb-a6c8-62156f5ae533"
sts_client_secret = "TvFt1rBA9gMBHYM8W5Rq"

# cert-manager API client credentials (client credentials — uploads signer certs)
cert_manager_client_id     = "8c1e34de-c3ca-4dcb-82f4-5d43ecdcd955"
cert_manager_client_secret = "w6EzRUfpsi5ybCU6IawF"

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
jwt_subject = "bretton"
# IBM Verify lowercases all cert labels on storage — kid must match exactly
jwt_key_id  = "demotokensigner"

# Token type URN configured in your IBM Verify STS client
subject_token_type = "urn:demo:token-type:user-jwt"
