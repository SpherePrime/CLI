package backend

import (
	"fmt"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/voice"
)

func (b *Backend) SttCatalog(workspaceID string) (proto.SttCatalog, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.SttCatalog{}, err
	}
	cfg := ws.Cfg.Config()
	language := ""
	dataDir := ""
	if cfg != nil && cfg.Options != nil {
		dataDir = cfg.Options.DataDirectory
		if cfg.Options.Voice != nil {
			language = cfg.Options.Voice.Language
		}
	}
	status := voice.Status(dataDir, language)
	return proto.SttCatalog{
		Engine:    status.Engine,
		Model:     status.Model,
		Language:  status.Language,
		Languages: status.Languages,
		Installed: status.Installed,
	}, nil
}

func (b *Backend) SelectSttModel(workspaceID string, request proto.SttModelRequest) (proto.SttSelection, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.SttSelection{}, err
	}
	language := voice.DisplayLanguage(request.Language)
	if err := ws.Cfg.SetConfigField(config.ScopeGlobal, "options.voice.language", language); err != nil {
		return proto.SttSelection{}, fmt.Errorf("persist options.voice.language: %w", err)
	}
	return proto.SttSelection{
		Engine:   voice.EngineTinyASR,
		Model:    "en-ru-uk",
		Language: language,
	}, nil
}
