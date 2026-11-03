package ui

import (
	"context"
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/noisefree/internal/catalog"
)

// enginesScreen is the engine library: browse, install, remove, add custom.
type enginesScreen struct {
	a          *App
	filter     *segmented.Control
	add        *button.Button
	gpuInstall *button.Button
	installing map[string]installState
	dialog     *addEngineDialog
}

func newEnginesScreen(a *App) *enginesScreen {
	s := &enginesScreen{a: a, installing: map[string]installState{}}
	s.filter = giode.Input.Segmented("All", "Installed", "GPU")
	s.add = giode.Button("Add engine", primaryButton()).Icon(icon("plus", 15, "")).OnClick(func() { s.dialog.open = true })
	s.gpuInstall = giode.Button("Install pack", primaryButton()).Icon(icon("download", 15, "")).
		OnClick(func() { s.install("onnxruntime-gpu") })
	s.dialog = newAddEngineDialog(a)
	return s
}

// install downloads an entry in the background, showing progress on its card.
func (s *enginesScreen) install(id string) {
	if _, busy := s.installing[id]; busy {
		return
	}
	e, _ := s.a.svc.Store.Get(id)
	s.installing[id] = installState{status: "Starting…"}
	go func() {
		err := s.a.svc.Store.Install(context.Background(), id, func(f float64, status string) {
			s.a.win.Post(func() { s.installing[id] = installState{progress: f, status: status} })
		})
		s.a.win.Post(func() {
			delete(s.installing, id)
			s.a.home.suppression.refresh()
			if err != nil {
				s.a.toast(giode.ToastError, "Install failed: "+err.Error())
				return
			}
			if e.Kind == catalog.KindRuntime {
				s.a.toast(giode.ToastSuccess, e.Name+" installed. Turn on GPU acceleration on the Microphone page.")
				return
			}
			s.a.toastAction(giode.ToastSuccess, e.Name+" is ready.", "Use now", func() { s.a.useEngine(id) })
		})
	}()
}

// remove uninstalls an entry, switching away from it first if it's active.
func (s *enginesScreen) remove(id string) {
	e, _ := s.a.svc.Store.Get(id)
	if s.a.svc.Settings().Engine == id {
		s.a.useEngine("rnnoise")
	}
	if err := s.a.svc.Store.Remove(id); err != nil {
		s.a.toast(giode.ToastError, "Couldn't remove "+e.Name+": "+err.Error())
		return
	}
	s.a.home.suppression.refresh()
	s.a.toast(giode.ToastInfo, e.Name+" removed.")
}

// View lays out the page.
func (s *enginesScreen) View() giode.Element {
	store := s.a.svc.Store
	var cards []giode.Element
	for _, e := range store.Entries() {
		if e.Hidden || !s.matches(e) {
			continue
		}
		card := giode.Keyed(s, e.ID, func() *engineCard { return newEngineCard(s.a, e.ID) })
		cards = append(cards, card.View(e))
	}
	if len(cards) == 0 {
		cards = append(cards, giode.Text("No engines match this filter.", muted))
	}

	return giode.Stack(giode.Styles{Gap: 20, Width: -1},
		giode.Row(giode.Styles{Gap: 16, Align: giode.AlignEnd, Wrap: giode.WrapNoWrap, Width: -1},
			pageHeader("Engines", "Install the noise suppressor that suits your voice and your hardware."),
			spacer(), s.add),
		giode.Row(giode.Styles{Wrap: giode.WrapNoWrap}, s.filter),
		s.gpuPack(),
		giode.Row(giode.Styles{Gap: 20, Width: -1, Align: giode.AlignStretch}, cards...),
		s.dialog.View(),
	)
}

func (s *enginesScreen) matches(e catalog.Entry) bool {
	switch s.filter.Selected() {
	case 1:
		return s.a.svc.Store.Installed(e)
	case 2:
		return e.GPU
	}
	return true
}

// gpuPack is the banner offering the GPU runtime where one is needed.
func (s *enginesScreen) gpuPack() giode.Element {
	if !catalog.GPUSupported() || !catalog.GPUNeedsPack() {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	e, _ := s.a.svc.Store.Get("onnxruntime-gpu")
	var action giode.Element = s.gpuInstall
	switch st, busy := s.installing[e.ID]; {
	case busy:
		action = giode.Stack(giode.Styles{Gap: 6, Width: 200},
			giode.Text(st.status, caption),
			giode.Progress(float32(st.progress), giode.Styles{Color: "theme.primary", Background: track, Height: 6}))
	case s.a.svc.Store.Installed(e):
		action = iconPill("check", "Installed", "theme.success", tintSuccess)
	}
	return giode.Row(giode.Styles{Gap: 16, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1,
		Padding: giode.UniformInset(20), BorderRadius: 16, BorderWidth: 1, BorderColor: "theme.border",
		BackgroundGradient: giode.Linear(100, tintPrimaryHi, tintAccent)},
		giode.Box(giode.Styles{Background: "theme.surface", BorderRadius: 12, Padding: giode.UniformInset(10)},
			icon("gpu", 24, "theme.primary")),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			giode.Text("GPU acceleration pack", h2),
			giode.Text(fmt.Sprintf("%s. Lets neural engines run on your graphics card. %d MB download.", e.Summary, s.a.svc.Store.DownloadSizeMB(e)), muted)),
		action)
}
