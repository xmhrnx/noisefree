package core

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "image/jpeg" // custom backgrounds
	_ "image/png"

	"github.com/xmhrnx/noisefree/internal/camera"
	"github.com/xmhrnx/noisefree/internal/settings"
	"github.com/xmhrnx/noisefree/internal/vcam"
	"github.com/xmhrnx/noisefree/internal/vision"
)

// fakeCameraEnv names an image to use as a camera, for demos and tests
// without a webcam.
const fakeCameraEnv = "NOISEFREE_FAKE_CAMERA"

// VideoStatus describes the camera pipeline for the UI.
type VideoStatus struct {
	Running      bool
	FPS          float32
	FrameMs      float32 // processing time per frame
	InferenceMs  float32
	FaceFound    bool
	Backend      string // where the networks run: "cpu", "directml", ...
	Warning      string
	Err          error
	ModelsReady  bool // AI effects available (ONNX Runtime installed)
	Width        int
	Height       int
	VirtualCam   vcam.Status
	VirtualOn    bool  // frames are going to the virtual camera
	VirtualError error // why the virtual camera couldn't open
}

// Video runs camera -> effects -> virtual camera.
type Video struct {
	svc *Service

	mu        sync.Mutex
	proc      *vision.Processor
	models    *vision.Models
	modelsGPU bool
	stop      chan struct{}
	done      chan struct{}
	status    VideoStatus
	previews  [3]*image.RGBA
	latest    int
	seq       uint64
	custom    image.Image
	customFor string
	vcCache   vcam.Status
	vcAt      time.Time
	vcBusy    bool
}

func newVideo(s *Service) *Video { return &Video{svc: s, latest: -1} }

// Cameras lists the cameras.
func (v *Video) Cameras() []camera.Device {
	if path := os.Getenv(fakeCameraEnv); path != "" {
		return append([]camera.Device{{ID: "fake", Name: "Test camera (" + path + ")"}}, camera.List()...)
	}
	return camera.List()
}

// Status returns the pipeline's state.
func (v *Video) Status() VideoStatus {
	v.mu.Lock()
	defer v.mu.Unlock()
	st := v.status
	if v.proc != nil {
		ps := v.proc.Stats()
		st.FrameMs = float32(ps.Frame.Microseconds()) / 1000
		st.InferenceMs = float32(ps.Inference.Microseconds()) / 1000
		st.FaceFound = ps.FaceFound
	}
	return st
}

// Preview returns the latest processed frame (at most 640 wide) and a
// sequence number that changes with every new frame. The image stays valid
// for two more frames.
func (v *Video) Preview() (*image.RGBA, uint64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.latest < 0 {
		return nil, 0
	}
	return v.previews[v.latest], v.seq
}

// EnableVirtualCamera makes the virtual camera available where that can be
// done from the app (Linux), then restarts the pipeline to use it.
func (v *Video) EnableVirtualCamera() error {
	if err := vcam.Enable(); err != nil {
		return err
	}
	v.mu.Lock()
	v.vcAt = time.Time{} // re-check now
	v.mu.Unlock()
	if v.Running() {
		return v.Restart()
	}
	return nil
}

// DetectVirtualCamera reports the virtual camera's status while the camera
// is off. Checking can run a program (Linux), so the answer is cached and
// refreshed in the background every few seconds.
func (v *Video) DetectVirtualCamera() vcam.Status {
	v.mu.Lock()
	defer v.mu.Unlock()
	if time.Since(v.vcAt) > 5*time.Second && !v.vcBusy {
		v.vcBusy = true
		go func() {
			st := vcam.Detect()
			v.mu.Lock()
			v.vcCache, v.vcAt, v.vcBusy = st, time.Now(), false
			v.mu.Unlock()
		}()
	}
	return v.vcCache
}

// Running reports whether the camera is on.
func (v *Video) Running() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.stop != nil && v.status.Running
}

// loadModels loads (or reloads, when the GPU choice changed) the networks.
// Without ONNX Runtime the AI effects are unavailable, not an error.
func (v *Video) loadModels(useGPU bool) (string, error) {
	v.mu.Lock()
	have, gpu := v.models, v.modelsGPU
	v.mu.Unlock()
	if have != nil && gpu == useGPU {
		return "", nil
	}
	lib, acc, warn := v.svc.Store.RuntimeFor(useGPU)
	if lib == "" {
		return "", nil
	}
	m, err := vision.LoadModels(lib, acc)
	if err != nil {
		return "", err
	}
	if m.Warning != "" {
		warn = m.Warning
	}
	v.mu.Lock()
	old := v.models
	v.models, v.modelsGPU = m, useGPU
	v.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return warn, nil
}

// Restart (re)starts the camera with the current settings. It blocks while
// the camera and models open.
func (v *Video) Restart() error {
	v.Stop()
	set := v.svc.Settings()
	warn, merr := v.loadModels(set.UseGPU)
	h := set.Video.Height
	if h != 360 && h != 540 {
		h = 720
	}
	w := h * 16 / 9
	var src camera.Source
	var err error
	if set.Video.Camera == "fake" || (set.Video.Camera == "" && os.Getenv(fakeCameraEnv) != "") {
		src, err = camera.OpenFake(os.Getenv(fakeCameraEnv), w, h)
	} else {
		src, err = camera.Open(cameraID(v.Cameras(), set.Video.Camera), w, h)
	}
	if err != nil {
		v.setErr(err)
		return err
	}
	v.mu.Lock()
	v.proc = vision.NewProcessor(v.models)
	v.status = VideoStatus{Running: true, ModelsReady: v.models != nil, VirtualCam: vcam.Detect(), Warning: warn}
	if v.models != nil {
		v.status.Backend = string(v.models.Accelerator())
	}
	if merr != nil {
		v.status.Warning = "AI effects unavailable: " + merr.Error()
	}
	v.stop, v.done = make(chan struct{}), make(chan struct{})
	stop, done := v.stop, v.done
	v.mu.Unlock()
	v.Apply()
	go v.loop(src, set.Video.VirtualCamera, stop, done)
	return nil
}

func cameraID(devs []camera.Device, name string) string {
	for _, d := range devs {
		if d.Name == name {
			return d.ID
		}
	}
	return ""
}

func (v *Video) setErr(err error) {
	v.mu.Lock()
	v.status.Err, v.status.Running = err, false
	v.mu.Unlock()
}

// Stop turns the camera off.
func (v *Video) Stop() {
	v.mu.Lock()
	stop, done := v.stop, v.done
	v.stop = nil
	v.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
	v.mu.Lock()
	v.status.Running, v.status.VirtualOn = false, false
	v.latest = -1
	v.mu.Unlock()
}

// Apply pushes the current settings into the running effects.
func (v *Video) Apply() {
	set := v.svc.Settings().Video
	vs := vision.Settings{
		Background: vision.BackgroundMode(set.Background), BlurStrength: set.BlurStrength,
		RemoveColor: parseColor(set.RemoveColor),
		EyeContact:  set.EyeContact, EyeStrength: set.EyeStrength,
		Lighting: set.Lighting, LightStrength: set.LightStrength,
		Denoise: set.Denoise, Sharpen: set.Sharpen,
	}
	if vs.Background == vision.BackgroundReplace {
		vs.BackgroundImage = v.backgroundImage(set)
	}
	v.mu.Lock()
	if v.proc != nil {
		v.proc.SetSettings(vs)
	}
	v.mu.Unlock()
}

func (v *Video) backgroundImage(set settings.Video) image.Image {
	if set.ImagePath != "" {
		v.mu.Lock()
		cached, path := v.custom, v.customFor
		v.mu.Unlock()
		if path == set.ImagePath && cached != nil {
			return cached
		}
		if f, err := os.Open(set.ImagePath); err == nil {
			img, _, err := image.Decode(f)
			f.Close()
			if err == nil {
				v.mu.Lock()
				v.custom, v.customFor = img, set.ImagePath
				v.mu.Unlock()
				return img
			}
		}
	}
	for _, p := range vision.Presets {
		if p.Name == set.Preset {
			return p.Image()
		}
	}
	return vision.Presets[0].Image()
}

func parseColor(s string) color.RGBA {
	s = strings.TrimPrefix(s, "#")
	if n, err := strconv.ParseUint(s, 16, 32); err == nil && len(s) == 6 {
		return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
	}
	return color.RGBA{0, 177, 64, 255}
}

// loop reads, processes and publishes frames until stop closes.
func (v *Video) loop(src camera.Source, virtual bool, stop, done chan struct{}) {
	defer close(done)
	defer src.Close()
	var out vcam.Output
	defer func() {
		if out != nil {
			out.Close()
		}
	}()
	var frames int
	tick := time.Now()
	for {
		select {
		case <-stop:
			return
		default:
		}
		frame, err := src.Read()
		if err != nil {
			v.setErr(fmt.Errorf("camera stopped: %w", err))
			if v.svc.OnNotice != nil {
				v.svc.OnNotice("The camera stopped: " + err.Error())
			}
			return
		}
		v.mu.Lock()
		proc := v.proc
		v.mu.Unlock()
		result, err := proc.Process(frame)
		if err != nil {
			v.setErr(err)
			return
		}
		if virtual && out == nil {
			if o, err := vcam.Open(result.Rect.Dx(), result.Rect.Dy(), 30); err == nil {
				out = o
				v.mu.Lock()
				v.status.VirtualOn, v.status.VirtualError = true, nil
				v.mu.Unlock()
			} else {
				virtual = false // don't retry every frame; Restart tries again
				v.mu.Lock()
				v.status.VirtualError = err
				v.mu.Unlock()
			}
		}
		if out != nil {
			if err := out.Write(result); err != nil && !errors.Is(err, os.ErrClosed) {
				v.mu.Lock()
				v.status.VirtualError, v.status.VirtualOn = err, false
				v.mu.Unlock()
				out.Close()
				out = nil
			}
		}
		v.publish(result)
		frames++
		if d := time.Since(tick); d >= time.Second {
			v.mu.Lock()
			v.status.FPS = float32(float64(frames) / d.Seconds())
			v.status.Width, v.status.Height = result.Rect.Dx(), result.Rect.Dy()
			v.mu.Unlock()
			frames, tick = 0, time.Now()
		}
	}
}

// publish stores a downscaled copy for the preview in the next ring slot.
func (v *Video) publish(img *image.RGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	step := 1
	for w/step > 640 {
		step++
	}
	pw, ph := w/step, h/step
	v.mu.Lock()
	next := (v.latest + 1) % len(v.previews)
	v.mu.Unlock()
	p := v.previews[next]
	if p == nil || p.Rect.Dx() != pw || p.Rect.Dy() != ph {
		p = image.NewRGBA(image.Rect(0, 0, pw, ph))
	}
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			var r, g, b int
			for dy := 0; dy < step; dy++ {
				o := (y*step+dy)*img.Stride + x*step*4
				for dx := 0; dx < step; dx++ {
					r, g, b = r+int(img.Pix[o]), g+int(img.Pix[o+1]), b+int(img.Pix[o+2])
					o += 4
				}
			}
			n := step * step
			i := y*p.Stride + x*4
			p.Pix[i], p.Pix[i+1], p.Pix[i+2], p.Pix[i+3] = uint8(r/n), uint8(g/n), uint8(b/n), 255
		}
	}
	v.mu.Lock()
	v.previews[next], v.latest = p, next
	v.seq++
	v.mu.Unlock()
}

// Close stops the camera and frees the models.
func (v *Video) Close() {
	v.Stop()
	v.mu.Lock()
	m := v.models
	v.models = nil
	v.mu.Unlock()
	if m != nil {
		m.Close()
	}
}
