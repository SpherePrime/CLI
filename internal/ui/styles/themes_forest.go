package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

func Forest() Styles {
	return pantheraBase(quickStyleOpts{
		primary:   colortone.Guac,
		secondary: colortone.Guppy,
		accent:    colortone.Turtle,
		keyword:   colortone.Julep,

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
		info:              colortone.Guac,
		infoMoreSubtle:    colortone.Turtle,
		infoMostSubtle:    colortone.Damson,
		success:           colortone.Julep,
		successMoreSubtle: colortone.Bok,
		successMostSubtle: colortone.Guac,
		yolo:              colortone.Citron,
		plan:              colortone.Guac,
		planMoreSubtle:    colortone.Turtle,

		ansiBlack:   colortone.BBQ,
		ansiRed:     colortone.Coral,
		ansiGreen:   colortone.Guac,
		ansiYellow:  colortone.Mustard,
		ansiBlue:    colortone.Guac,
		ansiMagenta: colortone.Tang,
		ansiCyan:    colortone.Turtle,
		ansiWhite:   colortone.Smoke,

		ansiBrightBlack:   colortone.Iron,
		ansiBrightRed:     colortone.Tuna,
		ansiBrightGreen:   colortone.Julep,
		ansiBrightYellow:  colortone.Zest,
		ansiBrightBlue:    colortone.Guac,
		ansiBrightMagenta: colortone.Tang,
		ansiBrightCyan:    colortone.Sardine,
		ansiBrightWhite:   colortone.Salt,
	})
}
