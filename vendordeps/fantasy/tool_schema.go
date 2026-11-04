package fantasy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/SpherePrime/CLI/vendordeps/google/jsonschema-go/jsonschema"
)

var toolSchemas = struct {
	sync.Mutex
	entries map[[32]byte]*jsonschema.Resolved
}{entries: make(map[[32]byte]*jsonschema.Resolved)}

func CanonicalToolSchema(info ToolInfo) (map[string]any, error) {
	source := info.InputSchema
	if source == nil {
		required := info.Required
		if required == nil {
			required = []string{}
		}
		properties := info.Parameters
		if properties == nil {
			properties = map[string]any{}
		}
		source = map[string]any{"type": "object", "properties": properties, "required": required}
	}
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("tool %s schema: %w", info.Name, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ValidateToolInput(info ToolInfo, input string) error {
	canonical, err := CanonicalToolSchema(info)
	if err != nil {
		return err
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return err
	}
	key := sha256.Sum256(data)
	toolSchemas.Lock()
	resolved := toolSchemas.entries[key]
	toolSchemas.Unlock()
	if resolved == nil {
		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return fmt.Errorf("tool %s schema: %w", info.Name, err)
		}
		resolved, err = schema.Resolve(nil)
		if err != nil {
			return fmt.Errorf("tool %s schema: %w", info.Name, err)
		}
		toolSchemas.Lock()
		if len(toolSchemas.entries) >= 512 {
			clear(toolSchemas.entries)
		}
		toolSchemas.entries[key] = resolved
		toolSchemas.Unlock()
	}
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("tool input must be an object")
	}
	if err := resolved.Validate(value); err != nil {
		return fmt.Errorf("tool %s input: %w", info.Name, err)
	}
	return nil
}
