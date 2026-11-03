package ui

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/slider"
	"github.com/xmhrnx/noisefree/internal/settings"
	"github.com/xmhrnx/noisefree/internal/vision"
	xdraw "golang.org/x/image/draw"
)

var removeColors = []string{"#00B140", "#0047BB", "#2B2C30", "#000000", "#F2F2F2"}

// backgroundCard picks what shows behind you.
type backgroundCard struct {
	a      *App
	mode   *segmented.Control
	blur   *slider.Slider
	choose *button.Button
	thumbs []image.Image
}

func newBackgroundCard(a *App) *backgroundCard {
	c := &backgroundCard{a: a}
	set := a.svc.Settings().Video
	c.mode = giode.Input.Segmented("Off", "Blur", "Replace", "Remove").OnChange(func(i int) {
		c.update(func(v *settings.Video) { v.Background = i })
	})
	c.mode.Styles(giode.Styles{Width: -1})
	c.mode.SetSelected(set.Background)
	c.blur = giode.Input.Slider(giode.Styles{Color: "theme.primary", Background: track, BorderColor: "theme.primary"}).
		OnChange(func(v float32) { c.update(func(s *settings.Video) { s.BlurStrength = v }) })
	c.blur.SetValue(set.BlurStrength)
	c.choose = giode.Button("Your own image…", secondaryButton()).Icon(icon("image-plus", 15, "")).OnClick(c.pickImage)
	for _, p := range vision.Presets {
		img := p.Image()
		thumb := image.NewRGBA(image.Rect(0, 0, 128, 72))
		xdraw.ApproxBiLinear.Scale(thumb, thumb.Rect, img, img.Bounds(), xdraw.Src, nil)
		c.thumbs = append(c.thumbs, thumb)
	}
	return c
}

func (c *backgroundCard) update(fn func(*settings.Video)) {
	c.a.svc.Update(func(s *settings.Settings) { fn(&s.Video) })
	c.a.svc.Video.Apply()
}

// pickImage lets the user choose a picture and keeps a copy with the app's
// data, so the background survives the original being moved.
func (c *backgroundCard) pickImage() {
	go func() {
		files, err := c.a.win.PickFiles(context.Background(), giode.PickOptions{Types: []string{"image/*"}})
		if err != nil || len(files) == 0 {
			if err != nil && !errors.Is(err, giode.ErrCanceled) {
				c.a.win.Post(func() { c.a.toast(giode.ToastError, "Couldn't open the image: "+err.Error()) })
			}
			return
		}
		f := files[0]
		defer f.Close()
		dst := filepath.Join(c.a.svc.Dir, "background"+filepath.Ext(f.Name))
		out, err := os.Create(dst)
		if err == nil {
			_, err = out.ReadFrom(f)
			out.Close()
		}
		c.a.win.Post(func() {
			if err != nil {
				c.a.toast(giode.ToastError, "Couldn't save the image: "+err.Error())
				return
			}
			c.update(func(v *settings.Video) { v.ImagePath, v.Background = dst, int(vision.BackgroundReplace) })
			c.mode.SetSelected(int(vision.BackgroundReplace))
		})
	}()
}

// View lays out the card.
func (c *backgroundCard) View(st giode.Styles, vs videoStatusView) giode.Element {
	set := c.a.svc.Settings().Video
	kids := []giode.Element{c.mode}
	switch vision.BackgroundMode(set.Background) {
	case vision.BackgroundBlur:
		kids = append(kids, sliderRow("Blur", percent(set.BlurStrength), c.blur))
	case vision.BackgroundReplace:
		var tiles []giode.Element
		for i, p := range vision.Presets {
			selected := set.ImagePath == "" && set.Preset == p.Name
			tiles = append(tiles, giode.Keyed(c, p.Name, func() *presetTile {
				name := p.Name
				return newPresetTile(name, c.thumbs[i], func() {
					c.update(func(v *settings.Video) { v.Preset, v.ImagePath = name, "" })
				})
			}).View(selected))
		}
		kids = append(kids, giode.Row(giode.Styles{Gap: 10, Width: -1}, tiles...), c.choose)
	case vision.BackgroundRemove:
		var sw []giode.Element
		for _, col := range removeColors {
			sw = append(sw, giode.Keyed(c, col, func() *swatch {
				hex := col
				return newSwatch(hex, func() { c.update(func(v *settings.Video) { v.RemoveColor = hex }) })
			}).View(set.RemoveColor == col))
		}
		kids = append(kids, giode.Row(giode.Styles{Gap: 10}, sw...),
			giode.Text("A solid colour; green works as a chroma key in OBS.", muted))
	}
	if !vs.ModelsReady && set.Background != int(vision.BackgroundOff) {
		kids = append(kids, giode.Text("Needs the AI effects: install them above.", giode.Styles{FontSize: 12.5, Color: "theme.warning"}))
	}
	return sectionStyled(st, "aperture", "Background", "Blur, replace or remove what's behind you.", nil, kids...)
}
