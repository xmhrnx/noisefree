package ui

import "github.com/xmhrnx/giode"

// swatch is a selectable colour circle.
type swatch struct {
	press    *giode.PressableElement
	color    string
	selected bool
}

func newSwatch(color string, onClick func()) *swatch {
	s := &swatch{color: color}
	s.press = giode.Pressable(s.content).Label(color).OnClick(func(giode.ClickEvent) { onClick() })
	return s
}

func (s *swatch) content(st giode.PressState) giode.Element {
	ring := "transparent"
	switch {
	case s.selected:
		ring = "theme.primary"
	case st.Hovered:
		ring = "theme.muted"
	}
	return giode.Box(giode.Styles{BorderWidth: 2, BorderColor: ring, BorderRadius: 99, Padding: giode.UniformInset(2)},
		giode.Box(giode.Styles{Width: 26, Height: 26, BorderRadius: 99, Background: s.color, BorderWidth: 1, BorderColor: "theme.border"}))
}

// View lays the swatch out.
func (s *swatch) View(selected bool) giode.Element {
	s.selected = selected
	return s.press
}
