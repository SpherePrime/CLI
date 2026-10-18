package lsp

import (
	powernapconfig "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/powernap/pkg/config"
)

func (s *Manager) configuredServers() map[string]*powernapconfig.ServerConfig {
	manager := powernapconfig.NewManager()
	manager.LoadDefaults()
	for name, clientConfig := range s.cfg.Config().LSP {
		actualName := resolveServerName(manager, name)
		if clientConfig.Disabled {
			manager.RemoveServer(actualName)
			continue
		}
		manager.AddServer(actualName, &powernapconfig.ServerConfig{
			Command:     clientConfig.Command,
			Args:        clientConfig.Args,
			Environment: clientConfig.Env,
			FileTypes:   clientConfig.FileTypes,
			RootMarkers: clientConfig.RootMarkers,
			InitOptions: clientConfig.InitOptions,
			Settings:    clientConfig.Options,
		})
	}
	return manager.GetServers()
}
