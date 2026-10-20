package backend

import (
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
)

func (b *Backend) SaveProvider(workspaceID string, request proto.ProviderSaveRequest) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if request.Scope == config.ScopeGlobal && request.OriginalID != "" && request.OriginalID != request.Provider.ID {
		for _, opened := range b.workspaces.Seq2() {
			if err := opened.Cfg.ValidateGlobalProviderRename(request.OriginalID); err != nil {
				return err
			}
		}
	}
	if err := ws.Cfg.SaveProvider(request.Scope, request.OriginalID, request.Provider, request.APIKey); err != nil {
		return err
	}
	if request.Scope == config.ScopeGlobal {
		for _, opened := range b.workspaces.Seq2() {
			if opened.ID != ws.ID {
				if err := opened.Cfg.ReloadProviderConfiguration(request.OriginalID, request.Provider.ID); err != nil {
					return err
				}
			}
			publishConfigChanged(opened)
		}
	} else {
		publishConfigChanged(ws)
	}
	return nil
}
