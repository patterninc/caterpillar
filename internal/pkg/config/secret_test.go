package config

import "testing"

func TestAssumeRoleARN(t *testing.T) {
	if got := assumeRoleARN(`/caterpillar/github/app_jwt`); got != dataPlatformCaterpillarSSMRole {
		t.Fatalf("caterpillar path: got %q", got)
	}
	if got := assumeRoleARN(`/heimdall/github_svc_account_token`); got != `` {
		t.Fatalf("heimdall path should not assume: got %q", got)
	}
}
