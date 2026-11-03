package vcam

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	cardLabel = "NoiseFree Camera"

	vidiocQueryCap = 0x80685600
	vidiocSFmt     = 0xc0d05605
	bufTypeOutput  = 2
	fieldNone      = 1
	colorspaceSRGB = 8
)

var fourccYUYV = uint32('Y') | uint32('U')<<8 | uint32('Y')<<16 | uint32('V')<<24

type v4l2Capability struct {
	Driver       [16]byte
	Card         [32]byte
	BusInfo      [32]byte
	Version      uint32
	Capabilities uint32
	DeviceCaps   uint32
	Reserved     [3]uint32
}

type v4l2PixFormat struct {
	Width, Height, PixelFormat, Field, BytesPerLine, SizeImage, Colorspace, Priv, Flags, YcbcrEnc, Quantization, XferFunc uint32
}

type v4l2Format struct {
	Type uint32
	_    uint32 // the union is 8-byte aligned
	Pix  v4l2PixFormat
	_    [200 - 48]byte
}

// loopbacks returns the v4l2loopback devices, NoiseFree's first.
func loopbacks() []string {
	paths, _ := filepath.Glob("/dev/video*")
	var ours, others []string
	for _, p := range paths {
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		var c v4l2Capability
		_, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), vidiocQueryCap, uintptr(unsafe.Pointer(&c)))
		f.Close()
		if e != 0 || !bytes.HasPrefix(c.Driver[:], []byte("v4l2 loopback")) {
			continue
		}
		if bytes.Contains(c.Card[:], []byte("NoiseFree")) {
			ours = append(ours, p)
		} else {
			others = append(others, p)
		}
	}
	return append(ours, others...)
}

func detect() Status {
	if devs := loopbacks(); len(devs) > 0 {
		return Status{Ready: true, Name: cardLabel}
	}
	if _, err := os.Stat("/sys/module/v4l2loopback"); err == nil {
		return Status{Help: "The v4l2loopback module is loaded but has no device NoiseFree can write to."}
	}
	if out, err := exec.Command("modinfo", "v4l2loopback").CombinedOutput(); err == nil && len(out) > 0 {
		return Status{CanEnable: true, Name: cardLabel,
			Help: "Turn on the virtual camera to add \"" + cardLabel + "\" for Zoom, Meet, Teams, OBS and the browser. Linux asks for your password to load the camera driver."}
	}
	return Status{
		Help:       "Install v4l2loopback (Debian/Ubuntu: sudo apt install v4l2loopback-dkms; Fedora: sudo dnf install v4l2loopback; Arch: sudo pacman -S v4l2loopback-dkms), then turn on the virtual camera here.",
		InstallURL: "https://github.com/umlaeute/v4l2loopback",
	}
}

func enable() error {
	cmd := exec.Command("pkexec", "modprobe", "v4l2loopback", "devices=1", "exclusive_caps=1", "card_label="+cardLabel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("load v4l2loopback: %v %s", err, bytes.TrimSpace(out))
	}
	return nil
}

type v4l2Output struct {
	f   *os.File
	buf []byte
	w   int
	h   int
}

func open(w, h, _ int) (Output, error) {
	devs := loopbacks()
	if len(devs) == 0 {
		return nil, fmt.Errorf("no virtual camera device; %s", detect().Help)
	}
	f, err := os.OpenFile(devs[0], os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	w &^= 1
	var fm v4l2Format
	fm.Type = bufTypeOutput
	fm.Pix = v4l2PixFormat{Width: uint32(w), Height: uint32(h), PixelFormat: fourccYUYV, Field: fieldNone,
		BytesPerLine: uint32(w * 2), SizeImage: uint32(w * h * 2), Colorspace: colorspaceSRGB}
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), vidiocSFmt, uintptr(unsafe.Pointer(&fm))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("set virtual camera format: %v", e)
	}
	return &v4l2Output{f: f, buf: make([]byte, w*h*2), w: w, h: h}, nil
}

func (o *v4l2Output) Write(img *image.RGBA) error {
	if img.Rect.Dx()&^1 != o.w || img.Rect.Dy() != o.h {
		return fmt.Errorf("frame size %v doesn't match the virtual camera's %dx%d", img.Rect.Size(), o.w, o.h)
	}
	toYUYV(o.buf, img)
	_, err := o.f.Write(o.buf)
	return err
}

func (o *v4l2Output) Close() error { return o.f.Close() }
