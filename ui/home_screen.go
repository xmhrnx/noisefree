package ui

import (
	"gioui.org/layout"
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

// homeScreen is the main page: the toggle, live activity, preview and
// controls.
type homeScreen struct {
	hero         *heroCard
	activity     *activityCard
	preview      *previewCard
	suppression  *suppressionCard
	devices      *devicesCard
	enhancements *enhancementsCard
	vmic         *virtualMicCard
}

func newHomeScreen(a *App) *homeScreen {
	return &homeScreen{
		hero:         newHeroCard(a),
		activity:     &activityCard{a: a},
		preview:      newPreviewCard(a),
		suppression:  newSuppressionCard(a),
		devices:      newDevicesCard(a),
		enhancements: newEnhancementsCard(a),
		vmic:         newVirtualMicCard(a),
	}
}

// Cards side by side share a row and stretch to its height, so their
// edges line up; on narrow windows they stack.
var (
	stackedCard = cardStyle.With(giode.Styles{Width: -1})
)

// View lays the cards out in rows of two, or one column on narrow windows.
func (h *homeScreen) View() giode.Element {
	return giode.Stack(giode.Styles{Gap: 20, Width: -1},
		pageHeader("Microphone", "Crystal-clear voice on any GPU, any CPU."),
		giode.Responsive(
			giode.Breakpoint{MaxWidth: 880, Name: "compact", View: func() giode.Element {
				centered := stackedCard.With(giode.Styles{Align: giode.AlignCenter})
				return giode.Stack(giode.Styles{Gap: 20, Width: -1},
					h.hero.View(centered), h.activity.View(stackedCard), h.preview.View(stackedCard),
					h.suppression.View(stackedCard), h.devices.View(stackedCard),
					h.vmic.View(stackedCard), h.enhancements.View(stackedCard))
			}},
			giode.Breakpoint{View: func() giode.Element {
				return giode.Stack(giode.Styles{Gap: 20, Width: -1},
					pair(420, h.hero.View, h.activity.View),
					pair(0, h.suppression.View, h.devices.View),
					pair(0, h.preview.View, h.vmic.View),
					h.enhancements.View(stackedCard))
			}},
		),
	)
}

const pairGap = 20

// pair puts two cards side by side at equal height. The row measures its
// width first and gives each card an exact width: a stretched row sizes
// its height from its children, and wrapping text must be measured at the
// width it's finally drawn at. firstWidth fixes the first card's width in
// dp; 0 splits the row evenly.
func pair(firstWidth int, a, b func(giode.Styles) giode.Element) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		total := int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
		w1 := firstWidth
		if w1 == 0 {
			w1 = (total - pairGap) / 2
		}
		w2 := total - pairGap - w1
		first := cardStyle.With(giode.Styles{Width: properties.Width(w1)})
		if firstWidth != 0 { // the hero card centers its content
			first = first.With(giode.Styles{Align: giode.AlignCenter, Justify: giode.JustifyCenter})
		}
		second := cardStyle.With(giode.Styles{Width: properties.Width(w2)})
		return giode.Row(giode.Styles{Gap: pairGap, Width: -1, Wrap: giode.WrapNoWrap, Align: giode.AlignStretch},
			a(first), b(second)).Layout(gtx)
	})
}

// pageHeader is the title at the top of each page.
func pageHeader(title, subtitle string) giode.Element {
	return giode.Stack(giode.Styles{Gap: 4},
		giode.Text(title, h1),
		giode.Text(subtitle, giode.Styles{FontSize: 14, Color: "theme.muted"}))
}
