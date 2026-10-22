package desktopengine

import "net/http"

const ABIVersion = 1
const MaxRequestBytes = 32 << 20
const maxChunkBytes = 32 << 10
const maxPendingRequests = 256
const eventQueueSize = 64

type Options struct {
	WorkingDir string `json:"working_dir"`
	DataDir    string `json:"data_dir"`
	Debug      bool   `json:"debug"`
}

type Request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type Event struct {
	Type    string      `json:"type"`
	Status  int         `json:"status,omitempty"`
	Headers http.Header `json:"headers,omitempty"`
	Data    string      `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
