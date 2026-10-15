package common

// KeyInfo describes one rebindable keybinding action for display: the name
// that goes in options.keybindings, the keys it answers to by default, and
// the short label the help line shows.
//
// It lives here rather than in the model package because the keybindings
// dialog must be able to render it, and the dialog package cannot import the
// model package (the model already imports the dialog for every other dialog).
// Keeping the data type in a package that both import is what makes the
// dialog self-contained.
type KeyInfo struct {
	Name        string
	Default     string
	Description string
}
