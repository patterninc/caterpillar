//go:build integration

package file_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file"
)

const s3SettleTimeout = 10 * time.Second

// s3Integration skips unless an endpoint override is set, so the integration
// tag on its own can never reach real AWS. The file task builds a default
// virtual-hosted S3 client, which LocalStack only serves on its
// s3.localhost.localstack.cloud hostname, hence the S3-specific override.
func s3Integration(t *testing.T) (*s3.Client, string) {

	t.Helper()

	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is unset; integration tests run only against LocalStack")
	}
	if os.Getenv("AWS_ENDPOINT_URL_S3") == "" {
		t.Fatal("AWS_ENDPOINT_URL_S3 is unset: the file task uses virtual-hosted S3 addressing, so point it at http://s3.localhost.localstack.cloud:4566")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }), region

}

// s3Bucket creates a uniquely named bucket and removes it, with everything in
// it, when the test ends.
func s3Bucket(t *testing.T, client *s3.Client) string {

	t.Helper()

	ctx := context.Background()
	bucket := "caterpillar-it-" + uuid.NewString()[:13]

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("creating bucket %s: %v", bucket, err)
	}

	t.Cleanup(func() {
		pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				t.Errorf("listing bucket %s for cleanup: %v", bucket, err)
				return
			}
			for _, obj := range page.Contents {
				if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: obj.Key}); err != nil {
					t.Errorf("deleting s3://%s/%s: %v", bucket, aws.ToString(obj.Key), err)
				}
			}
		}
		if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Errorf("deleting bucket %s: %v", bucket, err)
		}
	})

	return bucket

}

func s3Object(t *testing.T, client *s3.Client, bucket, key string) []byte {

	t.Helper()

	out, err := client.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("getting s3://%s/%s: %v", bucket, key, err)
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("reading s3://%s/%s: %v", bucket, key, err)
	}

	return body

}

func newS3FileTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := file.New()
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

func TestIntegrationS3Write(t *testing.T) {

	client, region := s3Integration(t)
	bucket := s3Bucket(t, client)

	tk := newS3FileTask(t, fmt.Sprintf(`
name: write
type: file
path: s3://%s/out/data.txt
region: %s
success_file: true
tags:
  env: integration
`, bucket, region))

	a := ack.New()
	a.AddBranch(1)
	input := make(chan *record.Record, 1)
	input <- &record.Record{ID: 1, Data: []byte("hello\nworld\n"), Context: ack.WithContext(context.Background(), a)}
	close(input)

	if err := tk.Run(input, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	select {
	case <-a.Wait():
		if a.Failed() {
			t.Error("record was rejected after a successful write")
		}
	case <-time.After(s3SettleTimeout):
		t.Fatal("record never settled")
	}

	if got := s3Object(t, client, bucket, "out/data.txt"); !bytes.Equal(got, []byte("hello\nworld\n")) {
		t.Errorf("object body = %q, want %q", got, "hello\nworld\n")
	}

	if got := s3Object(t, client, bucket, "out/_SUCCESS"); len(got) != 0 {
		t.Errorf("_SUCCESS body = %q, want empty", got)
	}

	tagging, err := client.GetObjectTagging(context.Background(), &s3.GetObjectTaggingInput{Bucket: aws.String(bucket), Key: aws.String("out/data.txt")})
	if err != nil {
		t.Fatalf("GetObjectTagging: %v", err)
	}
	tags := map[string]string{}
	for _, tag := range tagging.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	if tags["env"] != "integration" || len(tags) != 1 {
		t.Errorf("object tags = %v, want map[env:integration]", tags)
	}

}

func TestIntegrationS3Read(t *testing.T) {

	client, region := s3Integration(t)
	bucket := s3Bucket(t, client)

	objects := map[string]string{
		"in/a.txt":      "alpha",
		"in/b.txt":      "bravo",
		"in/skip.csv":   "not matched by the glob",
		"other/c.txt":   "outside the prefix",
		"in/deep/d.txt": "single star does not descend",
	}
	for key, body := range objects {
		if _, err := client.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte(body)),
		}); err != nil {
			t.Fatalf("putting s3://%s/%s: %v", bucket, key, err)
		}
	}

	tk := newS3FileTask(t, fmt.Sprintf(`
name: read
type: file
path: s3://%s/in/*.txt
region: %s
`, bucket, region))

	output := make(chan *record.Record, 16)
	if err := tk.Run(nil, output); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(output)

	var bodies, names []string
	for r := range output {
		bodies = append(bodies, string(r.Data))
		name, _ := r.GetContextValue(string(task.CtxKeyFileNameWrite))
		names = append(names, name)
	}
	sort.Strings(bodies)
	sort.Strings(names)

	if want := []string{"alpha", "bravo"}; fmt.Sprint(bodies) != fmt.Sprint(want) {
		t.Errorf("emitted bodies = %q, want %q", bodies, want)
	}
	if want := []string{"a.txt", "b.txt"}; fmt.Sprint(names) != fmt.Sprint(want) {
		t.Errorf("file name context = %q, want %q", names, want)
	}

}

func TestIntegrationS3ReadNoMatch(t *testing.T) {

	client, region := s3Integration(t)
	bucket := s3Bucket(t, client)

	tk := newS3FileTask(t, fmt.Sprintf(`
name: read
type: file
path: s3://%s/missing/*.txt
region: %s
`, bucket, region))

	output := make(chan *record.Record, 1)
	if err := tk.Run(nil, output); err == nil {
		t.Error("Run on a glob matching nothing returned nil, want a no files found error")
	}

}
