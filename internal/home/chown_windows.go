//go:build windows

package home

// chown is a no-op on Windows: file ownership is not something Prime
// hands back there, and the sudo resolution never activates on this
// platform anyway.
func chown(string, int, int) error { return nil }
