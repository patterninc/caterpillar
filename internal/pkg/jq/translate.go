package jq

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/translate"
	"github.com/itchyny/gojq"
)

type awsTranslateClient struct {
	*translate.Client
}

func new(ctx context.Context) (*awsTranslateClient, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &awsTranslateClient{
		Client: translate.NewFromConfig(awsConfig),
	}, nil

}

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
		panic(err)
	}

	if len(args) < 3 {
		panic("translate requires 3 arguments: text, source language, target language")
	}

	textStr, ok := args[0].(string)
	if !ok {
		panic("invalid text type")
	}

	sourceLang, ok := args[1].(string)
	if !ok {
		panic("invalid source language type")
	}

	targetLang, ok := args[2].(string)
	if !ok {
		panic("invalid target language type")
	}

	translatedText, err := txClient.TranslateText(ctx, textStr, sourceLang, targetLang)
	if err != nil {
		panic(err)
	}

	return translatedText
}

func translateOption() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("translate", 3, 3, translateText),
	}
}
