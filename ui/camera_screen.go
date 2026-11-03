package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/dropdown"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/noisefree/internal/catalog"
	"github.com/xmhrnx/noisefree/internal/core"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// cameraScreen is the camera effects page: a live preview and the effect
// controls.
type cameraScreen struct {
	a          *App
	power      *button.Button
	camera     *dropdown.Select
	cameras    []string
	resolution *segmented.Control
	installAI  *button.Button
	background *backgroundCard
	appearance *appearanceCard
	quality    *qualityCard
	virtual    *virtualCameraCard
	starting   bool
}

var resolutions = []int{720, 540, 360}

type videoStatusView = core.VideoStatus

func mustEntry(a *App, id string) catalog.Entry {
	e, _ := a.svc.Store.Get(id)
	return e
}

func newCameraScreen(a *App) *cameraScreen {
	c := &cameraScreen{a: a}
	set := a.svc.Settings().Video
	c.power = giode.Button("Turn camera on", primaryButton()).Icon(icon("video", 15, "")).OnClick(c.toggle)
	c.camera = giode.Input.Select(fieldStyle).OnChange(func(i int) {
		if i < len(c.cameras) {
			name := c.cameras[i]
			a.svc.Update(func(s *settings.Settings) { s.Video.Camera = name })
			c.restart()
		}
	})
	c.resolution = giode.Input.Segmented("720p", "540p", "360p · light").OnChange(func(i int) {
		a.svc.Update(func(s *settings.Settings) { s.Video.Height = resolutions[i] })
		c.restart()
	})
	for i, r := range resolutions {
		if r == set.Height {
			c.resolution.SetSelected(i)
		}
	}
	c.installAI = giode.Button("Install AI effects", primaryButton()).Icon(icon("download", 15, "")).OnClick(func() {
		a.engines.install("onnxruntime")
	})
	c.background = newBackgroundCard(a)
	c.appearance = newAppearanceCard(a)
	c.quality = newQualityCard(a)
	c.virtual = newVirtualCameraCard(a)
	return c
}

// refreshCameras rescans the cameras in the background.
func (c *cameraScreen) refreshCameras() {
	go func() {
		devs := c.a.svc.Video.Cameras()
		c.a.win.Post(func() {
			c.cameras = c.cameras[:0]
			for _, d := range devs {
				c.cameras = append(c.cameras, d.Name)
			}
			if len(c.cameras) == 0 {
				c.camera.SetOptions("No camera found")
				return
			}
			c.camera.SetOptions(c.cameras...)
			cur := c.a.svc.Settings().Video.Camera
			for i, n := range c.cameras {
				if n == cur {
					c.camera.SetSelected(i)
				}
			}
		})
	}()
}

func (c *cameraScreen) toggle() {
	on := !c.a.svc.Video.Running()
	c.a.svc.Update(func(s *settings.Settings) { s.Video.Enabled = on })
	if !on {
		go c.a.svc.Video.Stop()
		return
	}
	c.restart()
}

// restart reopens the camera with the current settings, if it's on.
func (c *cameraScreen) restart() {
	if !c.a.svc.Settings().Video.Enabled {
		return
	}
	c.starting = true
	go func() {
		err := c.a.svc.Video.Restart()
		c.a.win.Post(func() {
			c.starting = false
			if err != nil {
				c.a.toast(giode.ToastError, "Couldn't start the camera: "+err.Error())
			}
		})
	}()
}

// View lays out the page.
func (c *cameraScreen) View() giode.Element {
	st := c.a.svc.Video.Status()
	label, iconName := "Turn camera on", "video"
	if st.Running {
		label, iconName = "Turn camera off", "video-off"
	}
	c.power.Label(label).Icon(icon(iconName, 15, "")).Loading(c.starting)
	kids := []giode.Element{
		giode.Row(giode.Styles{Gap: 16, Align: giode.AlignEnd, Wrap: giode.WrapNoWrap, Width: -1},
			pageHeader("Camera", "Studio-quality video on any GPU, any CPU."), spacer(), c.power),
		c.previewCard(st),
	}
	if !st.ModelsReady && !c.a.svc.Store.Installed(mustEntry(c.a, "onnxruntime")) {
		kids = append(kids, c.aiBanner())
	}
	kids = append(kids,
		pair(0, func(s giode.Styles) giode.Element { return c.background.View(s, st) },
			func(s giode.Styles) giode.Element { return c.appearance.View(s, st) }),
		pair(0, func(s giode.Styles) giode.Element { return c.quality.View(s) },
			func(s giode.Styles) giode.Element { return c.virtual.View(s, st) }),
	)
	return giode.Stack(giode.Styles{Gap: 20, Width: -1}, kids...)
}

func (c *cameraScreen) previewCard(st videoStatusView) giode.Element {
	frame, _ := c.a.svc.Video.Preview()
	var chips []giode.Element
	switch {
	case st.Running:
		chips = append(chips,
			iconPill("activity", fmt.Sprintf("%.0f fps", st.FPS), "theme.success", tintSuccess),
			iconPill("timer", fmt.Sprintf("%.0f ms per frame", st.FrameMs), "theme.accent", tintAccent))
		if st.ModelsReady {
			chips = append(chips, iconPill("gpu", "AI on "+backendName(st.Backend), "theme.primary", tintPrimary))
			if st.FaceFound {
				chips = append(chips, iconPill("scan-face", "Face tracked", "theme.success", tintSuccess))
			}
		}
	case st.Err != nil:
		chips = append(chips, iconPill("circle-alert", st.Err.Error(), "theme.danger", tintDanger))
	default:
		chips = append(chips, giode.Text("The camera is off. Turn it on to see the effects live.", muted))
	}
	if st.Warning != "" {
		chips = append(chips, iconPill("circle-alert", st.Warning, "theme.warning", tintWarning))
	}
	return giode.Box(cardStyle.With(giode.Styles{Width: -1}),
		giode.Box(giode.Styles{Width: -1, MaxHeight: 480, Align: giode.AlignCenter}, videoView(frame, 12)),
		giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Width: -1}, chips...),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		giode.Row(giode.Styles{Gap: 16, Align: giode.AlignEnd, Wrap: giode.WrapNoWrap, Width: -1},
			giode.Box(giode.Styles{FlexGrow: 1}, labelled("Camera", c.camera)),
			labelled("Resolution", c.resolution)),
	)
}

func (c *cameraScreen) aiBanner() giode.Element {
	e := mustEntry(c.a, "onnxruntime")
	var action giode.Element = c.installAI
	if st, busy := c.a.engines.installing[e.ID]; busy {
		action = giode.Stack(giode.Styles{Gap: 6, Width: 200},
			giode.Text(st.status, caption),
			giode.Progress(float32(st.progress), giode.Styles{Color: "theme.primary", Background: track, Height: 6}))
	}
	return giode.Row(giode.Styles{Gap: 16, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1,
		Padding: giode.UniformInset(20), BorderRadius: 16, BorderWidth: 1, BorderColor: "theme.border",
		BackgroundGradient: giode.Linear(100, tintPrimaryHi, tintAccent)},
		giode.Box(giode.Styles{Background: "theme.surface", BorderRadius: 12, Padding: giode.UniformInset(10)},
			icon("wand-sparkles", 24, "theme.primary")),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			giode.Text("Turn on the AI effects", h2),
			giode.Text(fmt.Sprintf("Background blur and replacement and eye contact need ONNX Runtime (%d MB download). Lighting and quality work without it.",
				c.a.svc.Store.DownloadSizeMB(e)), muted)),
		action)
}

func backendName(b string) string {
	switch b {
	case "directml":
		return "GPU · DirectML"
	case "coreml":
		return "GPU · Core ML"
	case "cuda":
		return "GPU · CUDA"
	}
	return "CPU"
}
