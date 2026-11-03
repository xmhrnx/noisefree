package catalog

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func zipOf(files map[string]string) []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for n, c := range files {
		f, _ := w.Create(n)
		f.Write([]byte(c))
	}
	w.Close()
	return b.Bytes()
}

func tgzOf(files map[string]string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for n, c := range files {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(c)), Typeflag: tar.TypeReg})
		tw.Write([]byte(c))
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}

func TestInstallFromManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.zip":
			w.Write(zipOf(map[string]string{"pkg/ladspa/libx.so": "plugin", "pkg/readme": "x"}))
		case "/b.tgz":
			w.Write(tgzOf(map[string]string{"rt/lib/librt.so": "runtime"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rt := `{"id":"rt","name":"Runtime","kind":"runtime","hidden":true,"downloads":{"*":[{"url":"` + srv.URL + `/b.tgz","archive":"tgz","pick":[{"match":"lib/librt.so","as":"librt.so"}]}]}}`
	if _, err := s.AddManifest([]byte(rt)); err != nil {
		t.Fatal(err)
	}
	m := `{"id":"x","name":"X","kind":"ladspa","requires":["rt"],"plugin":{"file":"x"},"downloads":{"*":[{"url":"` + srv.URL + `/a.zip","archive":"zip","pick":[{"match":"ladspa/libx.so","as":"x"}]}]}}`
	e, err := s.AddManifest([]byte(m))
	if err != nil {
		t.Fatal(err)
	}
	if s.Installed(e) {
		t.Fatal("installed before install")
	}
	var last float64
	if err := s.Install(context.Background(), "x", func(f float64, _ string) { last = f }); err != nil {
		t.Fatal(err)
	}
	if last != 1 || !s.Installed(e) {
		t.Fatalf("not installed (progress %v)", last)
	}
	if b, _ := os.ReadFile(filepath.Join(s.EngineDir("rt"), "librt.so")); string(b) != "runtime" {
		t.Fatalf("runtime file = %q", b)
	}
	if err := s.Remove("x"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("x"); ok {
		t.Fatal("manifest still listed after removal")
	}
}

func TestMissingPickFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipOf(map[string]string{"other.so": "x"}))
	}))
	defer srv.Close()
	s, _ := NewStore(t.TempDir())
	m := `{"id":"y","name":"Y","kind":"ladspa","plugin":{"file":"y"},"downloads":{"*":[{"url":"` + srv.URL + `/y.zip","archive":"zip","pick":[{"match":"lib/y.so","as":"y"}]}]}}`
	if _, err := s.AddManifest([]byte(m)); err != nil {
		t.Fatal(err)
	}
	if err := s.Install(context.Background(), "y", func(float64, string) {}); err == nil {
		t.Fatal("expected a missing-file error")
	}
	if _, err := os.Stat(s.EngineDir("y")); !os.IsNotExist(err) {
		t.Fatal("partial install left behind")
	}
}

func TestManifestCantReplaceBuiltin(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if _, err := s.AddManifest([]byte(`{"id":"rnnoise","name":"Evil","kind":"ladspa","plugin":{"file":"x"}}`)); err == nil {
		t.Fatal("built-in engine was replaced")
	}
}

func TestBuiltinEnginesOpen(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	for _, id := range []string{"lite", "rnnoise"} {
		p, _, err := s.Open(id, false)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		buf := make([]float32, p.FrameSize())
		p.Process(buf)
		p.Close()
	}
}

// NOISEFREE_TEST_NETWORK=1 installs DTLN and ONNX Runtime from the internet.
func TestInstallDTLNFromInternet(t *testing.T) {
	if os.Getenv("NOISEFREE_TEST_NETWORK") == "" {
		t.Skip("set NOISEFREE_TEST_NETWORK=1")
	}
	s, _ := NewStore(t.TempDir())
	if err := s.Install(context.Background(), "dtln", func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	p, warn, err := s.Open("dtln", true)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	t.Logf("backend %s, warning %q", p.Backend(), warn)
	p.Process(make([]float32, p.FrameSize()))
}
