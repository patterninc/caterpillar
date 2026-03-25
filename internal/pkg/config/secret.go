package config

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	awsTrue = aws.Bool(true)
	ctx     = context.Background()
)

func getSecret(path string) (string, error) {
	var value *ssm.GetParameterOutput
	var err error

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ``, err
	}

	svc := ssm.NewFromConfig(cfg)

	for attempt := range 3 {
		value, err = svc.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(path),
			WithDecryption: awsTrue,
		})
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(100+rand.IntN(400)) * time.Millisecond)
		}
	}

	if err != nil {
		return ``, err
	}

	if value == nil || value.Parameter == nil {
		return ``, fmt.Errorf("can't get %s parameter value", path)
	}

	return *value.Parameter.Value, nil

}
