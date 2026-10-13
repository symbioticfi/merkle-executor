// Package curator exposes the shared permission artifact source to the daemon.
package curator

import (
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
)

type Source = permissions.Source

var New = permissions.New
var ValidateConfig = permissions.ValidateConfig
