package file

import (
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3client "github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file/s3_client"
)

const (
	s3Scheme = `s3`
)

func getS3Reader(f *file, key string) (io.ReadCloser, error) {

	// get bucket
	bucket, _, err := f.parseS3URI()
	if err != nil {
		return nil, err
	}

	svc, err := s3client.New(ctx, f.Region)
	if err != nil {
		return nil, err
	}

	getObjectOutput, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})

	if err != nil {
		return nil, err
	}

	return getObjectOutput.Body, nil

}

func writeS3File(f *file, reader io.Reader) error {

	// upload file to s3
	svc, err := s3client.New(ctx, f.Region)
	if err != nil {
		return err
	}

	// get bucket and key
	bucket, key, err := f.parseS3URI()
	if err != nil {
		return err
	}

	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   reader,
	})

	return err

}

func (f *file) parseS3URI() (bucket string, key string, err error) {

	path, err := f.Path.Get(f.CurrentRecord)
	if err != nil {
		return ``, ``, err
	}
	parts := strings.SplitN(path[5:], `/`, 2) // f.Path[5:] is to trim `s3://` from the path

	if len(parts) < 2 || parts[0] == `` {
		return ``, ``, fmt.Errorf("invalid S3 URI: %s", f.Path)
	}

	return parts[0], parts[1], nil

}

func getS3KeysFromGlob(f *file) ([]string, error) {

	// get bucket and key
	bucket, glob, err := f.parseS3URI()
	if err != nil {
		return nil, err
	}

	svc, err := s3client.New(ctx, f.Region)
	if err != nil {
		return nil, err
	}

	objects, err := svc.GetObjectsWithGlob(ctx, bucket, glob)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(objects))
	for _, object := range objects {
		keys = append(keys, *object.Key)
	}

	return keys, nil

}
