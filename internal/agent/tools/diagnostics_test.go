package tools

import "testing"

func TestIsVendoredPath(t *testing.T) {
	t.Parallel()
	if !isVendoredPath(`C:\repo\vendordeps\pdfcpu\file.go`) {
		t.Fatal("expected Windows vendordeps path to be detected")
	}
	if !isVendoredPath("/repo/node_modules/pkg/index.js") {
		t.Fatal("expected POSIX node_modules path to be detected")
	}
	if isVendoredPath("/repo/internal/agent/agent.go") {
		t.Fatal("did not expect internal path to be marked vendored")
	}
}
