#!/bin/bash
set -e

PROFILE="sandbox"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--profile <aws-profile>]  (default: sandbox)"
      exit 1
      ;;
  esac
done

# Ensure AWS SSO session is active
if aws sts get-caller-identity --profile "$PROFILE" &>/dev/null; then
  echo "AWS SSO session already active for profile: $PROFILE"
else
  echo "AWS SSO session not active, logging in for profile: $PROFILE"
  aws sso login --profile "$PROFILE"
fi

# Export profile for subprocesses
export AWS_PROFILE="$PROFILE"

echo "AWS profile '$PROFILE' is ready."
