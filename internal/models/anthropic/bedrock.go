package anthropic

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/gregriff/ducky/internal/models"
)

// buildBedrockConfig creates a bedrock config and appends it to opts.
func buildBedrockConfig(
	ctx context.Context,
	bedrockConfig *models.BedrockConfig,
	opts []option.RequestOption,
) ([]option.RequestOption, error) {
	var certsPEM *os.File
	var err error
	if bedrockConfig.ExtraCerts && bedrockConfig.CertsPath != "" {
		certsPEM, err = os.Open(bedrockConfig.CertsPath)
		if err != nil {
			return nil, fmt.Errorf("error opening extra ca certs file: %w", err)
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithCustomCABundle(certsPEM))
	if err != nil {
		return nil, fmt.Errorf("error loading default aws config with extra certs: %w", err)
	}

	// ensure creds are present.
	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving AWS credentials: %w", err)
	}

	// this needs to be unset so that AWS SigV4 signing is used for the chat session.
	cfg.BearerAuthTokenProvider = nil

	return append(opts, bedrock.WithConfig(cfg)), nil
}
