//go:build windows

package voice

import (
	"fmt"

	win "github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

// loadDynamicLibrary opens a DLL with LoadLibrary, which is the Windows
// counterpart of dlopen that purego leaves to the caller.
func loadDynamicLibrary(path string) (uintptr, error) {
	handle, err := win.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

// findSymbol resolves a symbol in a loaded DLL without panicking.
func findSymbol(handle uintptr, name string) (uintptr, error) {
	proc, err := win.GetProcAddress(win.Handle(handle), name)
	if err != nil {
		return 0, fmt.Errorf("GetProcAddress %s: %w", name, err)
	}
	return uintptr(proc), nil
}
