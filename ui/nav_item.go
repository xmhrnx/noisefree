package ui

import "github.com/xmhrnx/giode"

// navItem is one sidebar entry.
type navItem struct {
	press  *giode.PressableElement
	icon   string
	label  string
	active func() bool
}

func newNavItem(iconName, label string, active func() bool, onClick func()) *navItem {
	n := &navItem{icon: iconName, label: label, active: active}
	n.press = giode.Pressable(n.content).Label(label).OnClick(func(giode.ClickEvent) { onClick() })
	return n
}

func (n *navItem) content(s giode.PressState) giode.Element {
	bg, fg, weight := "transparent", "theme.muted", giode.FontWeightMedium
	switch {
	case n.active():
		bg, fg, weight = tintPrimary, "theme.text", giode.FontWeightSemiBold
	case s.Hovered:
		bg, fg = hover, "theme.text"
	}
	iconColor := fg
	if n.active() {
		iconColor = "theme.primary"
	}
	return giode.Row(giode.Styles{Width: -1, Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Background: bg, BorderRadius: 10, Padding: giode.SymmetricInset(12, 10)},
		icon(n.icon, 18, iconColor),
		giode.Text(n.label, giode.Styles{FontSize: 13.5, FontWeight: weight, Color: fg}))
}

// View lays the item out.
func (n *navItem) View() giode.Element { return n.press }
