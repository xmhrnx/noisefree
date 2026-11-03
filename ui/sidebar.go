package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
)

// Pages of the app.
const (
	pageHome = iota
	pageEngines
	pageSettings
)

// sidebar is the navigation column with the live status card at the bottom.
type sidebar struct {
	a     *App
	items []*navItem
}

func newSidebar(a *App) *sidebar {
	s := &sidebar{a: a}
	add := func(iconName, label string, page int) {
		s.items = append(s.items, newNavItem(iconName, label,
			func() bool { return a.page == page }, func() { a.page = page }))
	}
	add("mic", "Microphone", pageHome)
	add("blocks", "Engines", pageEngines)
	add("settings", "Settings", pageSettings)
	return s
}

// View lays out the sidebar.
func (s *sidebar) View() giode.Element {
	nav := []giode.Element{s.brand(), giode.Box(giode.Styles{Height: 18})}
	for _, it := range s.items {
		nav = append(nav, it.View())
	}
	nav = append(nav, spacer(), s.status())
	return giode.Row(giode.Styles{Height: -1, Wrap: giode.WrapNoWrap},
		giode.Stack(giode.Styles{Width: 236, Height: -1, Gap: 4, Padding: giode.UniformInset(16), Background: sidebarBg}, nav...),
		vline())
}

func (s *sidebar) brand() giode.Element {
	return giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(4, 6)},
		logo(36),
		giode.Stack(giode.Styles{Gap: 1},
			giode.Text("NoiseFree", giode.Styles{FontSize: 18, FontWeight: giode.FontWeightBold, Color: "theme.text"}),
			giode.Text("AI noise removal", overline)))
}

func (s *sidebar) status() giode.Element {
	live := s.a.live()
	dot, title := "theme.muted", "Paused"
	switch {
	case s.a.svc.AudioError() != nil:
		dot, title = "theme.danger", "Audio error"
	case live && s.a.svc.Settings().Enabled:
		dot, title = "theme.success", "Live"
	case live:
		dot, title = "theme.warning", "Bypassed"
	}
	st := s.a.svc.Pipe.Stats()
	detail := "Audio stopped"
	if live {
		detail = fmt.Sprintf("%s · %.0f ms", s.a.engineName(), st.LatencyMs)
	}
	return giode.Stack(giode.Styles{Gap: 6, Padding: giode.UniformInset(14), Background: "theme.surface",
		BorderRadius: 12, BorderWidth: 1, BorderColor: "theme.border", Width: -1},
		giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 8, Height: 8, BorderRadius: 4, Background: dot}),
			giode.Text(title, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"})),
		giode.Text(detail, giode.Styles{FontSize: 12, Color: "theme.muted", TextOverflow: giode.TextOverflowEllipsis}),
	)
}
