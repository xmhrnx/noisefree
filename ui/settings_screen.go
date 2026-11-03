package ui

import (
	"context"
	"net/url"
	"path/filepath"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// settingsScreen holds app-wide preferences.
type settingsScreen struct {
	a                                   *App
	theme, buffer                       *segmented.Control
	closeToTray, atLogin, vmicOn, guard *toggle.Switch
	openFolder                          *button.Button
}

func newSettingsScreen(a *App) *settingsScreen {
	s := &settingsScreen{a: a}
	set := a.svc.Settings()
	s.theme = giode.Input.Segmented("Dark", "Light").OnChange(func(i int) {
		theme := []string{"dark", "light"}[i]
		a.svc.Update(func(st *settings.Settings) { st.Theme = theme })
		applyTheme(theme)
	})
	if set.Theme == "light" {
		s.theme.SetSelected(1)
	}
	s.buffer = giode.Input.Segmented("10 ms · lowest latency", "20 ms · lowest CPU").OnChange(func(i int) {
		a.svc.Update(func(st *settings.Settings) { st.BufferMs = []int{10, 20}[i] })
		a.restartAudio()
	})
	if set.BufferMs == 20 {
		s.buffer.SetSelected(1)
	}
	s.closeToTray = giode.Input.Switch("").OnChange(func(on bool) {
		a.svc.Update(func(st *settings.Settings) { st.CloseToTray = on })
		a.win.SetCloseToTray(on)
	})
	s.closeToTray.SetChecked(set.CloseToTray)
	s.atLogin = giode.Input.Switch("").OnChange(func(on bool) {
		if err := a.win.SetLaunchAtLogin(on); err != nil {
			a.toast(giode.ToastError, "Couldn't change start at login: "+err.Error())
		}
	})
	s.atLogin.SetChecked(a.win.LaunchAtLogin())
	s.vmicOn = giode.Input.Switch("").OnChange(func(on bool) {
		a.svc.Update(func(st *settings.Settings) { st.VirtualMic = on })
		a.restartAudio()
	})
	s.vmicOn.SetChecked(set.VirtualMic)
	s.guard = giode.Input.Switch("").OnChange(func(on bool) {
		a.svc.Update(func(st *settings.Settings) { st.ProtectDefaults = on })
	})
	s.guard.SetChecked(set.ProtectDefaults)
	s.openFolder = giode.Button("Open folder", secondaryButton()).Icon(icon("external-link", 15, "")).OnClick(func() {
		u := (&url.URL{Scheme: "file", Path: filepath.ToSlash(a.svc.Dir)}).String()
		go func() {
			if err := a.win.OpenURL(context.Background(), u); err != nil {
				a.win.Post(func() { a.toast(giode.ToastError, err.Error()) })
			}
		}()
	})
	return s
}

// applyTheme switches between the dark and light palettes.
func applyTheme(theme string) {
	if theme == "light" {
		giode.SetMode(giode.ModeLight)
		return
	}
	giode.SetMode(giode.ModeDark)
}

// View lays out the page.
func (s *settingsScreen) View() giode.Element {
	version := giode.AppVersion
	if version == "" {
		version = "development build"
	}
	divider := giode.Divider(giode.Styles{Color: "theme.border"})
	return giode.Stack(giode.Styles{Gap: 20, Width: -1, MaxWidth: 820},
		pageHeader("Settings", "Make NoiseFree fit your setup."),
		section("sun", "Appearance", "Pick a look.", nil,
			settingRow("Theme", "Dark suits streaming setups; light suits bright rooms.", s.theme)),
		section("activity", "Performance", "Trade a few milliseconds for even less CPU.", nil,
			settingRow("Audio buffer", "Larger buffers wake the CPU half as often.", s.buffer)),
		section("mic", "Virtual microphone", "Let other apps record your clean voice.", nil,
			settingRow("Use a virtual microphone", "On Linux NoiseFree creates one; on Windows and macOS it uses VB-Cable or BlackHole.", s.vmicOn),
			divider,
			settingRow("Keep my speakers and microphone as the defaults",
				"Virtual cable drivers often make themselves the system default when installed, which silences your computer. NoiseFree switches back to your real devices.", s.guard)),
		section("monitor", "Window", "How NoiseFree behaves in the background.", nil,
			settingRow("Keep running in the tray", "Closing the window keeps noise removal running.", s.closeToTray),
			divider,
			settingRow("Start at login", "Have clean audio ready before your first call.", s.atLogin)),
		section("info", "About NoiseFree "+version, "Open-source noise removal for every GPU and CPU.", nil,
			settingRow("Engines and settings", s.a.svc.Dir, s.openFolder),
			divider,
			giode.Text("NoiseFree is built with Giode and Gio. Bundled: RNNoise (BSD-3-Clause, Xiph.Org), miniaudio (public domain), Inter (OFL), Lucide icons (ISC). "+
				"Downloadable engines keep their own licenses, shown on their cards.", muted)),
	)
}
