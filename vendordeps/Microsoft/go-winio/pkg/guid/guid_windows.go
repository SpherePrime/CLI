//go:build windows
// +build windows

package guid

import "github.com/dwertyfa288/CLI/vendordeps/x/sys/windows"

// GUID represents a GUID/UUID. It has the same structure as
// github.com/dwertyfa288/CLI/vendordeps/x/sys/windows.GUID so that it can be used with functions expecting
// that type. It is defined as its own type so that stringification and
// marshaling can be supported. The representation matches that used by native
// Windows code.
type GUID windows.GUID
