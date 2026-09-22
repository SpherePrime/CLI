// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"github.com/SpherePrime/CLI/vendordeps/go-logr/logr"

	"github.com/SpherePrime/CLI/vendordeps/otel/internal/global"
)

// SetLogger configures the logger used internally to opentelemetry.
func SetLogger(logger logr.Logger) {
	global.SetLogger(logger)
}
