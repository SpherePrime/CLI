package config

import (
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

const imageOutputOption = "image_output"

func IsImageOutputModel(model catwalk.Model) bool {
	if flag, ok := model.Options.ProviderOptions[imageOutputOption].(bool); ok {
		return flag
	}
	name := strings.ToLower(model.ID)
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	for _, segment := range strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == '.' || r == ':' }) {
		if segment == "image" || strings.HasPrefix(segment, "imagen") || segment == "dall" {
			return true
		}
	}
	return false
}

type ProviderImageModel struct {
	ProviderID string
	BaseURL    string
	APIKey     string
	Model      string
}

func (c *Config) ProviderImageModel() (ProviderImageModel, bool) {
	if c == nil || c.Providers == nil {
		return ProviderImageModel{}, false
	}
	preferred := ""
	if selected, ok := c.Models[SelectedModelTypeLarge]; ok {
		preferred = selected.Provider
	}
	var found ProviderImageModel
	ok := false
	for providerID, provider := range c.Providers.Seq2() {
		if provider.Disable || strings.TrimSpace(provider.BaseURL) == "" {
			continue
		}
		if provider.Type != "" && provider.Type != catwalk.TypeOpenAI && provider.Type != catwalk.TypeOpenAICompat {
			continue
		}
		for _, model := range provider.Models {
			if !IsImageOutputModel(model) {
				continue
			}
			candidate := ProviderImageModel{ProviderID: providerID, BaseURL: strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/"), APIKey: provider.APIKey, Model: model.ID}
			if providerID == preferred {
				return candidate, true
			}
			if !ok || providerID < found.ProviderID {
				found, ok = candidate, true
			}
			break
		}
	}
	return found, ok
}
