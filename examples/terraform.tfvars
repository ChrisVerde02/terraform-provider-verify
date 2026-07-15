verify_tenant_url = "https://ChrisVerde02.verify.ibm.com"
# The client_id must be the exact UUID from the IBM Verify STS application.
# Previously had a stray leading "Y" which caused IBM Verify to reject the
# client as unknown (invalid_client).
sts_client_id     = "9ada0c66-b6e9-4fdb-a6c8-62156f5ae533"
# The client_secret must match exactly what is stored in the IBM Verify STS
# application. A stray leading "Y" (same copy-paste error as client_id) caused
# IBM Verify to return CSIAQ0155E invalid_client / authentication failure.
sts_client_secret = "TvFt1rBA9gMBHYM8W5Rq"