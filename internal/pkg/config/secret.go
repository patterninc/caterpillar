package config

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	awsTrue = aws.Bool(true)
	ctx     = context.Background()
)

func getSecret(path string) (string, error) {

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ``, err
	}

	svc := ssm.NewFromConfig(cfg)

	value, err := svc.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(path),
		WithDecryption: awsTrue,
	})

	if err != nil {
		return ``, err
	}

	if value == nil || value.Parameter == nil {
		return ``, fmt.Errorf("can't get %s parameter value", path)
	}

	return *value.Parameter.Value, nil

}
