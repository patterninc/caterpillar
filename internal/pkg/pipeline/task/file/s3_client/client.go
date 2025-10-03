package s3client

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bmatcuk/doublestar"
)

type Client struct {
	*s3.Client
}

func New(ctx context.Context, region string) (*Client, error) {

	// load config with specified region
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	// create client
	return &Client{
		Client: s3.NewFromConfig(awsConfig),
	}, nil

}

func (c *Client) GetObjects(ctx context.Context, bucketName, pattern string) ([]types.Object, error) {

	var matchingObjects []types.Object

	// List all objects in the bucket
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}

	prefix := getRootDir(pattern)
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	paginator := s3.NewListObjectsV2Paginator(c.Client, input)

	for paginator.HasMorePages() {

		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		// TODO: run this in parallel
		objects := match(pattern, output.Contents)
		matchingObjects = append(matchingObjects, objects...)

	}

	return matchingObjects, nil

}

func ParseURI(path string) (bucket string, key string, err error) {

	parts := strings.SplitN(path[5:], `/`, 2) // f.Path[5:] is to trim `s3://` from the path

	if len(parts) < 2 || parts[0] == `` {
		return ``, ``, fmt.Errorf("invalid S3 URI: %s", path)
	}

	return parts[0], parts[1], nil

}

func getRootDir(pattern string) string {

	pattern = filepath.Clean(pattern)

	parts := strings.Split(pattern, string(filepath.Separator))
	index := 0

	for ; index < len(parts); index++ {
		if strings.ContainsAny(parts[index], "*?[") {
			break
		}
	}

	if index == 0 {
		return ""
	}

	return filepath.Join(parts[:index]...)

}

func match(pattern string, objects []types.Object) []types.Object {

	var matches []types.Object

	for _, obj := range objects {
		matched, err := doublestar.Match(pattern, *obj.Key)
		if err != nil {
			continue
		}

		if matched {
			matches = append(matches, obj)
		}
	}

	return matches

}
