package ui

import "github.com/xmhrnx/giode"

// settingRow lays out a labelled control: title and description on the
// left, the control on the right.
func settingRow(title, desc string, control giode.Element) giode.Element {
	return giode.Row(giode.Styles{Gap: 16, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
		giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
			giode.Text(title, giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightMedium, Color: "theme.text"}),
			giode.Text(desc, muted)),
		control)
}

// labelled stacks a caption above a full-width control.
func labelled(label string, control giode.Element) giode.Element {
	return giode.Stack(giode.Styles{Gap: 8, Width: -1}, giode.Text(label, overline), control)
}
