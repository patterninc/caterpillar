//go:build integration

package parameter_store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/aws/parameter_store"
)

const ssmSettleTimeout = 10 * time.Second

// ssmIntegration skips unless an endpoint override is set, so the integration
// tag on its own can never reach real AWS. The task loads its region from the
// environment alone, so AWS_REGION must be set for it as well.
func ssmIntegration(t *testing.T) (*ssm.Client, string) {

	t.Helper()

	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is unset; integration tests run only against LocalStack")
	}
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Fatal("AWS_REGION is unset: the parameter store task takes its region from the environment")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}
	client := ssm.NewFromConfig(cfg)

	prefix := "/caterpillar-it/" + uuid.NewString()
	t.Cleanup(func() {
		out, err := client.GetParametersByPath(context.Background(), &ssm.GetParametersByPathInput{Path: aws.String(prefix), Recursive: aws.Bool(true)})
		if err != nil {
			t.Errorf("listing %s for cleanup: %v", prefix, err)
			return
		}
		for _, p := range out.Parameters {
			if _, err := client.DeleteParameter(context.Background(), &ssm.DeleteParameterInput{Name: p.Name}); err != nil {
				t.Errorf("deleting %s: %v", aws.ToString(p.Name), err)
			}
		}
	})

	return client, prefix

}

func newParameterStoreTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := parameter_store.New()
	if err != nil {
		t.Fatalf("constructing task: %v", err)
	}
	if err := yaml.Unmarshal([]byte(spec), tk); err != nil {
		t.Fatalf("unmarshalling %q: %v", spec, err)
	}
	if err := tk.Init(); err != nil {
		t.Fatalf("initialising task: %v", err)
	}

	return tk

}

func ssmRun(t *testing.T, tk task.Task, data []byte, withOutput bool) ([]*record.Record, *ack.Ack, error) {

	t.Helper()

	a := ack.New()
	a.AddBranch(1)
	input := make(chan *record.Record, 1)
	input <- &record.Record{ID: 1, Data: data, Context: ack.WithContext(context.Background(), a)}
	close(input)

	var output chan *record.Record
	var sendOutput chan<- *record.Record
	if withOutput {
		output = make(chan *record.Record, 4)
		sendOutput = output
	}

	err := tk.Run(input, sendOutput)

	var emitted []*record.Record
	if output != nil {
		close(output)
		for r := range output {
			emitted = append(emitted, r)
		}
	}

	return emitted, a, err

}

func ssmAssertSettled(t *testing.T, a *ack.Ack, wantFailed bool) {

	t.Helper()

	select {
	case <-a.Wait():
		if got := a.Failed(); got != wantFailed {
			t.Errorf("Failed() = %v, want %v", got, wantFailed)
		}
	case <-time.After(ssmSettleTimeout):
		t.Fatal("record never settled")
	}

}

func TestIntegrationParameterStoreSet(t *testing.T) {

	client, prefix := ssmIntegration(t)
	name := prefix + "/token"

	tk := newParameterStoreTask(t, fmt.Sprintf("name: set\ntype: aws_parameter_store\nset:\n  %s: .token\n", name))

	if _, a, err := ssmRun(t, tk, []byte(`{"token":"s3cr3t-value"}`), false); err != nil {
		t.Fatalf("Run: %v", err)
	} else {
		ssmAssertSettled(t, a, false)
	}

	out, err := client.GetParameter(context.Background(), &ssm.GetParameterInput{Name: aws.String(name), WithDecryption: aws.Bool(true)})
	if err != nil {
		t.Fatalf("GetParameter: %v", err)
	}
	if got := aws.ToString(out.Parameter.Value); got != "s3cr3t-value" {
		t.Errorf("parameter value = %q, want s3cr3t-value", got)
	}
	if out.Parameter.Type != types.ParameterTypeSecureString {
		t.Errorf("parameter type = %q, want SecureString (secure defaults to true)", out.Parameter.Type)
	}

	if _, a, err := ssmRun(t, tk, []byte(`{"token":"rotated"}`), false); err != nil {
		t.Fatalf("second Run: %v", err)
	} else {
		ssmAssertSettled(t, a, false)
	}

	out, err = client.GetParameter(context.Background(), &ssm.GetParameterInput{Name: aws.String(name), WithDecryption: aws.Bool(true)})
	if err != nil {
		t.Fatalf("GetParameter after overwrite: %v", err)
	}
	if got := aws.ToString(out.Parameter.Value); got != "rotated" {
		t.Errorf("parameter value after overwrite = %q, want rotated (overwrite defaults to true)", got)
	}

}

func TestIntegrationParameterStoreLookup(t *testing.T) {

	client, prefix := ssmIntegration(t)
	name := prefix + "/db/password"

	if _, err := client.PutParameter(context.Background(), &ssm.PutParameterInput{
		Name:  aws.String(name),
		Value: aws.String("hunter2"),
		Type:  types.ParameterTypeSecureString,
	}); err != nil {
		t.Fatalf("PutParameter: %v", err)
	}

	tk := newParameterStoreTask(t, fmt.Sprintf("name: lookup\ntype: aws_parameter_store\nlookup:\n  db_password: %s\n", name))

	emitted, a, err := ssmRun(t, tk, []byte(`{"id":1}`), true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted %d records, want 1", len(emitted))
	}
	if got, _ := emitted[0].GetContextValue("db_password"); got != "hunter2" {
		t.Errorf("context db_password = %q, want hunter2", got)
	}
	if string(emitted[0].Data) != `{"id":1}` {
		t.Errorf("record data = %q, want it forwarded unchanged", emitted[0].Data)
	}

	ack.Release(emitted[0].Context)
	ssmAssertSettled(t, a, false)

}

func TestIntegrationParameterStoreLookupMissing(t *testing.T) {

	tests := []struct {
		name       string
		onMissing  string
		wantErr    bool
		wantFailed bool
	}{
		{name: "error by default", onMissing: "", wantErr: true, wantFailed: true},
		{name: "skip drops the record", onMissing: "skip", wantErr: false, wantFailed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, prefix := ssmIntegration(t)

			spec := fmt.Sprintf("name: lookup\ntype: aws_parameter_store\nlookup:\n  v: %s/absent\n", prefix)
			if tt.onMissing != "" {
				spec += "on_missing: " + tt.onMissing + "\n"
			}
			tk := newParameterStoreTask(t, spec)

			emitted, a, err := ssmRun(t, tk, []byte(`{}`), true)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run error = %v, want error %v", err, tt.wantErr)
			}
			if len(emitted) != 0 {
				t.Errorf("emitted %d records, want 0", len(emitted))
			}
			ssmAssertSettled(t, a, tt.wantFailed)

		})
	}

}
