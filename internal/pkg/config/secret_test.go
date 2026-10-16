package config

import (
	"testing"
)

func TestAssumeRoleARN(t *testing.T) {
	t.Setenv(ssmAssumeRoleEnv, `arn:aws:iam::842676014003:role/heimdall-caterpillar-ssm-read`)

	if got := assumeRoleARN(`/caterpillar/github/app_jwt`); got != `arn:aws:iam::842676014003:role/heimdall-caterpillar-ssm-read` {
		t.Fatalf("caterpillar path with env: got %q", got)
	}
	if got := assumeRoleARN(`/heimdall/github_svc_account_token`); got != `` {
		t.Fatalf("heimdall path should not assume: got %q", got)
	}

	t.Setenv(ssmAssumeRoleEnv, ``)
	if got := assumeRoleARN(`/caterpillar/github/app_jwt`); got != `` {
		t.Fatalf("unset env should use task role: got %q", got)
	}
}
