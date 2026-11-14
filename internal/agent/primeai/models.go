package primeai

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

func BuildAutomaticModel(models []catwalk.Model) catwalk.Model {
	supportsImages := false
	for _, m := range models {
		if m.ID == AutomaticModelID {
			continue
		}
		if m.SupportsImages {
			supportsImages = true
			break
		}
	}
	return catwalk.Model{
		ID:                     AutomaticModelID,
		Name:                   AutomaticModelName,
		ContextWindow:          270000,
		DefaultMaxTokens:       16384,
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "medium",
		SupportsImages:         supportsImages,
	}
}

func WithAutomaticModel(models []catwalk.Model) []catwalk.Model {
	filtered := make([]catwalk.Model, 0, len(models)+1)
	for _, m := range models {
		if m.ID == "" || m.ID == AutomaticModelID {
			continue
		}
		filtered = append(filtered, m)
	}
	auto := BuildAutomaticModel(filtered)
	return append([]catwalk.Model{auto}, filtered...)
}

func cleanStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func MapPublicModelsWithAccess(models []PublicModel) ([]catwalk.Model, map[string]ModelAccess) {
	mapped := make([]catwalk.Model, 0, len(models)+1)
	access := make(map[string]ModelAccess, len(models))
	for _, pm := range models {
		if pm.ID == "" || pm.ID == AutomaticModelID {
			continue
		}
		if pm.Kind != "" && pm.Kind != "text" {
			continue
		}
		var priceIn, priceOut float64
		if pm.PriceIn != nil {
			priceIn = *pm.PriceIn
		}
		if pm.PriceOut != nil {
			priceOut = *pm.PriceOut
		}
		reasoningLevels := make([]string, 0, len(pm.ReasoningEfforts))
		for _, effort := range pm.ReasoningEfforts {
			if trimmed := strings.TrimSpace(effort); trimmed != "" {
				reasoningLevels = append(reasoningLevels, trimmed)
			}
		}
		if len(reasoningLevels) == 0 {
			reasoningLevels = []string{"low", "medium", "high"}
		}
		mapped = append(mapped, catwalk.Model{
			ID:                     pm.ID,
			Name:                   cmp.Or(pm.Name, pm.ID),
			CostPer1MIn:            priceIn,
			CostPer1MOut:           priceOut,
			ContextWindow:          cmp.Or(pm.ContextWindow, pm.MaxContextLength, 270000),
			DefaultMaxTokens:       cmp.Or(pm.DefaultMaxTokens, pm.MaxOutputTokens, 16384),
			CanReason:              true,
			ReasoningLevels:        reasoningLevels,
			DefaultReasoningEffort: cmp.Or(strings.TrimSpace(pm.DefaultReasoningEffort), "medium"),
			SupportsImages:         pm.SupportsVision || pm.SupportsAttachments,
		})
		access[pm.ID] = ModelAccess{
			AllowedTariffIDs:   cleanStringSlice(pm.AllowedTariffIDs),
			AllowedTariffNames: cleanStringSlice(pm.AllowedTariffNames),
			PlanLabel:          strings.TrimSpace(pm.PlanLabel),
			Locked:             pm.Locked,
		}
	}
	return WithAutomaticModel(mapped), access
}

func MapPublicModels(models []PublicModel) []catwalk.Model {
	mapped, _ := MapPublicModelsWithAccess(models)
	return mapped
}

func (c *Client) FetchModelsWithAccess(ctx context.Context) ([]catwalk.Model, map[string]ModelAccess, error) {
	resp, err := c.doWithSession(ctx, func(reqCtx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, c.base()+"/api/prime/models", nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Cache-Control", "no-store")
		return req, nil
	})
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, nil, fmt.Errorf("read PrimeAI models response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("PrimeAI models request failed (%d): %s", resp.StatusCode, extractErrorMessage(body, http.StatusText(resp.StatusCode)))
	}

	var payload struct {
		Models []PublicModel `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode PrimeAI models response: %w", err)
	}

	mapped, access := MapPublicModelsWithAccess(payload.Models)
	c.setModelAccess(access)
	return mapped, access, nil
}

func (c *Client) FetchModels(ctx context.Context) ([]catwalk.Model, error) {
	models, _, err := c.FetchModelsWithAccess(ctx)
	return models, err
}

func FetchModels(ctx context.Context) ([]catwalk.Model, error) {
	return DefaultClient().FetchModels(ctx)
}
