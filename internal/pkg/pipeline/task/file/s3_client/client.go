package s3client

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bmatcuk/doublestar"
)

type client struct {
	*s3.Client
	region string
}

var c client

func New(ctx context.Context, region string) (*client, error) {

	if c.Client != nil && c.region == region {
		return &c, nil
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	c.Client = s3.NewFromConfig(awsConfig)
	c.region = region

	return &c, nil

}

func (c *client) GetObjectsWithGlob(ctx context.Context, bucketName, pattern string) ([]types.Object, error) {

	var matchingObjects []types.Object
	
	
	// List all objects in the bucket
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}
	
	prefix := getRootDirFromGlob(pattern)
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	paginator := s3.NewListObjectsV2Paginator(c.Client, input)

	for {

		if !paginator.HasMorePages() {
			break
		}

		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		objects := match(pattern, output.Contents)
		matchingObjects = append(matchingObjects, objects...)

	}

	return matchingObjects, nil

}

func getRootDirFromGlob(pattern string) string {

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
