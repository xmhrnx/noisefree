package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

const testManifest = `<?xml version="1.0" encoding="utf-8"?>
<Package>
  <Capabilities>
    <rescap:Capability Name="runFullTrust"/>
  </Capabilities>
</Package>
`

// testPackage builds a package the way giode does: stored entries, a block
// map of every payload file, a content types list and a signature.
func testPackage(t *testing.T) []byte {
	t.Helper()
	exe := bytes.Repeat([]byte("MZ"), blockSize) // two blocks
	files := []struct {
		name string
		body []byte
	}{{"NoiseFree.exe", exe}, {manifestName, []byte(testManifest)}}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var bm strings.Builder
	bm.WriteString(`<BlockMap xmlns="http://schemas.microsoft.com/appx/2010/blockmap">`)
	for _, f := range files {
		if err := store(zw, f.name, f.body); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&bm, `<File Name="%s" Size="%d" LfhSize="%d">`, f.name, len(f.body), 30+len(f.name))
		for off := 0; off < len(f.body); off += blockSize {
			sum := sha256.Sum256(f.body[off:min(off+blockSize, len(f.body))])
			fmt.Fprintf(&bm, `<Block Hash="%s"/>`, base64.StdEncoding.EncodeToString(sum[:]))
		}
		bm.WriteString(`</File>`)
	}
	bm.WriteString(`</BlockMap>`)
	for name, body := range map[string]string{blockMapName: bm.String(), "[Content_Types].xml": "<Types/>", signatureName: "sig"} {
		if err := store(zw, name, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPatch(t *testing.T) {
	out, changed, err := patch(testPackage(t))
	if err != nil || !changed {
		t.Fatalf("patch: changed %v, %v", changed, err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{}
	for _, f := range zr.File {
		if f.Method != zip.Store {
			t.Errorf("%s is compressed", f.Name)
		}
		r, _ := f.Open()
		bodies[f.Name], _ = io.ReadAll(r)
		r.Close()
	}
	if _, ok := bodies[signatureName]; ok {
		t.Error("the stale signature was kept")
	}
	m := string(bodies[manifestName])
	if !strings.Contains(m, `<rescap:Capability Name="runFullTrust"/>
    <DeviceCapability Name="microphone"/>
  </Capabilities>`) {
		t.Errorf("manifest:\n%s", m)
	}
	// Every file's block map entry matches its contents.
	re := regexp.MustCompile(`<File Name="([^"]+)" Size="(\d+)" LfhSize="(\d+)">((?:<Block [^>]*/>)*)</File>`)
	matches := re.FindAllStringSubmatch(string(bodies[blockMapName]), -1)
	if len(matches) != 2 {
		t.Fatalf("block map has %d files, want 2", len(matches))
	}
	for _, mm := range matches {
		body := bodies[mm[1]]
		if mm[2] != fmt.Sprint(len(body)) || mm[3] != fmt.Sprint(30+len(mm[1])) {
			t.Errorf("%s: size %s, lfh %s; body is %d bytes", mm[1], mm[2], mm[3], len(body))
		}
		var want strings.Builder
		for off := 0; off < len(body); off += blockSize {
			sum := sha256.Sum256(body[off:min(off+blockSize, len(body))])
			fmt.Fprintf(&want, `<Block Hash="%s"/>`, base64.StdEncoding.EncodeToString(sum[:]))
		}
		if mm[4] != want.String() {
			t.Errorf("%s: block hashes don't match its contents", mm[1])
		}
	}

	again, changed, err := patch(out)
	if err != nil || changed || !bytes.Equal(again, out) {
		t.Errorf("patching twice changed the package (changed %v, %v)", changed, err)
	}
}
