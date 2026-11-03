package catalog

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Progress reports install progress from 0 to 1 with a short status line.
type Progress func(fraction float64, status string)

// Install downloads an entry and everything it requires.
func (s *Store) Install(ctx context.Context, id string, progress Progress) error {
	e, ok := s.Get(id)
	if !ok {
		return fmt.Errorf("unknown engine %q", id)
	}
	plan := []Entry{}
	for _, r := range e.Requires {
		re, ok := s.Get(r)
		if !ok {
			return fmt.Errorf("%s needs %q, which isn't in the catalog", e.Name, r)
		}
		if !s.Installed(re) {
			plan = append(plan, re)
		}
	}
	plan = append(plan, e)
	for i, p := range plan {
		step := func(f float64, status string) {
			progress((float64(i)+f)/float64(len(plan)), status)
		}
		if err := s.installOne(ctx, p, step); err != nil {
			return err
		}
	}
	progress(1, "Installed")
	return nil
}

func (s *Store) installOne(ctx context.Context, e Entry, progress Progress) error {
	if e.Builtin {
		return nil
	}
	downloads, ok := e.DownloadsHere()
	if !ok {
		return fmt.Errorf("%s isn't available for %s", e.Name, Platform())
	}
	final := s.EngineDir(e.ID)
	tmp := final + ".partial"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var files []string
	for i, d := range downloads {
		step := func(f float64) {
			progress((float64(i)+f)/float64(len(downloads)), "Downloading "+e.Name)
		}
		got, err := fetch(ctx, d, tmp, step)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
		files = append(files, got...)
	}
	if err := writeReceipt(tmp, receipt{Version: e.Version, Files: files}); err != nil {
		return err
	}
	_ = os.RemoveAll(final)
	return os.Rename(tmp, final)
}

// fetch downloads d into dir and returns the names of the files it saved.
func fetch(ctx context.Context, d Download, dir string, progress func(float64)) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NoiseFree-Broadcast")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", path.Base(d.URL), resp.Status)
	}
	tmp, err := os.CreateTemp(dir, "download-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha256.New()
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	if _, err := io.Copy(io.MultiWriter(tmp, h), pr); err != nil {
		return nil, err
	}
	if d.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), d.SHA256) {
		return nil, errors.New("checksum mismatch: the download is corrupt or was tampered with")
	}
	switch d.Archive {
	case "":
		if len(d.Pick) != 1 || d.Pick[0].As == "" {
			return nil, errors.New("a plain download needs exactly one pick with a name")
		}
		if err := tmp.Close(); err != nil {
			return nil, err
		}
		dst := filepath.Join(dir, d.Pick[0].As)
		if err := os.Rename(tmp.Name(), dst); err != nil {
			return nil, err
		}
		return []string{d.Pick[0].As}, os.Chmod(dst, 0o755)
	case "zip":
		return extractZip(tmp, dir, d.Pick)
	case "tgz":
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return extractTgz(tmp, dir, d.Pick)
	}
	return nil, fmt.Errorf("unknown archive type %q", d.Archive)
}

// matchPick returns the pick an archive member satisfies: a path suffix
// match first, then (as a fallback for repackaged archives) the base name.
func matchPick(name string, picks []Pick, exact bool) (Pick, bool) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	for _, p := range picks {
		if exact && (name == p.Match || strings.HasSuffix(name, "/"+p.Match)) {
			return p, true
		}
		if !exact && path.Base(name) == path.Base(p.Match) {
			return p, true
		}
	}
	return Pick{}, false
}

func extractZip(f *os.File, dir string, picks []Pick) ([]string, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return nil, err
	}
	saved := map[string]bool{}
	for _, exact := range []bool{true, false} {
		for _, zf := range zr.File {
			p, ok := matchPick(zf.Name, picks, exact)
			if !ok || saved[p.As] || zf.FileInfo().IsDir() {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			err = writeFile(filepath.Join(dir, p.As), rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			saved[p.As] = true
		}
	}
	return checkPicks(picks, saved)
}

func extractTgz(r io.Reader, dir string, picks []Pick) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	saved := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if p, ok := matchPick(h.Name, picks, true); ok && !saved[p.As] {
			if err := writeFile(filepath.Join(dir, p.As), tr); err != nil {
				return nil, err
			}
			saved[p.As] = true
		}
	}
	return checkPicks(picks, saved)
}

func checkPicks(picks []Pick, saved map[string]bool) ([]string, error) {
	var names []string
	for _, p := range picks {
		if !saved[p.As] {
			return nil, fmt.Errorf("%s not found in the archive", p.Match)
		}
		names = append(names, p.As)
	}
	return names, nil
}

func writeFile(dst string, r io.Reader) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type progressReader struct {
	r     io.Reader
	n     int64
	total int64
	fn    func(float64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	if p.total > 0 {
		p.fn(float64(p.n) / float64(p.total))
	}
	return n, err
}
