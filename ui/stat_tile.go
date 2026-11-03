package ui

import "github.com/xmhrnx/giode"

// statTile shows one live number with an icon.
func statTile(iconName, label, value, color, tint string) giode.Element {
	return giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, FlexGrow: 1,
		Background: raised, BorderRadius: 12, BorderWidth: 1, BorderColor: "theme.border", Padding: giode.SymmetricInset(12, 10)},
		giode.Box(giode.Styles{Background: tint, BorderRadius: 8, Padding: giode.UniformInset(6)}, icon(iconName, 15, color)),
		giode.Stack(giode.Styles{Gap: 1},
			giode.Text(label, caption),
			giode.Text(value, giode.Styles{FontSize: 14, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"})))
}
