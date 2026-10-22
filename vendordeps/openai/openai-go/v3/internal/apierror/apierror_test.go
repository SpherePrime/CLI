package apierror

import (
	"net/http"
	"testing"
)

func TestUnmarshalJSONStringError(t *testing.T) {
	e := &Error{}
	if err := e.UnmarshalJSON([]byte(`"rate limited"`)); err != nil {
		t.Fatalf("UnmarshalJSON string: %v", err)
	}
	if e.Message != "rate limited" {
		t.Fatalf("Message = %q, want %q", e.Message, "rate limited")
	}
	if e.JSON.raw != `"rate limited"` {
		t.Fatalf("raw = %q", e.JSON.raw)
	}
}

func TestUnmarshalJSONObjectError(t *testing.T) {
	e := &Error{}
	body := `{"code":"404","message":"not found","param":"","type":"invalid_request_error"}`
	if err := e.UnmarshalJSON([]byte(body)); err != nil {
		t.Fatalf("UnmarshalJSON object: %v", err)
	}
	if e.Message != "not found" || e.Code != "404" {
		t.Fatalf("Message = %q, Code = %q", e.Message, e.Code)
	}
}

func TestNewRaw(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/chat", nil)
	e := NewRaw(502, req, nil, `<html>bad gateway</html>`)
	if e.StatusCode != 502 || e.Message != `<html>bad gateway</html>` {
		t.Fatalf("unexpected error: %+v", e)
	}
	if e.JSON.raw != `<html>bad gateway</html>` {
		t.Fatalf("raw = %q", e.JSON.raw)
	}
}
