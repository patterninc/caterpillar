Check the current AWS environment and account status. Run these checks and report results:

1. **AWS Identity** — Run `aws sts get-caller-identity` to confirm credentials are valid. Report account ID, ARN, and user/role name.

2. **Account Type** — Check if the account appears to be sandbox/dev or production:
   - Look at the account alias: `aws iam list-account-aliases`
   - Check for Organizations info: `aws organizations describe-organization 2>/dev/null`
   - Flag if the account ID or alias contains "sandbox", "dev", "test", or "staging"

3. **Region** — Report the active region from `AWS_REGION`, `AWS_DEFAULT_REGION`, or `aws configure get region`.

4. **Credential Type** — Report whether using:
   - Environment variables (`AWS_ACCESS_KEY_ID`)
   - Shared credentials file (`~/.aws/credentials`)
   - SSO session (`aws sso login` profile)
   - IAM role (instance/task role)

Report a clear summary table. If any check fails, explain what's missing and how to fix it.
