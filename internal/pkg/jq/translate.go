package jq

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/translate"
	"github.com/itchyny/gojq"
)

type awsTranslateClient struct {
	*translate.Client
}

var (
	client *awsTranslateClient
	once   sync.Once
)

func new(ctx context.Context) (*awsTranslateClient, error) {

	var err error
	once.Do(func() {
		awsConfig, loadErr := config.LoadDefaultConfig(ctx)
		if loadErr != nil {
			err = loadErr
			return
		}

		client = &awsTranslateClient{
			Client: translate.NewFromConfig(awsConfig),
		}
	})

	return client, err
}

// TranslateText translates the given text from sourceLang to targetLang.
func (c *awsTranslateClient) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {

	output, err := c.Client.TranslateText(ctx, &translate.TranslateTextInput{
		Text:               &text,
		SourceLanguageCode: &sourceLang,
		TargetLanguageCode: &targetLang,
	})

	if err != nil {
		return "", err
	}
	return *output.TranslatedText, nil
}

func translateText(_ any, args []any) any {
	ctx := context.Background()
	txClient, err := new(ctx)
	if err != nil {
		return err
	}

	if len(args) < 3 {
		return fmt.Errorf("translate requires 3 arguments: text, source language, target language")
	}

	textStr, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("expected string for text for translate, got %T", args[0])
	}

	sourceLang, ok := args[1].(string)
	if !ok {
		return fmt.Errorf("expected string for source language for translate, got %T", args[1])
	}

	targetLang, ok := args[2].(string)
	if !ok {
		return fmt.Errorf("expected string for target language for translate, got %T", args[2])
	}

	translatedText, err := txClient.TranslateText(ctx, textStr, sourceLang, targetLang)
	if err != nil {
		return err
	}

	return translatedText
}

func translateOption() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("translate", 3, 3, translateText),
	}
}
