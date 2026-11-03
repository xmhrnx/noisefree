package ui

import (
	"image"

	"github.com/xmhrnx/giode"
)

// presetTile is a selectable background thumbnail.
type presetTile struct {
	press    *giode.PressableElement
	name     string
	thumb    image.Image
	selected bool
}

func newPresetTile(name string, thumb image.Image, onClick func()) *presetTile {
	t := &presetTile{name: name, thumb: thumb}
	t.press = giode.Pressable(t.content).Label(name).OnClick(func(giode.ClickEvent) { onClick() })
	return t
}

func (t *presetTile) content(s giode.PressState) giode.Element {
	border := "theme.border"
	switch {
	case t.selected:
		border = "theme.primary"
	case s.Hovered:
		border = "theme.muted"
	}
	return giode.Stack(giode.Styles{Gap: 6, Align: giode.AlignCenter},
		giode.Box(giode.Styles{BorderWidth: 2, BorderColor: border, BorderRadius: 10, Padding: giode.UniformInset(2)},
			giode.Image(t.thumb, giode.Styles{Width: 96, Height: 54, BorderRadius: 8, Fit: giode.FitCover})),
		giode.Text(t.name, caption))
}

// View lays the tile out.
func (t *presetTile) View(selected bool) giode.Element {
	t.selected = selected
	return t.press
}
