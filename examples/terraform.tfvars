verify_tenant_url = ""
# The client_id must be the exact UUID from the IBM Verify STS application.
# Previously had a stray leading "Y" which caused IBM Verify to reject the
# client as unknown (invalid_client).
sts_client_id     = ""
# The client_secret must match exactly what is stored in the IBM Verify STS
# application. A stray leading "Y" (same copy-paste error as client_id) caused
# IBM Verify to return CSIAQ0155E invalid_client / authentication failure.
sts_client_secret = ""