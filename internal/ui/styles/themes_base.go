package styles

func pantheraBase(o quickStyleOpts) Styles {
	s := quickStyle(o)

	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(o.onPrimary).
		Background(o.primary)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(o.primary)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(o.fgMoreSubtle)

	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(o.primary)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(o.bgMostVisible)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(o.accent)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(o.fgMoreSubtle)

	s.Messages.SubduedHypercreditIcon = s.Messages.SubduedHypercreditIcon.
		Foreground(o.secondary)

	return s
}
