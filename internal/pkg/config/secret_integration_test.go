//go:build integration

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/google/uuid"

	"github.com/patterninc/caterpillar/internal/pkg/config"
)

// secretIntegration skips unless an endpoint override is set, so the
// integration tag on its own can never reach real AWS.
func secretIntegration(t *testing.T) *ssm.Client {

	t.Helper()

	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is unset; integration tests run only against LocalStack")
	}
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Fatal("AWS_REGION is unset: the secret macro takes its region from the environment")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}

	return ssm.NewFromConfig(cfg)

}

func secretPipelineFile(t *testing.T, body string) string {

	t.Helper()

	path := filepath.Join(t.TempDir(), "pipeline.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	return path

}

func TestIntegrationSecretMacro(t *testing.T) {

	client := secretIntegration(t)

	ctx := context.Background()
	plain := "/caterpillar-it/" + uuid.NewString() + "/plain"
	secure := "/caterpillar-it/" + uuid.NewString() + "/secure"

	for name, p := range map[string]struct {
		value string
		kind  types.ParameterType
	}{
		plain:  {value: "plain-value", kind: types.ParameterTypeString},
		secure: {value: "secure-value", kind: types.ParameterTypeSecureString},
	} {
		if _, err := client.PutParameter(ctx, &ssm.PutParameterInput{Name: aws.String(name), Value: aws.String(p.value), Type: p.kind}); err != nil {
			t.Fatalf("PutParameter %s: %v", name, err)
		}
		t.Cleanup(func() {
			if _, err := client.DeleteParameter(ctx, &ssm.DeleteParameterInput{Name: aws.String(name)}); err != nil {
				t.Errorf("deleting %s: %v", name, err)
			}
		})
	}

	path := secretPipelineFile(t, `plain: '{{ secret "`+plain+`" }}'
secure: '{{ secret "`+secure+`" }}'
`)

	var got map[string]string
	if err := config.Load(path, &got); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got["plain"] != "plain-value" {
		t.Errorf("plain = %q, want plain-value", got["plain"])
	}
	if got["secure"] != "secure-value" {
		t.Errorf("secure = %q, want secure-value (the macro must decrypt SecureString)", got["secure"])
	}

}

func TestIntegrationSecretMacroMissing(t *testing.T) {

	secretIntegration(t)

	path := secretPipelineFile(t, `value: '{{ secret "/caterpillar-it/`+uuid.NewString()+`/absent" }}'
`)

	var got map[string]string
	if err := config.Load(path, &got); err == nil {
		t.Fatalf("Load with a missing parameter returned nil and %v, want an error", got)
	}

}
