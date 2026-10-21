//go:build !windows

package voice

import (
	purego "github.com/SpherePrime/CLI/vendordeps/ebitengine/purego"
)

// loadDynamicLibrary opens a shared library on the dlopen platforms.
func loadDynamicLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW)
}

// findSymbol resolves a symbol in a loaded library without panicking.
func findSymbol(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}
