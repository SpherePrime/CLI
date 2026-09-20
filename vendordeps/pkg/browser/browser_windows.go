package browser

import "github.com/dwertyfa288/CLI/vendordeps/x/sys/windows"

func openBrowser(url string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(url), nil, nil, windows.SW_SHOWNORMAL)
}
