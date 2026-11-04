package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

type gatewayTransport struct {
	base       http.RoundTripper
	parameters map[string][]string
}

func withGatewayCapabilities(client *http.Client, models []catwalk.Model) *http.Client {
	parameters := make(map[string][]string)
	allowed := map[string]bool{
		"max_tokens": true, "max_completion_tokens": true, "max_output_tokens": true,
		"temperature": true, "top_p": true, "frequency_penalty": true, "presence_penalty": true,
	}
	for _, model := range models {
		gateway, ok := model.Options.ProviderOptions["gateway"].(map[string]any)
		if !ok || gateway["type"] != "codex" {
			continue
		}
		data, err := json.Marshal(gateway["unsupported_generation_parameters"])
		if err != nil {
			continue
		}
		var names []string
		if json.Unmarshal(data, &names) != nil {
			continue
		}
		for _, name := range names {
			if allowed[name] {
				parameters[model.ID] = append(parameters[model.ID], name)
			}
		}
	}
	if len(parameters) == 0 {
		return client
	}
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = gatewayTransport{base: base, parameters: parameters}
	return &copy
}

func (transport gatewayTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	path := strings.TrimRight(request.URL.Path, "/")
	if request.Method != http.MethodPost || request.Body == nil || (!strings.HasSuffix(path, "/chat/completions") && !strings.HasSuffix(path, "/responses")) {
		return transport.base.RoundTrip(request)
	}
	data, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil {
		return nil, err
	}
	var body map[string]json.RawMessage
	var model string
	if json.Unmarshal(data, &body) == nil && json.Unmarshal(body["model"], &model) == nil {
		changed := false
		for _, name := range transport.parameters[model] {
			if _, exists := body[name]; exists {
				delete(body, name)
				changed = true
			}
		}
		if changed {
			data, err = json.Marshal(body)
			if err != nil {
				return nil, err
			}
		}
	}
	clone := request.Clone(request.Context())
	clone.Body = io.NopCloser(bytes.NewReader(data))
	clone.ContentLength = int64(len(data))
	clone.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	return transport.base.RoundTrip(clone)
}
