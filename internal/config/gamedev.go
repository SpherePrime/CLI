package config

func (c *Config) SetGameDevOverride(v *bool) {
	if c == nil {
		return
	}
	if v == nil {
		c.gameDevOverride = nil
		return
	}
	val := *v
	c.gameDevOverride = &val
}

func (c *Config) IsGameDevEnabled(sessionID string) bool {
	if c == nil {
		return false
	}
	if c.gameDevOverride != nil {
		return *c.gameDevOverride
	}
	if sessionID != "" && c.SessionGameDev != nil {
		if enabled, ok := c.SessionGameDev[sessionID]; ok {
			return enabled
		}
	}
	if c.Options != nil {
		return c.Options.GameDev
	}
	return false
}

func (s *ConfigStore) IsGameDevEnabled(sessionID string) bool {
	if s == nil {
		return false
	}
	s.writeMu.RLock()
	override := s.overrides.GameDev
	s.writeMu.RUnlock()
	if override != nil {
		return *override
	}
	return s.Config().IsGameDevEnabled(sessionID)
}

func (s *ConfigStore) OverrideGameDev(enabled bool) {
	s.mutateInMemory(func(c *Config) {
		v := enabled
		s.overrides.GameDev = &v
		c.SetGameDevOverride(&v)
		if c.Options == nil {
			c.Options = &Options{}
		}
		c.Options.GameDev = enabled
	})
}

func (s *ConfigStore) UpdateGameDevConfig(scope Scope, sessionID string, enabled bool) error {
	if scope != ScopeGlobal && scope != ScopeWorkspace {
		scope = ScopeGlobal
	}
	return s.update(scope, func(c *Config) map[string]any {
		s.overrides.GameDev = nil
		c.SetGameDevOverride(nil)
		if sessionID != "" {
			if c.SessionGameDev == nil {
				c.SessionGameDev = make(map[string]bool)
			}
			c.SessionGameDev[sessionID] = enabled
			return map[string]any{"session_game_dev": c.SessionGameDev}
		}
		if c.Options == nil {
			c.Options = &Options{}
		}
		c.Options.GameDev = enabled
		return map[string]any{"options.game_dev": enabled}
	})
}
