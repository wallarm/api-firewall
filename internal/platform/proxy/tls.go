package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// BuildTLSConfig creates a TLS configuration with optional custom root CA.
func BuildTLSConfig(insecure bool, rootCA string) (*tls.Config, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		// On some systems (e.g. scratch containers), system pool is unavailable
		rootCAs = x509.NewCertPool()
	}

	if rootCA != "" {
		certs, err := os.ReadFile(rootCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read root CA %q: %w", rootCA, err)
		}

		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, errors.New("failed to append root CA certificates")
		}
	}

	return &tls.Config{
		InsecureSkipVerify: insecure,
		RootCAs:            rootCAs,
	}, nil
}
