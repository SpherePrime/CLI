//go:build windows

package winio

//go:generate go run github.com/dwertyfa288/CLI/vendordeps/Microsoft/go-winio/tools/mkwinsyscall -output zsyscall_windows.go ./*.go
