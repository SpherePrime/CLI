package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

func RosePine() Styles {
	return pantheraBase(quickStyleOpts{
		primary:   colortone.Prince,
		secondary: colortone.Mauve,
		accent:    colortone.Malibu,
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
		info:              colortone.Prince,
		infoMoreSubtle:    colortone.Mauve,
		infoMostSubtle:    colortone.Damson,
		success:           colortone.Violet,
		successMoreSubtle: colortone.Mauve,
		successMostSubtle: colortone.Lilac,
		yolo:              colortone.Zest,
		plan:              colortone.Prince,
		planMoreSubtle:    colortone.Mauve,

		ansiBlack:   colortone.BBQ,
		ansiRed:     colortone.Coral,
		ansiGreen:   colortone.Guac,
		ansiYellow:  colortone.Mustard,
		ansiBlue:    colortone.Prince,
		ansiMagenta: colortone.Mauve,
		ansiCyan:    colortone.Malibu,
		ansiWhite:   colortone.Smoke,

		ansiBrightBlack:   colortone.Iron,
		ansiBrightRed:     colortone.Tuna,
		ansiBrightGreen:   colortone.Julep,
		ansiBrightYellow:  colortone.Zest,
		ansiBrightBlue:    colortone.Prince,
		ansiBrightMagenta: colortone.Mauve,
		ansiBrightCyan:    colortone.Sardine,
		ansiBrightWhite:   colortone.Salt,
	})
}
