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
	var stores []*config.ConfigStore
	if request.Scope == config.ScopeGlobal {
		for _, opened := range b.workspaces.Seq2() {
			stores = append(stores, opened.Cfg)
		}
	}
	if err := ws.Cfg.SaveProviderWithWorkspaces(request.Scope, request.OriginalID, request.Provider, request.APIKey, stores); err != nil {
		return err
	}
	if request.Scope == config.ScopeGlobal {
		for _, opened := range b.workspaces.Seq2() {
			publishConfigChanged(opened)
		}
	} else {
		publishConfigChanged(ws)
	}
	return nil
}
