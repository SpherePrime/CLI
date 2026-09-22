//go:build !windows

package model

// setConsoleTitle is a platform hook for setting the native console tab title.
// On non-Windows platforms the terminal title is set via ANSI OSC sequences
// emitted by bubbletea, so this is a no-op.
func setConsoleTitle(title string) {}
