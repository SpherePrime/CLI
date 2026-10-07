package model

import (
	"github.com/SpherePrime/CLI/internal/appearance"
	"image"
)

func (m *UI) customDesign() bool {
	return m.com.Styles.Design != "" && m.com.Styles.Design != "classic"
}

func (m *UI) designColumns(width, height int) (center, sidebar, inspector image.Rectangle) {
	area := image.Rect(0, 0, max(width, 0), max(height, 0))
	center = insetRight(insetLeft(area, appMargin), appMargin)
	spec := appearance.DesignSpec(m.com.Styles.Design)
	visible := !m.forceCompactMode && width >= compactModeWidthBreakpoint && height >= compactModeHeightBreakpoint
	if visible && spec.SidebarWidth > 0 {
		sideWidth := min(spec.SidebarWidth, max(0, center.Dx()-40))
		if spec.Sidebar == "left" {
			sidebar = center
			sidebar.Max.X = sidebar.Min.X + sideWidth
			center.Min.X = sidebar.Max.X + 1
		} else {
			sidebar = center
			sidebar.Min.X = sidebar.Max.X - sideWidth
			center.Max.X = sidebar.Min.X - 1
		}
	}
	if visible && width >= 150 && spec.InspectorWidth > 0 {
		inspector = center
		inspector.Min.X = inspector.Max.X - spec.InspectorWidth
		center.Max.X = inspector.Min.X - 1
	}
	if spec.MaxWidth > 0 && center.Dx() > spec.MaxWidth {
		left := center.Min.X + (center.Dx()-spec.MaxWidth)/2
		center.Min.X = left
		center.Max.X = left + spec.MaxWidth
	}
	return center, sidebar, inspector
}
func (m *UI) generateDesignLayout(width, height, editorHeight, helpHeight int) uiLayout {
	area := image.Rect(0, 0, max(width, 0), max(height, 0))
	l := uiLayout{area: area}
	statusHeight := min(max(helpHeight, 1), area.Dy())
	l.status = image.Rect(0, area.Max.Y-statusHeight, area.Max.X, area.Max.Y)
	app := insetRight(insetLeft(image.Rect(0, 0, area.Max.X, l.status.Min.Y), appMargin), appMargin)
	app = insetTop(app, appMargin)
	spec := appearance.DesignSpec(m.com.Styles.Design)
	headerHeight := min(spec.HeaderHeight, max(0, app.Dy()-1))
	l.header = image.Rect(app.Min.X, app.Min.Y, app.Max.X, app.Min.Y+headerHeight)
	body := app
	body.Min.Y = l.header.Max.Y
	if body.Dy() > 2 {
		body.Min.Y++
	}
	center, sidebar, inspector := m.designColumns(width, height)
	center.Min.Y = body.Min.Y
	center.Max.Y = body.Max.Y
	if spec.MaxWidth > 0 {
		l.header.Min.X = center.Min.X
		l.header.Max.X = center.Max.X
	}
	if sidebar.Dx() > 0 {
		sidebar.Min.Y = body.Min.Y
		sidebar.Max.Y = body.Max.Y
		l.sidebar = sidebar
	}
	if inspector.Dx() > 0 {
		inspector.Min.Y = body.Min.Y
		inspector.Max.Y = body.Max.Y
		l.inspector = inspector
	}
	editorHeight = fitLowerPane(editorHeight, center.Dy())
	chatArea := center
	l.editor = image.Rect(center.Min.X, center.Max.Y-editorHeight, center.Max.X, center.Max.Y)
	chatArea.Max.Y = l.editor.Min.Y
	if chatArea.Dy() > 2 {
		chatArea.Max.Y--
	}

	if spec.Sidebar == "bottom" && !m.forceCompactMode && width >= 80 && height >= 26 {
		dockHeight := fitLowerPane(4, chatArea.Dy())
		l.sidebar = image.Rect(chatArea.Min.X, chatArea.Max.Y-dockHeight, chatArea.Max.X, chatArea.Max.Y)
		chatArea.Max.Y = l.sidebar.Min.Y
		if chatArea.Dy() > 2 {
			chatArea.Max.Y--
		}
	}
	pillsHeight := fitLowerPane(m.pillsAreaHeight(), chatArea.Dy())
	if pillsHeight > 0 {
		l.pills = image.Rect(chatArea.Min.X, chatArea.Max.Y-pillsHeight, chatArea.Max.X, chatArea.Max.Y)
		chatArea.Max.Y = l.pills.Min.Y
	}
	l.main = chatArea
	l.sessionDetails = image.Rect(center.Min.X, body.Min.Y, center.Max.X, min(body.Max.Y, body.Min.Y+sessionDetailsMaxHeight))
	return l.withinArea()
}
func (m *UI) applyDesignLayout(l uiLayout) uiLayout { return l }
