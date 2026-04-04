package anthropic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/gregriff/ducky/internal/models"
)

// buildBedrockConfig creates a bedrock config and appends it to opts.
func buildBedrockConfig(
	ctx context.Context,
	bedrockConfig *models.BedrockConfig,
	opts []option.RequestOption,
) ([]option.RequestOption, error) {
	if !bedrockConfig.ExtraCerts {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("error creating aws config: %w", err)
		}
		return append(opts, bedrock.WithConfig(cfg)), nil
	}

	httpClient, err := newCustomCertClient(bedrockConfig.CertsPath)
	if err != nil {
		return nil, fmt.Errorf("error creating httpClient with extra certs: %w", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithHTTPClient(httpClient))
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

func newCustomCertClient(certPath string) (*awshttp.BuildableClient, error) {
	if certPath == "" {
		return nil, nil
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert: %w", err)
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool() // fallback if system pool unavailable
	}

	if ok := rootCAs.AppendCertsFromPEM(certPEM); !ok {
		return nil, fmt.Errorf("error appending CA cert")
	}

	httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		tr.TLSClientConfig = &tls.Config{RootCAs: rootCAs}
	})

	return httpClient, nil
}
