package ui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/dialog"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/noisefree/internal/catalog"
)

// addEngineDialog adds engines that aren't in the built-in catalog: a LADSPA
// plugin file, or a JSON manifest from a file or a URL.
type addEngineDialog struct {
	a                      *App
	open                   bool
	dlg                    *dialog.Dialog
	pluginBtn, manifestBtn *button.Button
	url                    *input.Input
	fetchBtn, closeBtn     *button.Button
}

func newAddEngineDialog(a *App) *addEngineDialog {
	d := &addEngineDialog{a: a}
	d.pluginBtn = giode.Button("Choose plugin file…", secondaryButton()).Icon(icon("file-plus", 15, "")).OnClick(d.pickPlugin)
	d.manifestBtn = giode.Button("Choose manifest…", secondaryButton()).Icon(icon("file-plus", 15, "")).OnClick(d.pickManifest)
	d.url = giode.Input.Text("https://example.com/engine.json", fieldStyle).Keyboard(giode.KeyboardURL)
	d.fetchBtn = giode.Button("Add", primaryButton()).OnClick(d.fetchManifest)
	d.closeBtn = giode.Button("Close", secondaryButton()).OnClick(func() { d.open = false })
	d.dlg = giode.Dialog(&d.open, d.content, giode.Styles{Background: "theme.surface", BorderRadius: 18, Width: 520,
		BoxShadow: giode.BoxShadow{Y: 20, Blur: 60, Color: "#00000066"}})
	return d
}

func (d *addEngineDialog) content() giode.Element {
	option := func(iconName, title, desc string, action giode.Element) giode.Element {
		return giode.Row(giode.Styles{Gap: 14, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1,
			Padding: giode.UniformInset(14), Background: raised, BorderRadius: 12, BorderWidth: 1, BorderColor: "theme.border"},
			giode.Box(giode.Styles{Background: tintPrimary, BorderRadius: 10, Padding: giode.UniformInset(8)}, icon(iconName, 18, "theme.primary")),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(title, giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}),
				giode.Text(desc, muted)),
			action)
	}
	return giode.Stack(giode.Styles{Gap: 16, Width: -1},
		giode.Text("Add a noise suppression engine", giode.Styles{FontSize: 18, FontWeight: giode.FontWeightBold, Color: "theme.text"}),
		giode.Text("NoiseFree runs any LADSPA voice plugin at 48 kHz, and installs engines described by a JSON manifest.", muted),
		option("blocks", "LADSPA plugin", "A .so, .dll or .dylib with a mono effect.", d.pluginBtn),
		option("file-plus", "Engine manifest", "A .json file describing downloads and settings.", d.manifestBtn),
		giode.Stack(giode.Styles{Gap: 8, Width: -1},
			giode.Text("Manifest URL", overline),
			giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
				giode.Box(giode.Styles{FlexGrow: 1}, d.url), d.fetchBtn)),
		giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap}, spacer(), d.closeBtn),
	)
}

// pickFile shows the system picker in the background and hands the file's
// name and bytes to done on the UI goroutine.
func (d *addEngineDialog) pickFile(exts []string, done func(name string, data []byte)) {
	go func() {
		files, err := d.a.win.PickFiles(context.Background(), giode.PickOptions{Types: exts})
		if err != nil || len(files) == 0 {
			if err != nil && !errors.Is(err, giode.ErrCanceled) {
				d.a.win.Post(func() { d.a.toast(giode.ToastError, "Couldn't open the file: "+err.Error()) })
			}
			return
		}
		f := files[0]
		data, err := io.ReadAll(f)
		f.Close()
		d.a.win.Post(func() {
			if err != nil {
				d.a.toast(giode.ToastError, "Couldn't read the file: "+err.Error())
				return
			}
			done(f.Name, data)
		})
	}()
}

func (d *addEngineDialog) pickPlugin() {
	d.pickFile([]string{"so", "dll", "dylib"}, func(name string, data []byte) {
		e, err := d.a.svc.Store.AddPluginFile(name, data)
		d.finish(e, err)
	})
}

func (d *addEngineDialog) pickManifest() {
	d.pickFile([]string{"json"}, func(_ string, data []byte) {
		e, err := d.a.svc.Store.AddManifest(data)
		d.finish(e, err)
	})
}

func (d *addEngineDialog) fetchManifest() {
	url := d.url.Text()
	d.fetchBtn.Loading(true)
	go func() {
		data, err := download(url)
		d.a.win.Post(func() {
			d.fetchBtn.Loading(false)
			if err != nil {
				d.a.toast(giode.ToastError, "Couldn't fetch the manifest: "+err.Error())
				return
			}
			e, err := d.a.svc.Store.AddManifest(data)
			d.finish(e, err)
		})
	}()
}

func (d *addEngineDialog) finish(e catalog.Entry, err error) {
	if err != nil {
		d.a.toast(giode.ToastError, err.Error())
		return
	}
	d.open = false
	d.url.SetText("")
	d.a.home.suppression.refresh()
	if d.a.svc.Store.Installed(e) {
		d.a.toastAction(giode.ToastSuccess, e.Name+" added.", "Use now", func() { d.a.useEngine(e.ID) })
		return
	}
	d.a.toast(giode.ToastSuccess, e.Name+" added to the library. Install it from its card.")
}

func download(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{resp.Status}
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

type httpError struct{ status string }

func (e *httpError) Error() string { return e.status }

// View places the dialog (it takes no space).
func (d *addEngineDialog) View() giode.Element { return d.dlg }
