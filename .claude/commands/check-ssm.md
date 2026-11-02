Verify that AWS SSM Parameter Store paths exist and are readable. The user will provide one or more SSM parameter paths.

Run these checks:

1. **Parameter exists** — For each path:
   - `aws ssm get-parameter --name <path> --with-decryption 2>&1`
   - Report: name, type (String/SecureString/StringList), version, last modified date

2. **Path prefix check** — If the user gives a prefix path (e.g. `/prod/kafka/`):
   - `aws ssm get-parameters-by-path --path <prefix> --recursive --max-results 10`
   - List all parameters found under that prefix (names only, not values)

3. **Value preview** — For non-SecureString params, show the value. For SecureString, show `[ENCRYPTED]` and confirm decryption works.

4. **Pipeline implications** — Based on findings:
   - Confirm the paths match what the pipeline uses in `{{ secret "/path" }}`
   - Flag any paths that don't exist — the pipeline will fail at init
   - Note if any are StringList type — may need parsing in the pipeline

Report a clear summary. If access is denied, explain the IAM permissions needed (`ssm:GetParameter`, `ssm:GetParametersByPath`, `kms:Decrypt`).
