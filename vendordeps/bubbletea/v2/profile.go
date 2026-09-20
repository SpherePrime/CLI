package tea

import "github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/colorprofile"

// ColorProfileMsg is a message that describes the terminal's color profile.
// This message is send to the program's update function when the program is
// started.
//
// To upgrade the terminal color profile, use the `tea.RequestCapability`
// command to request the `RGB` and `Tc` terminfo capabilities. Bubble Tea will
// then cache the terminal's color profile and send a `ColorProfileMsg` to the
// program's update function.
type ColorProfileMsg struct {
	colorprofile.Profile
}
