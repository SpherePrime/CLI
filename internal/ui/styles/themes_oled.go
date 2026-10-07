package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

func CharcoalOLED() Styles {
	return pantheraBase(quickStyleOpts{
		primary:   colortone.Guppy,
		secondary: colortone.Malibu,
		accent:    colortone.Sardine,
		keyword:   colortone.Flamingo,

		fgBase:       colortone.Sash,
		fgMoreSubtle: colortone.Squid,
		fgSubtle:     colortone.Smoke,
		fgMostSubtle: colortone.Oyster,

		onPrimary: colortone.Butter,

		bgBase:         colortone.Pepper,
		bgLeastVisible: colortone.BBQ,
		bgLessVisible:  colortone.Char,
		bgMostVisible:  colortone.Iron,

		separator: colortone.Char,

		destructive:       colortone.Coral,
		error:             colortone.Sriracha,
		warningSubtle:     colortone.Zest,
		warning:           colortone.Mustard,
		attention:         colortone.Tang,
		busy:              colortone.Citron,
		info:              colortone.Guppy,
		infoMoreSubtle:    colortone.Malibu,
		infoMostSubtle:    colortone.Damson,
		success:           colortone.Julep,
		successMoreSubtle: colortone.Bok,
		successMostSubtle: colortone.Guac,
		yolo:              colortone.Zest,
		plan:              colortone.Guppy,
		planMoreSubtle:    colortone.Malibu,

		ansiBlack:   colortone.BBQ,
		ansiRed:     colortone.Coral,
		ansiGreen:   colortone.Guac,
		ansiYellow:  colortone.Mustard,
		ansiBlue:    colortone.Guppy,
		ansiMagenta: colortone.Malibu,
		ansiCyan:    colortone.Malibu,
		ansiWhite:   colortone.Smoke,

		ansiBrightBlack:   colortone.Iron,
		ansiBrightRed:     colortone.Tuna,
		ansiBrightGreen:   colortone.Julep,
		ansiBrightYellow:  colortone.Zest,
		ansiBrightBlue:    colortone.Guppy,
		ansiBrightMagenta: colortone.Flamingo,
		ansiBrightCyan:    colortone.Sardine,
		ansiBrightWhite:   colortone.Salt,
	})
}
