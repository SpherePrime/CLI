package proto

import "github.com/SpherePrime/CLI/internal/config"

type ProviderSaveRequest struct {
	Scope      config.Scope        `json:"scope"`
	OriginalID string              `json:"original_id,omitempty"`
	Provider   config.ProviderEdit `json:"provider"`
	APIKey     *string             `json:"api_key,omitempty"`
}
