// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"github.com/dwertyfa288/CLI/vendordeps/go-logr/logr"

	"github.com/dwertyfa288/CLI/vendordeps/otel/internal/global"
)

// SetLogger configures the logger used internally to opentelemetry.
func SetLogger(logger logr.Logger) {
	global.SetLogger(logger)
}
