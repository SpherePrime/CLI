package desktopengine

import (
	"context"
	"net/http"
	"unicode/utf8"
)

type responseWriter struct {
	ctx    context.Context
	events chan<- Event
	header http.Header
	status int
}

func (w *responseWriter) Header() http.Header { return w.header }

func (w *responseWriter) emit(event Event) bool {
	select {
	case <-w.ctx.Done():
		return false
	case w.events <- event:
		return true
	}
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.emit(Event{Type: "headers", Status: status, Headers: w.header.Clone()})
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	written := 0
	for len(data) > 0 {
		size := min(len(data), maxChunkBytes)
		if size < len(data) {
			for size > 0 && !utf8.RuneStart(data[size]) {
				size--
			}
			if size == 0 {
				size = min(len(data), maxChunkBytes)
			}
		}
		if !w.emit(Event{Type: "data", Data: string(data[:size])}) {
			return written, w.ctx.Err()
		}
		written += size
		data = data[size:]
	}
	return written, nil
}

func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
}
