package ui

import (
	"fmt"
	"math"

	"github.com/xmhrnx/giode"
)

func percent(v float32) string { return fmt.Sprintf("%d%%", int(math.Round(float64(v*100)))) }

func dB(v float32) string {
	if v > 0 {
		return fmt.Sprintf("+%.0f dB", v)
	}
	return fmt.Sprintf("%.0f dB", v)
}

// pill is a small rounded label.
func pill(text, fg, bg string) giode.Element {
	return giode.Box(giode.Styles{Background: bg, BorderRadius: 99, Padding: giode.SymmetricInset(9, 3)},
		giode.Text(text, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: fg}))
}

// iconPill is a pill with a leading icon.
func iconPill(iconName, text, fg, bg string) giode.Element {
	return giode.Row(giode.Styles{Background: bg, BorderRadius: 99, Padding: giode.SymmetricInset(9, 4), Gap: 5,
		Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		icon(iconName, 12, fg),
		giode.Text(text, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: fg}))
}

// spacer grows to fill a row.
func spacer() giode.Element { return giode.Box(giode.Styles{FlexGrow: 1}) }

// vline is a 1dp vertical separator.
func vline() giode.Element {
	return giode.Box(giode.Styles{Width: 1, Height: -1, Background: "theme.border"})
}
