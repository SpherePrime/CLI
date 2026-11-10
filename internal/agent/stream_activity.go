package agent

import (
	"context"
	"io"
	"net/http"
)

type streamActivityKey struct{}

func withStreamActivity(ctx context.Context, touch func()) context.Context {
	return context.WithValue(ctx, streamActivityKey{}, touch)
}

type streamActivityTransport struct {
	base http.RoundTripper
}

func (t streamActivityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if touch, ok := req.Context().Value(streamActivityKey{}).(func()); ok {
		resp.Body = &streamActivityBody{ReadCloser: resp.Body, touch: touch}
	}
	return resp, nil
}

type streamActivityBody struct {
	io.ReadCloser
	touch func()
}

func (b *streamActivityBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.touch()
	}
	return n, err
}

func withStreamActivityTransport(client *http.Client) *http.Client {
	next := &http.Client{}
	if client != nil {
		copied := *client
		next = &copied
	}
	next.Transport = streamActivityTransport{base: next.Transport}
	return next
}
