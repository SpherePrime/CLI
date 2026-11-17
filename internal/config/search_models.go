package config

import (
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

const webSearchOption = "web_search"

// IsWebSearchModel reports whether the model declares web search support via
// ProviderOptions["web_search"].
func IsWebSearchModel(model catwalk.Model) bool {
	flag, ok := model.Options.ProviderOptions[webSearchOption].(bool)
	return ok && flag
}

type ProviderWebSearchModel struct {
	ProviderID string
	BaseURL    string
	APIKey     string
	Model      string
}

func firstWebSearchModel(models []catwalk.Model, preferredModels ...string) (catwalk.Model, bool) {
	for _, preferred := range preferredModels {
		if preferred == "" {
			continue
		}
		for _, model := range models {
			if model.ID == preferred && IsWebSearchModel(model) {
				return model, true
			}
		}
	}
	for _, model := range models {
		if IsWebSearchModel(model) {
			return model, true
		}
	}
	return catwalk.Model{}, false
}

// ProviderWebSearchModel finds a configured OpenAI/OpenAI-compatible provider
// model with web search support. It prefers the SelectedModelTypeLarge provider,
// then SelectedModelTypeSmall, and otherwise picks the lowest provider ID.
func (c *Config) ProviderWebSearchModel() (ProviderWebSearchModel, bool) {
	if c == nil || c.Providers == nil {
		return ProviderWebSearchModel{}, false
	}

	largeSel := c.Models[SelectedModelTypeLarge]
	smallSel := c.Models[SelectedModelTypeSmall]

	var (
		foundLarge, foundSmall, foundLowest ProviderWebSearchModel
		okLarge, okSmall, okLowest          bool
	)

	for providerID, provider := range c.Providers.Seq2() {
		if provider.Disable || strings.TrimSpace(provider.BaseURL) == "" {
			continue
		}
		if provider.Type != "" && provider.Type != catwalk.TypeOpenAI && provider.Type != catwalk.TypeOpenAICompat {
			continue
		}

		var preferredModels []string
		if providerID == largeSel.Provider {
			preferredModels = append(preferredModels, largeSel.Model)
		}
		if providerID == smallSel.Provider {
			preferredModels = append(preferredModels, smallSel.Model)
		}

		model, ok := firstWebSearchModel(provider.Models, preferredModels...)
		if !ok {
			continue
		}

		candidate := ProviderWebSearchModel{
			ProviderID: providerID,
			BaseURL:    strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/"),
			APIKey:     provider.APIKey,
			Model:      model.ID,
		}

		if largeSel.Provider != "" && providerID == largeSel.Provider {
			foundLarge, okLarge = candidate, true
		}
		if smallSel.Provider != "" && providerID == smallSel.Provider {
			foundSmall, okSmall = candidate, true
		}
		if !okLowest || providerID < foundLowest.ProviderID {
			foundLowest, okLowest = candidate, true
		}
	}

	if okLarge {
		return foundLarge, true
	}
	if okSmall {
		return foundSmall, true
	}
	return foundLowest, okLowest
}
