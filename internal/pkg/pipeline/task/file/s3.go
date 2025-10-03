package file

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3client "github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file/s3_client"
)

const (
	s3Scheme = `s3`
)

type s3Reader struct {
	client *s3client.Client
}

func newS3Reader(region string) (reader, error) {

	c, err := s3client.New(ctx, region)
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

func writeS3File(f *file, reader io.Reader) error {

	// create s3 client
	client, err := s3client.New(ctx, f.Region)
	if err != nil {
		return err
	}

	path, err := f.Path.Get(f.CurrentRecord)
	if err != nil {
		return err
	}

	// get bucket and key
	bucket, key, err := s3client.ParseURI(path)
	if err != nil {
		return err
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   reader,
	})

	return err

}
