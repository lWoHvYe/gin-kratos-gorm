//go:build tools

// Package tools pins build-time generators so `go mod tidy` keeps their
// dependencies in go.mod/go.sum and `make wire` works from a clean checkout.
package tools

import _ "github.com/google/wire/cmd/wire"
