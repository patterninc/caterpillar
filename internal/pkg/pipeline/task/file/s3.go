package file

import (
	"fmt"
	"io"
	"net/url"
	"unicode/utf16"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	s3client "github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file/s3_client"
)

const (
	s3Scheme = `s3`

	// S3 object tagging limits (see
	// https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-tagging.html).
	// Lengths are measured in UTF-16 code units.
	s3MaxTagsPerObject = 10
	s3MaxTagKeyLen     = 128
	s3MaxTagValueLen   = 256
)

type s3Reader struct {
	client *s3client.Client
}

func newS3Reader(f *file) (reader, error) {

	c, err := s3client.New(ctx, f.Region)
	if err != nil {
		return nil, err
	}

	return &s3Reader{client: c}, nil

}

func (r *s3Reader) read(path string) (io.ReadCloser, error) {

	bucket, key, err := s3client.ParseURI(path)
	if err != nil {
		return nil, err
	}

	getObjectOutput, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})

	if err != nil {
		return nil, err
	}

	return getObjectOutput.Body, nil

}

func (r *s3Reader) parse(glob string) ([]string, error) {

	bucket, glob, err := s3client.ParseURI(glob)
	if err != nil {
		return nil, err
	}

	objects, err := r.client.GetObjects(ctx, bucket, glob)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(objects))
	for _, object := range objects {
		path := fmt.Sprintf("s3://%s/%s", bucket, *object.Key)
		paths = append(paths, path)
	}

	return paths, nil

}

func writeS3File(f *file, rec *record.Record, reader io.Reader) error {

	// create s3 client
	client, err := s3client.New(ctx, f.Region)
	if err != nil {
		return err
	}

	path, err := f.Path.Get(rec)
	if err != nil {
		return err
	}

	// get bucket and key
	bucket, key, err := s3client.ParseURI(path)
	if err != nil {
		return err
	}

	tags, err := buildTags(f.Tags, rec)
	if err != nil {
		return err
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       &bucket,
		Key:          &key,
		Body:         reader,
		StorageClass: f.StorageClass,
		Tagging:      tags,
	})

	return err

}

// buildTags evaluates each tag value against the record and returns a
// URL-encoded query string (key1=value1&key2=value2) as required by the
// S3 PutObject Tagging header. Returns nil if no tags are configured.
func buildTags(tags map[string]config.String, rec *record.Record) (*string, error) {

	if len(tags) == 0 {
		return nil, nil
	}

	values := make(url.Values, len(tags))
	for k, v := range tags {
		resolved, err := v.Get(rec)
		if err != nil {
			return nil, fmt.Errorf("tag %q: %w", k, err)
		}
		if n := utf16Len(resolved); n > s3MaxTagValueLen {
			return nil, fmt.Errorf("tag %q: value length %d exceeds S3 limit of %d UTF-16 code units", k, n, s3MaxTagValueLen)
		}
		values.Set(k, resolved)
	}

	return aws.String(values.Encode()), nil

}

// validateS3Tags checks the static tag constraints enforced by S3: at most
// 10 tags per object and tag keys up to 128 UTF-16 code units. Uniqueness
// is already guaranteed by the map. Value lengths depend on per-record
// templating and are validated in buildTags.
func validateS3Tags(tags map[string]config.String) error {

	if len(tags) > s3MaxTagsPerObject {
		return fmt.Errorf("tags: %d tags configured, S3 allows at most %d per object", len(tags), s3MaxTagsPerObject)
	}

	for k := range tags {
		if k == "" {
			return fmt.Errorf("tags: empty key is not allowed")
		}
		if n := utf16Len(k); n > s3MaxTagKeyLen {
			return fmt.Errorf("tag %q: key length %d exceeds S3 limit of %d UTF-16 code units", k, n, s3MaxTagKeyLen)
		}
	}

	return nil

}

// utf16Len returns the number of UTF-16 code units that would encode s,
// which is how S3 measures tag key and value lengths.
func utf16Len(s string) int {

	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n

}
