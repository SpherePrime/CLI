package primeai

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
		ContextWindow:          200000,
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

func MapPublicModels(models []PublicModel) []catwalk.Model {
	mapped := make([]catwalk.Model, 0, len(models)+1)
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
		mapped = append(mapped, catwalk.Model{
			ID:                     pm.ID,
			Name:                   cmp.Or(pm.Name, pm.ID),
			CostPer1MIn:            priceIn,
			CostPer1MOut:           priceOut,
			ContextWindow:          200000,
			DefaultMaxTokens:       16384,
			CanReason:              true,
			ReasoningLevels:        []string{"low", "medium", "high"},
			DefaultReasoningEffort: "medium",
			SupportsImages:         pm.SupportsVision,
		})
	}
	return WithAutomaticModel(mapped)
}

func (c *Client) FetchModels(ctx context.Context) ([]catwalk.Model, error) {
	resp, err := c.doWithSession(ctx, func(reqCtx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, c.base()+"/api/plugin/models", nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Cache-Control", "no-store")
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read PrimeAI models response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PrimeAI models request failed (%d): %s", resp.StatusCode, extractErrorMessage(body, http.StatusText(resp.StatusCode)))
	}

	var payload struct {
		Models []PublicModel `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode PrimeAI models response: %w", err)
	}

	return MapPublicModels(payload.Models), nil
}

func FetchModels(ctx context.Context) ([]catwalk.Model, error) {
	return DefaultClient().FetchModels(ctx)
}
