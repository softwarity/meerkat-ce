package discovery

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// k8sCertPool trusts the cluster's own authority and nothing else.
//
// The API server presents a certificate signed by the cluster CA, which is
// mounted beside the token. Skipping verification would have been one line and
// it is the line that turns a gateway's own credentials into something a
// man-in-the-middle can collect.
func k8sCertPool() (*tls.Config, error) {
	pem, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("no cluster CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("the cluster CA at %s/ca.crt is not readable as PEM", saDir)
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}
