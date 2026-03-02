package grpc

import (
	"crypto/x509"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// StaticKeyFunc returns a jwt.Keyfunc that always uses the provided PEM-encoded
// public key for token verification.
func StaticKeyFunc(publicKeyPEM []byte) (jwt.Keyfunc, error) {
	pub, err := x509.ParsePKIXPublicKey(publicKeyPEM)
	if err != nil {
		// Try parsing as PEM block first.
		key, parseErr := jwt.ParseRSAPublicKeyFromPEM(publicKeyPEM)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}
		pub = key
	}

	return func(_ *jwt.Token) (any, error) {
		return pub, nil
	}, nil
}
