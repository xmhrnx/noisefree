// Command msixmic adds the microphone device capability to MSIX packages
// written by giode build msix.
//
// A packaged app only gets the microphone, and only shows up under
// Settings > Privacy & security > Microphone, when its manifest declares
// <DeviceCapability Name="microphone"/>. The Giode CLI (v0.1.35-alpha.1)
// declares runFullTrust alone and has no setting for more, so the release
// workflow runs this after building:
//
//	go run ./internal/cmd/msixmic dist/windows/*.msix
//
// It rewrites AppxManifest.xml and its entry in AppxBlockMap.xml, keeping
// every other entry as it is. A signature no longer matches afterwards, so
// it's dropped; Store packages are signed by the Store.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	manifestName  = "AppxManifest.xml"
	blockMapName  = "AppxBlockMap.xml"
	signatureName = "AppxSignature.p7x"
	capability    = `<DeviceCapability Name="microphone"/>`
	blockSize     = 65536
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: msixmic package.msix...")
		os.Exit(2)
	}
	for _, p := range os.Args[1:] {
		changed, err := patchFile(p)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "msixmic: %s: %v\n", p, err)
			os.Exit(1)
		case changed:
			fmt.Printf("%s: declared the microphone\n", p)
		default:
			fmt.Printf("%s: already declares the microphone\n", p)
		}
	}
}

// patchFile adds the capability to the package at path in place.
func patchFile(path string) (bool, error) {
	in, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	out, changed, err := patch(in)
	if err != nil || !changed {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

// patch returns the package with the microphone declared, and whether
// it had to change.
func patch(pkg []byte) ([]byte, bool, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil, false, err
	}
	bodies := map[string][]byte{}
	for _, f := range zr.File {
		b, err := readEntry(f)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", f.Name, err)
		}
		bodies[f.Name] = b
	}
	manifest, ok := bodies[manifestName]
	if !ok {
		return nil, false, errors.New("no " + manifestName)
	}
	if bytes.Contains(manifest, []byte(capability)) {
		return pkg, false, nil
	}
	newManifest, err := addCapability(manifest)
	if err != nil {
		return nil, false, err
	}
	blockMap, ok := bodies[blockMapName]
	if !ok {
		return nil, false, errors.New("no " + blockMapName)
	}
	newBlockMap, err := replaceBlocks(blockMap, manifestName, newManifest)
	if err != nil {
		return nil, false, err
	}
	bodies[manifestName], bodies[blockMapName] = newManifest, newBlockMap

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == signatureName {
			fmt.Fprintln(os.Stderr, "msixmic: removed the package signature, which no longer matches; sign the package again if it isn't for the Store")
			continue
		}
		if err := store(zw, f.Name, bodies[f.Name]); err != nil {
			return nil, false, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

func readEntry(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// addCapability declares the microphone after the package's other
// capabilities: the schema wants device capabilities last.
func addCapability(manifest []byte) ([]byte, error) {
	s := string(manifest)
	end := strings.Index(s, "</Capabilities>")
	if end < 0 {
		return nil, errors.New("the manifest has no <Capabilities> element")
	}
	return []byte(s[:end] + "  " + capability + "\n  " + s[end:]), nil
}

// replaceBlocks rewrites the block map's entry for name to describe body:
// its size and the SHA-256 of each 64 KiB block. The local file header
// size is unchanged, since the name is.
func replaceBlocks(blockMap []byte, name string, body []byte) ([]byte, error) {
	re := regexp.MustCompile(`<File Name="` + regexp.QuoteMeta(name) + `" Size="\d+"( LfhSize="\d+")>(?:<Block [^>]*/>)*</File>`)
	loc := re.FindSubmatchIndex(blockMap)
	if loc == nil {
		return nil, fmt.Errorf("%s has no uncompressed entry for %s", blockMapName, name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<File Name="%s" Size="%d"%s>`, name, len(body), blockMap[loc[2]:loc[3]])
	for off := 0; off < len(body); off += blockSize {
		sum := sha256.Sum256(body[off:min(off+blockSize, len(body))])
		fmt.Fprintf(&b, `<Block Hash="%s"/>`, base64.StdEncoding.EncodeToString(sum[:]))
	}
	b.WriteString(`</File>`)
	out := append([]byte(nil), blockMap[:loc[0]]...)
	out = append(out, b.String()...)
	return append(out, blockMap[loc[1]:]...), nil
}

// store adds an uncompressed entry with its sizes in the local header (no
// data descriptor), as giode writes them and packaging tools expect.
func store(zw *zip.Writer, name string, body []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc32.ChecksumIEEE(body),
		CompressedSize64: uint64(len(body)), UncompressedSize64: uint64(len(body))}
	w, err := zw.CreateRaw(h)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}
