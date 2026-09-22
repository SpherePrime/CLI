//go:build windows

package model

import (
	"sync"
	"unsafe"

	"github.com/SpherePrime/CLI/vendordeps/x/sys/windows"
)

var procSetConsoleTitleW = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleTitleW")

var (
	consoleTitleMu    sync.Mutex
	lastConsoleTitle  string
)

func setConsoleTitle(title string) {
	if title == "" {
		return
	}
	consoleTitleMu.Lock()
	if title == lastConsoleTitle {
		consoleTitleMu.Unlock()
		return
	}
	lastConsoleTitle = title
	consoleTitleMu.Unlock()

	utf16Title, err := windows.UTF16FromString(title)
	if err != nil {
		return
	}
	_, _, _ = procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(&utf16Title[0])))
}
