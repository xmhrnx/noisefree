package ui

import "github.com/xmhrnx/giode"

// homeScreen is the main page: the toggle, live activity and controls.
type homeScreen struct {
	hero         *heroCard
	activity     *activityCard
	suppression  *suppressionCard
	devices      *devicesCard
	enhancements *enhancementsCard
	vmic         *virtualMicCard
}

func newHomeScreen(a *App) *homeScreen {
	return &homeScreen{
		hero:         newHeroCard(a),
		activity:     &activityCard{a: a},
		suppression:  newSuppressionCard(a),
		devices:      newDevicesCard(a),
		enhancements: newEnhancementsCard(a),
		vmic:         newVirtualMicCard(a),
	}
}

// View lays the cards out in two columns, or one on narrow windows.
func (h *homeScreen) View() giode.Element {
	return giode.Stack(giode.Styles{Gap: 20, Width: -1},
		pageHeader("Microphone", "Crystal-clear voice on any GPU, any CPU."),
		giode.Responsive(
			giode.Breakpoint{MaxWidth: 880, Name: "compact", View: func() giode.Element {
				return giode.Stack(giode.Styles{Gap: 20, Width: -1},
					h.hero.View(), h.activity.View(),
					h.suppression.View(), h.devices.View(), h.vmic.View(), h.enhancements.View())
			}},
			giode.Breakpoint{View: func() giode.Element {
				col := giode.Styles{Gap: 20, FlexGrow: 1, Width: -1}
				return giode.Stack(giode.Styles{Gap: 20, Width: -1},
					giode.Row(giode.Styles{Gap: 20, Width: -1, Wrap: giode.WrapNoWrap, Align: giode.AlignStretch},
						giode.Box(giode.Styles{Width: 420}, h.hero.View()),
						giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, h.activity.View())),
					giode.Row(giode.Styles{Gap: 20, Width: -1, Wrap: giode.WrapNoWrap},
						giode.Stack(col, h.suppression.View(), h.enhancements.View()),
						giode.Stack(col, h.devices.View(), h.vmic.View())))
			}},
		),
	)
}

// pageHeader is the title at the top of each page.
func pageHeader(title, subtitle string) giode.Element {
	return giode.Stack(giode.Styles{Gap: 4},
		giode.Text(title, h1),
		giode.Text(subtitle, giode.Styles{FontSize: 14, Color: "theme.muted"}))
}
