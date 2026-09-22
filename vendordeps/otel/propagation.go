// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"github.com/SpherePrime/CLI/vendordeps/otel/internal/global"
	"github.com/SpherePrime/CLI/vendordeps/otel/propagation"
)

// GetTextMapPropagator returns the global TextMapPropagator. If none has been
// set, a No-Op TextMapPropagator is returned.
func GetTextMapPropagator() propagation.TextMapPropagator {
	return global.TextMapPropagator()
}

// SetTextMapPropagator sets propagator as the global TextMapPropagator.
func SetTextMapPropagator(propagator propagation.TextMapPropagator) {
	global.SetTextMapPropagator(propagator)
}
