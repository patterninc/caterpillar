#!/usr/bin/env bash
# Verifies AWS credentials are configured and the account is a sandbox/dev environment.
# Must pass before any pipeline runs against live AWS resources.
# Usage: source .claude/scripts/ensure-sandbox.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "============================================"
echo "  AWS Sandbox Environment Check"
echo "============================================"
echo ""

# --- 1. Check AWS credentials exist ---
echo -n "Checking AWS credentials... "
if ! IDENTITY=$(aws sts get-caller-identity 2>&1); then
  echo -e "${RED}FAILED${NC}"
  echo ""
  echo "No valid AWS credentials found. Set up credentials using one of:"
  echo ""
  echo "  Option 1: aws configure"
  echo "  Option 2: export AWS_ACCESS_KEY_ID=... && export AWS_SECRET_ACCESS_KEY=..."
  echo "  Option 3: aws sso login --profile <sandbox-profile>"
  echo ""
  exit 1
fi
echo -e "${GREEN}OK${NC}"

ACCOUNT_ID=$(echo "$IDENTITY" | python3 -c "import sys,json; print(json.load(sys.stdin)['Account'])")
ARN=$(echo "$IDENTITY" | python3 -c "import sys,json; print(json.load(sys.stdin)['Arn'])")
echo "  Account: $ACCOUNT_ID"
echo "  ARN:     $ARN"
echo ""

# --- 2. Check region ---
echo -n "Checking AWS region... "
REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-}}"
if [ -z "$REGION" ]; then
  REGION=$(aws configure get region 2>/dev/null || true)
fi
if [ -z "$REGION" ]; then
  echo -e "${RED}FAILED${NC}"
  echo ""
  echo "No AWS region configured. Set it with:"
  echo "  export AWS_REGION=us-east-1"
  echo ""
  exit 1
fi
echo -e "${GREEN}OK${NC}  ($REGION)"
echo ""

# --- 3. Check account is sandbox/dev ---
echo -n "Checking account type... "
ALIASES=$(aws iam list-account-aliases 2>/dev/null | python3 -c "import sys,json; print(' '.join(json.load(sys.stdin).get('AccountAliases',[])))" 2>/dev/null || true)

SANDBOX_PATTERN="sandbox|dev|test|staging|nonprod"
IS_SANDBOX=false

if echo "$ALIASES" | grep -qiE "$SANDBOX_PATTERN"; then
  IS_SANDBOX=true
fi
if echo "$ACCOUNT_ID" | grep -qiE "$SANDBOX_PATTERN"; then
  IS_SANDBOX=true
fi
if echo "$ARN" | grep -qiE "$SANDBOX_PATTERN"; then
  IS_SANDBOX=true
fi

if [ "$IS_SANDBOX" = true ]; then
  echo -e "${GREEN}SANDBOX${NC}"
  if [ -n "$ALIASES" ]; then
    echo "  Alias: $ALIASES"
  fi
  echo ""
  echo -e "${GREEN}============================================${NC}"
  echo -e "${GREEN}  Sandbox environment verified. Safe to run.${NC}"
  echo -e "${GREEN}============================================${NC}"
else
  echo -e "${RED}PRODUCTION (or unknown)${NC}"
  if [ -n "$ALIASES" ]; then
    echo "  Alias: $ALIASES"
  fi
  echo ""
  echo -e "${RED}============================================${NC}"
  echo -e "${RED}  BLOCKED: This account does not appear to${NC}"
  echo -e "${RED}  be a sandbox/dev environment.${NC}"
  echo -e "${RED}${NC}"
  echo -e "${RED}  Pipeline execution is not allowed against${NC}"
  echo -e "${RED}  production AWS accounts.${NC}"
  echo -e "${RED}${NC}"
  echo -e "${RED}  Switch to a sandbox account and retry:${NC}"
  echo -e "${RED}    export AWS_PROFILE=sandbox${NC}"
  echo -e "${RED}============================================${NC}"
  exit 1
fi
