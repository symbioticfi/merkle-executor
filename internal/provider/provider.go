// Package provider holds daemon-owned runtime provider dependencies.
package provider

import (
	"net/http"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Deps struct {
	Chain provider.Reader
	HTTP  *http.Client
}
