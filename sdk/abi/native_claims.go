// Package abi contains vendored protocol interfaces.
package abi

import _ "embed"

//go:embed NativeClaims.json
var NativeClaims []byte
