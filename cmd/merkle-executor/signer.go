package main

import (
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

// Credentials enter at startup; SDK components receive only the signing interface.
func signerFromEnv(name string) (signer.Signer, error) {
	if name == "" {
		return nil, errors.New("signer env variable name required")
	}
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil, errors.New("signer env variable is empty")
	}
	return signer.NewFromHexKey(value)
}
