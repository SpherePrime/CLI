package model

import (
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"image/color"
)

type backgroundScreen struct {
	uv.Screen
	background color.Color
}

func (screen backgroundScreen) SetCell(x, y int, cell *uv.Cell) {
	updated := uv.EmptyCell
	if cell != nil {
		updated = *cell
	}
	if updated.Style.Bg == nil {
		updated.Style.Bg = screen.background
	}
	screen.Screen.SetCell(x, y, &updated)
}
