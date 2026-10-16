#!/usr/bin/env bash
# Trigger: PostStartup
# Purpose: Verify AWS environment is ready when Claude initializes.
#          Shows account info or warns if SSO session is expired.

set -euo pipefail

PROFILE="${AWS_PROFILE:-sandbox}"

# Check if we can reach AWS
if ! command -v aws &>/dev/null; then
  echo "WARN: aws CLI not installed"
  exit 0
fi

if aws sts get-caller-identity --profile "$PROFILE" &>/dev/null; then
  ACCOUNT_ID=$(aws sts get-caller-identity --profile "$PROFILE" --query 'Account' --output text 2>/dev/null)
  ACCOUNT_ALIAS=$(aws iam list-account-aliases --profile "$PROFILE" --query 'AccountAliases[0]' --output text 2>/dev/null || echo "N/A")
  echo "AWS environment ready — profile: $PROFILE, account: $ACCOUNT_ALIAS ($ACCOUNT_ID)"
else
  echo "WARN: AWS SSO session expired for profile '$PROFILE'. Run: aws sso login --profile $PROFILE"
fi

exit 0
