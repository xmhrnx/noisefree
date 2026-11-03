// Package ui is NoiseFree's interface, built with Giode.
package ui

import (
	"sync/atomic"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/scroll"
	"github.com/xmhrnx/giode/components/titlebar"
	"github.com/xmhrnx/giode/components/toaster"
	"github.com/xmhrnx/noisefree/internal/core"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// App is the root of the interface and owns the state shared by its pages.
type App struct {
	win *giode.App
	svc *core.Service

	page     int
	sidebar  *sidebar
	home     *homeScreen
	camera   *cameraScreen
	engines  *enginesScreen
	settings *settingsScreen
	content  *scroll.Scroll
	bar      *titlebar.TitleBar
	toasts   *toaster.Toaster
	loading  bool
	// onHome and onCamera mirror the page for the redraw ticker's goroutine.
	onHome, onCamera atomic.Bool
}

// New builds the interface. Call it after giode.New and before Run.
func New(win *giode.App, svc *core.Service) *App {
	a := &App{win: win, svc: svc}
	applyTheme(svc.Settings().Theme)
	a.toasts = giode.Toaster().Placement(giode.ToastBottomEnd).Duration(5 * time.Second).Max(3)
	a.sidebar = newSidebar(a)
	a.home = newHomeScreen(a)
	a.engines = newEnginesScreen(a)
	a.camera = newCameraScreen(a)
	a.settings = newSettingsScreen(a)
	svc.OnNotice = func(msg string) {
		win.Post(func() {
			a.toast(giode.ToastWarning, msg)
			a.home.devices.refresh()
		})
	}
	a.content = giode.Scroll(a.pageView, giode.Styles{ScrollBar: giode.ScrollBar{Width: 6, ThumbColor: track,
		TrackColor: "transparent", Inset: 4, AutoHide: true}})
	if useCustomTitleBar {
		a.bar = giode.TitleBar(win, "", giode.Styles{Background: sidebarBg, Height: 38}).Variant(titleBarVariant)
	}
	return a
}

// Start loads the engine and opens audio in the background, then keeps the
// meters moving while the window is visible. Call it from Config.OnReady.
func (a *App) Start() {
	a.home.devices.refresh()
	a.loading = true
	go func() {
		warn, engineErr := a.svc.LoadEngine()
		audioErr := a.svc.Restart()
		a.win.Post(func() {
			a.loading = false
			a.home.suppression.refresh()
			a.home.devices.refresh()
			switch {
			case engineErr != nil:
				a.toast(giode.ToastError, "Couldn't load the engine: "+engineErr.Error())
			case warn != "":
				a.toast(giode.ToastWarning, warn)
			}
			if audioErr != nil {
				a.toast(giode.ToastError, audioErr.Error())
			}
		})
	}()
	go a.tick()
	go a.svc.Watch()
	a.camera.refreshCameras()
	if a.svc.Settings().Video.Enabled {
		a.camera.restart()
	}
}

// tick redraws 20 times a second while the live meters are on screen, and
// with each new frame (up to 30 a second) on the camera page; otherwise
// (other pages, minimized, in the tray) the UI sleeps and costs nothing.
func (a *App) tick() {
	t := time.NewTicker(33 * time.Millisecond)
	defer t.Stop()
	var lastFrame uint64
	var n int
	for range t.C {
		n++
		if !a.win.WindowVisible() {
			continue
		}
		if a.onCamera.Load() {
			// the camera page redraws only when a new frame arrived
			if _, seq := a.svc.Video.Preview(); seq != lastFrame {
				lastFrame = seq
				a.win.Invalidate()
			}
			continue
		}
		if !a.onHome.Load() || !a.svc.Pipe.Running() || n%3 == 0 { // meters at 20 Hz
			continue
		}
		in, out := a.svc.Pipe.In.RMS(), a.svc.Pipe.Out.RMS()
		a.win.Post(func() { a.home.activity.graph.push(in, out) })
	}
}

// View is the root view.
func (a *App) View() giode.Element {
	a.onHome.Store(a.page == pageHome)
	a.onCamera.Store(a.page == pageCamera)
	body := giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
		a.sidebar.View(),
		giode.Box(giode.Styles{FlexGrow: 1, Height: -1, Width: -1}, a.content))
	kids := []giode.Element{}
	if a.bar != nil {
		kids = append(kids, a.bar)
	}
	kids = append(kids, body, a.toasts)
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: "theme.background", Color: "theme.text"}, kids...)
}

func (a *App) pageView() giode.Element {
	var page giode.Element
	switch a.page {
	case pageCamera:
		page = a.camera.View()
	case pageEngines:
		page = a.engines.View()
	case pageSettings:
		page = a.settings.View()
	default:
		page = a.home.View()
	}
	return giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Top: 28, Right: 32, Bottom: 40, Left: 32}}, page)
}

func (a *App) live() bool { return a.svc.Pipe.Running() }

func (a *App) engineName() string {
	id, _, _ := a.svc.ActiveEngine()
	if e, ok := a.svc.Store.Get(id); ok {
		return e.Name
	}
	return "No engine"
}

func (a *App) toast(kind giode.ToastKind, msg string) { a.toasts.Show(kind, msg) }

func (a *App) toastAction(kind giode.ToastKind, msg, label string, fn func()) {
	a.toasts.ShowAction(kind, msg, label, fn)
}

// toggleEnabled flips noise removal on or off.
func (a *App) toggleEnabled() {
	on := !a.svc.Settings().Enabled
	a.svc.Update(func(s *settings.Settings) { s.Enabled = on })
	a.updateTray()
	if !a.live() {
		a.restartAudio()
	}
}

// useEngine makes id the active engine.
func (a *App) useEngine(id string) {
	a.svc.Update(func(s *settings.Settings) { s.Engine = id })
	a.home.suppression.refresh()
	a.applyEngine()
}

// applyEngine loads the engine from the settings in the background.
func (a *App) applyEngine() {
	go func() {
		warn, err := a.svc.LoadEngine()
		a.win.Post(func() {
			a.home.suppression.refresh()
			switch {
			case err != nil:
				a.toast(giode.ToastError, "Couldn't load the engine: "+err.Error())
			case warn != "":
				a.toast(giode.ToastWarning, warn)
			default:
				_, backend, _ := a.svc.ActiveEngine()
				a.toast(giode.ToastSuccess, a.engineName()+" is running on the "+backend+".")
			}
		})
	}()
}

// restartAudio reopens the devices in the background.
func (a *App) restartAudio() {
	go func() {
		err := a.svc.Restart()
		a.win.Post(func() {
			a.home.devices.refresh()
			a.home.activity.graph.reset()
			if err != nil {
				a.toast(giode.ToastError, err.Error())
			}
		})
	}()
}

// SetupTray adds the tray icon and its menu.
func (a *App) SetupTray(iconPNG []byte) {
	if !giode.TraySupported() {
		return
	}
	a.win.Tray().SetIcon(iconPNG).SetTooltip("NoiseFree · noise removal")
	a.updateTray()
}

func (a *App) updateTray() {
	if !giode.TraySupported() {
		return
	}
	a.win.Tray().SetMenu(
		giode.TrayAction("Open NoiseFree", a.win.ShowWindow),
		giode.TrayCheckbox("Noise removal", a.svc.Settings().Enabled, a.toggleEnabled),
		giode.TraySeparator(),
		giode.TrayAction("Quit", a.win.Quit),
	)
}

// AudioStopped is called when a device disappears (unplugged headset).
func (a *App) AudioStopped() {
	a.win.Post(func() {
		a.toast(giode.ToastWarning, "An audio device stopped. Reconnecting…")
		a.restartAudio()
	})
}
