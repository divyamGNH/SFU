package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc/credentials"
)

func LoadClientTLSCredentials() (credentials.TransportCredentials, error) {
	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		certDir = "../../certs"
	}

	caCert, err := os.ReadFile(filepath.Join(certDir, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("Failed to read CA certificate : %v", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	clientCertFile, err := tls.LoadX509KeyPair(
		filepath.Join(certDir, "sfu-client.crt"),
		filepath.Join(certDir, "sfu-client.key"),
	)

	if err != nil {
		return nil, fmt.Errorf("Failed to read client certificate keys : %v", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{clientCertFile},
		RootCAs:      certPool,
	}

	return credentials.NewTLS(config), nil
}
