// Package embedded provides access to all providers in a embedded manner.
// This basically means offline access to the providers.
package embedded

import (
	"github.com/dwertyfa288/CLI/vendordeps/catwalk/internal/providers"
	"github.com/dwertyfa288/CLI/vendordeps/catwalk/pkg/catwalk"
)

// GetAll returns all embedded providers.
func GetAll() []catwalk.Provider {
	return providers.GetAll()
}
