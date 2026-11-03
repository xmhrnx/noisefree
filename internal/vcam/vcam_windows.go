package vcam

import (
	"errors"
	"fmt"
	"image"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// OBS Studio's virtual camera is a DirectShow filter that reads frames from
// a shared-memory queue named OBSVirtualCamVideo; any app can write to it
// while OBS itself isn't using it. Layout from obs-studio
// plugins/win-dshow/shared-memory-queue.c.
const (
	queueName       = "OBSVirtualCamVideo"
	stateReady      = 2
	stateStopping   = 3
	frameHeaderSize = 32
	headerSize      = 80 // sizeof(struct queue_header)
	// The filter's CLSID, registered when OBS is installed.
	obsFilterCLSID = `CLSID\{A3FCE0F5-3493-419F-958A-ABA1250EC20B}`
)

// queueHeader mirrors struct queue_header.
type queueHeader struct {
	WriteIdx uint32
	ReadIdx  uint32
	State    uint32
	Offsets  [3]uint32
	Type     uint32
	Cx, Cy   uint32
	_        uint32
	Interval uint64
	Reserved [8]uint32
}

func align32(n uint32) uint32 { return (n + 31) &^ 31 }

var openFileMapping = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileMappingW")

// mappingExists reports whether another process (OBS) holds the queue.
func mappingExists(name *uint16) bool {
	h, _, _ := openFileMapping.Call(windows.FILE_MAP_READ, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return false
	}
	windows.CloseHandle(windows.Handle(h))
	return true
}

func obsInstalled() bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, obsFilterCLSID, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func detect() Status {
	if obsInstalled() {
		return Status{Ready: true, Name: "OBS Virtual Camera"}
	}
	return Status{
		Help:       "Install OBS Studio (free) once to get its virtual camera driver; you don't need to open OBS. Then pick \"OBS Virtual Camera\" in Zoom, Teams, Meet or the browser.",
		InstallURL: "https://obsproject.com/download",
	}
}

func enable() error { return errors.New("install OBS Studio to add the virtual camera") }

type obsOutput struct {
	h      windows.Handle
	view   uintptr
	header *queueHeader
	frames [3][]byte
	ts     [3]*uint64
	buf    []byte
	w, h2  int
	start  time.Time
}

func open(w, h, fps int) (Output, error) {
	if !obsInstalled() {
		return nil, errors.New(detect().Help)
	}
	w, h = w&^1, h&^1
	frameSize := uint32(w * h * 3 / 2)
	var offs [3]uint32
	size := align32(headerSize)
	for i := range offs {
		offs[i] = size
		size = align32(size + frameSize + frameHeaderSize)
	}
	name, _ := windows.UTF16PtrFromString(queueName)
	if mappingExists(name) {
		return nil, errors.New("OBS's virtual camera is in use by OBS; stop it there first")
	}
	hm, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, size, name)
	if err != nil {
		return nil, fmt.Errorf("create virtual camera queue: %w", err)
	}
	view, err := windows.MapViewOfFile(hm, windows.FILE_MAP_READ|windows.FILE_MAP_WRITE, 0, 0, 0)
	if err != nil {
		windows.CloseHandle(hm)
		return nil, fmt.Errorf("map virtual camera queue: %w", err)
	}
	// The view is memory the OS mapped, outside Go's heap: reinterpret the
	// address without a uintptr-to-pointer conversion vet would flag.
	base := *(*unsafe.Pointer)(unsafe.Pointer(&view))
	mem := unsafe.Slice((*byte)(base), size)
	o := &obsOutput{h: hm, view: view, header: (*queueHeader)(unsafe.Pointer(&mem[0])), buf: make([]byte, frameSize), w: w, h2: h, start: time.Now()}
	*o.header = queueHeader{State: 1 /* starting */, Offsets: offs, Cx: uint32(w), Cy: uint32(h),
		Interval: uint64(10_000_000 / max(fps, 1))}
	for i, off := range offs {
		o.ts[i] = (*uint64)(unsafe.Pointer(&mem[off]))
		o.frames[i] = mem[off+frameHeaderSize : off+frameHeaderSize+frameSize]
	}
	return o, nil
}

func (o *obsOutput) Write(img *image.RGBA) error {
	if img.Rect.Dx()&^1 != o.w || img.Rect.Dy()&^1 != o.h2 {
		return fmt.Errorf("frame size %v doesn't match the virtual camera's %dx%d", img.Rect.Size(), o.w, o.h2)
	}
	toNV12(o.buf, img)
	inc := atomic.AddUint32(&o.header.WriteIdx, 1)
	idx := inc % 3
	*o.ts[idx] = uint64(time.Since(o.start).Nanoseconds())
	copy(o.frames[idx], o.buf)
	atomic.StoreUint32(&o.header.ReadIdx, inc)
	atomic.StoreUint32(&o.header.State, stateReady)
	return nil
}

func (o *obsOutput) Close() error {
	atomic.StoreUint32(&o.header.State, stateStopping)
	windows.UnmapViewOfFile(o.view)
	return windows.CloseHandle(o.h)
}
