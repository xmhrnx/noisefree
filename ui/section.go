package ui

import "github.com/xmhrnx/giode"

// section is a titled card: an icon badge, a title with a subtitle, an
// optional trailing control, then the content.
func section(iconName, title, subtitle string, trailing giode.Element, kids ...giode.Element) giode.Element {
	head := []giode.Element{
		giode.Box(giode.Styles{Background: tintPrimary, BorderRadius: 10, Padding: giode.UniformInset(8)},
			icon(iconName, 18, "theme.primary")),
		giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
			giode.Text(title, h2),
			giode.Text(subtitle, muted)),
	}
	if trailing != nil {
		head = append(head, trailing)
	}
	all := append([]giode.Element{giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, head...)}, kids...)
	return giode.Box(cardStyle, all...)
}
