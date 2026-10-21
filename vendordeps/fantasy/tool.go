package fantasy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/SpherePrime/CLI/vendordeps/fantasy/schema"
)

// Schema represents a JSON schema for tool input validation.
type Schema = schema.Schema

// ToolInfo represents tool metadata, matching the existing pattern.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    []string       `json:"required"`
	Parallel    bool           `json:"parallel"` // Whether this tool can run in parallel with other tools
}

// ToolCall represents a tool invocation, matching the existing pattern.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

// ToolResponse represents the response from a tool execution, matching the existing pattern.
type ToolResponse struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	// Data contains binary data for image/media responses (e.g., image bytes, audio data).
	Data []byte `json:"data,omitempty"`
	// MediaType specifies the MIME type of the media (e.g., "image/png", "audio/wav").
	MediaType string `json:"media_type,omitempty"`
	// Images carries additional media items for UI display. The first item is
	// mirrored into Data/MediaType so providers keep seeing one media result;
	// the rest only travel to clients, not back to the model.
	Images []ImagePayload `json:"images,omitempty"`
	Metadata string `json:"metadata,omitempty"`
	IsError  bool   `json:"is_error"`
	StopTurn bool   `json:"stop_turn,omitempty"`
}

// ImagePayload is one media item attached to a tool response.
type ImagePayload struct {
	// Data holds the raw bytes; they are base64-encoded on the way to clients.
	Data []byte `json:"data"`
	// MediaType is the MIME type such as "image/png".
	MediaType string `json:"media_type"`
	// Path is the file on disk this item was saved to, when known.
	Path string `json:"path,omitempty"`
}

// NewTextResponse creates a text response.
func NewTextResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    "text",
		Content: content,
	}
}

// NewTextErrorResponse creates an error response.
func NewTextErrorResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    "text",
		Content: content,
		IsError: true,
	}
}

// NewImageResponse creates an image response with binary data.
func NewImageResponse(data []byte, mediaType string) ToolResponse {
	return ToolResponse{
		Type:      "image",
		Data:      data,
		MediaType: mediaType,
	}
}

// NewImagesResponse creates an image response carrying several images. The
// first image also populates Data/MediaType, because providers accept a single
// media item per tool result.
func NewImagesResponse(images []ImagePayload, content string) ToolResponse {
	response := ToolResponse{Type: "image", Content: content, Images: images}
	if len(images) > 0 {
		response.Data = images[0].Data
		response.MediaType = images[0].MediaType
	}
	return response
}

// NewMediaResponse creates a media response with binary data (e.g., audio, video).
func NewMediaResponse(data []byte, mediaType string) ToolResponse {
	return ToolResponse{
		Type:      "media",
		Data:      data,
		MediaType: mediaType,
	}
}

// WithResponseMetadata adds metadata to a response.
func WithResponseMetadata(response ToolResponse, metadata any) ToolResponse {
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return response
		}
		response.Metadata = string(metadataBytes)
	}
	return response
}

// AgentTool represents a tool that can be called by a language model.
// This matches the existing BaseTool interface pattern.
type AgentTool interface {
	Info() ToolInfo
	Run(ctx context.Context, params ToolCall) (ToolResponse, error)
	ProviderOptions() ProviderOptions
	SetProviderOptions(opts ProviderOptions)
}

// NewAgentTool creates a typed tool from a function with automatic schema generation.
// This is the recommended way to create tools.
func NewAgentTool[TInput any](
	name string,
	description string,
	fn func(ctx context.Context, input TInput, call ToolCall) (ToolResponse, error),
) AgentTool {
	var input TInput
	schema := schema.Generate(reflect.TypeOf(input))

	return &funcToolWrapper[TInput]{
		name:        name,
		description: description,
		fn:          fn,
		schema:      schema,
		parallel:    false, // Default to sequential execution
	}
}

// NewParallelAgentTool creates a typed tool from a function with automatic schema generation.
// This also marks a tool as safe to run in parallel with other tools.
func NewParallelAgentTool[TInput any](
	name string,
	description string,
	fn func(ctx context.Context, input TInput, call ToolCall) (ToolResponse, error),
) AgentTool {
	tool := NewAgentTool(name, description, fn)
	// Try to use the SetParallel method if available
	if setter, ok := tool.(interface{ SetParallel(bool) }); ok {
		setter.SetParallel(true)
	}
	return tool
}

// funcToolWrapper wraps a function to implement the AgentTool interface.
type funcToolWrapper[TInput any] struct {
	name            string
	description     string
	fn              func(ctx context.Context, input TInput, call ToolCall) (ToolResponse, error)
	schema          Schema
	providerOptions ProviderOptions
	parallel        bool
}

func (w *funcToolWrapper[TInput]) SetProviderOptions(opts ProviderOptions) {
	w.providerOptions = opts
}

func (w *funcToolWrapper[TInput]) ProviderOptions() ProviderOptions {
	return w.providerOptions
}

func (w *funcToolWrapper[TInput]) SetParallel(parallel bool) {
	w.parallel = parallel
}

func (w *funcToolWrapper[TInput]) Info() ToolInfo {
	if w.schema.Required == nil {
		w.schema.Required = []string{}
	}
	return ToolInfo{
		Name:        w.name,
		Description: w.description,
		Parameters:  schema.ToParameters(w.schema),
		Required:    w.schema.Required,
		Parallel:    w.parallel,
	}
}

func (w *funcToolWrapper[TInput]) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	var input TInput
	if err := json.Unmarshal([]byte(params.Input), &input); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("invalid parameters: %s", err)), nil
	}

	return w.fn(ctx, input, params)
}
