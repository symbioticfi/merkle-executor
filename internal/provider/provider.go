// Package provider holds daemon-owned runtime provider dependencies.
package provider

import (
	"net/http"

	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

type Deps struct {
	Chain chainio.Reader
	HTTP  *http.Client
}
