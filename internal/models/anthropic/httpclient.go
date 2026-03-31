package anthropic

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

func newHTTPClientWithExtraCACert(certPath string) (*http.Client, error) {
	if certPath == "" {
		return http.DefaultClient, nil
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

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: rootCAs}

	return &http.Client{Transport: transport}, nil
}
