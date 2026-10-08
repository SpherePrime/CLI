package model

import "github.com/SpherePrime/CLI/internal/ui/styles"

func resourceStateBadge(theme *styles.Styles, state string) string {
	badge := theme.Resource.DisabledIcon
	switch state {
	case "ON":
		badge = theme.Resource.OnlineIcon
	case "WAIT":
		badge = theme.Resource.BusyIcon
	case "ERR":
		badge = theme.Resource.ErrorIcon
	case "AUTH":
		badge = theme.Resource.NeedsAuthIcon
	}
	return badge.SetString("[" + state + "]").String()
}
