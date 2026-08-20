package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const dataPlatformCaterpillarSSMRole = `arn:aws:iam::842676014003:role/heimdall-caterpillar-ssm-read`

var (
	awsTrue = aws.Bool(true)
	ctx     = context.Background()
)

func assumeRoleARN(path string) string {
	if strings.HasPrefix(path, `/caterpillar/`) {
		return dataPlatformCaterpillarSSMRole
	}
	return ``
}

func getSecret(path string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ``, err
	}

	if roleARN := assumeRoleARN(path); roleARN != `` {
		cfg.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleARN),
		)
	}

	svc := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.Retryer = retry.NewAdaptiveMode(func(amo *retry.AdaptiveModeOptions) {
			amo.StandardOptions = []func(*retry.StandardOptions){
				func(so *retry.StandardOptions) {
					so.MaxAttempts = 15
				},
			}
		})
	})

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
