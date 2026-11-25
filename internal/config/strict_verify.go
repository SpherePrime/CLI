package config

func (c *Config) SetStrictVerifyOverride(v *bool) {
	if c == nil {
		return
	}
	if v == nil {
		c.strictVerifyOverride = nil
		return
	}
	val := *v
	c.strictVerifyOverride = &val
}

func (c *Config) IsStrictVerifyEnabled(sessionID string) bool {
	if c == nil {
		return false
	}
	if c.strictVerifyOverride != nil {
		return *c.strictVerifyOverride
	}
	if sessionID != "" && c.SessionStrictVerify != nil {
		if enabled, ok := c.SessionStrictVerify[sessionID]; ok {
			return enabled
		}
	}
	if c.Options != nil {
		return c.Options.StrictVerify
	}
	return false
}

func (s *ConfigStore) IsStrictVerifyEnabled(sessionID string) bool {
	if s == nil {
		return false
	}
	s.writeMu.RLock()
	override := s.overrides.StrictVerify
	s.writeMu.RUnlock()
	if override != nil {
		return *override
	}
	return s.Config().IsStrictVerifyEnabled(sessionID)
}

func (s *ConfigStore) OverrideStrictVerify(enabled bool) {
	s.mutateInMemory(func(c *Config) {
		v := enabled
		s.overrides.StrictVerify = &v
		c.SetStrictVerifyOverride(&v)
		if c.Options == nil {
			c.Options = &Options{}
		}
		c.Options.StrictVerify = enabled
	})
}

func (s *ConfigStore) UpdateStrictVerifyConfig(scope Scope, sessionID string, enabled bool) error {
	if scope != ScopeGlobal && scope != ScopeWorkspace {
		scope = ScopeGlobal
	}
	return s.update(scope, func(c *Config) map[string]any {
		s.overrides.StrictVerify = nil
		c.SetStrictVerifyOverride(nil)
		if sessionID != "" {
			if c.SessionStrictVerify == nil {
				c.SessionStrictVerify = make(map[string]bool)
			}
			c.SessionStrictVerify[sessionID] = enabled
			return map[string]any{"session_strict_verify": c.SessionStrictVerify}
		}
		if c.Options == nil {
			c.Options = &Options{}
		}
		c.Options.StrictVerify = enabled
		return map[string]any{"options.strict_verify": enabled}
	})
}
