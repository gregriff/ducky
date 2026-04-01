package anthropic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/gregriff/ducky/internal/models"
)

// buildBedrockConfig creates a bedrock config and appends it to opts.
func buildBedrockConfig(bedrockConfig *models.BedrockConfig, opts []option.RequestOption) []option.RequestOption {
	if !bedrockConfig.ExtraCerts {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Fatalf("error creating aws config: %v", err)
		}
		return append(opts, bedrock.WithConfig(cfg))
	}

	httpClient, err := newCustomCertClient(bedrockConfig.CertsPath)
	if err != nil {
		log.Fatalf("error creating httpClient with extra certs: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		log.Fatalf("error loading default aws config with extra certs: %v", err)
	}

	return append(opts, bedrock.WithConfig(cfg))
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
