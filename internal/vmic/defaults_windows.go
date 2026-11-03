package vmic

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows audio endpoints are reached through COM: IMMDeviceEnumerator
// lists them, and IPolicyConfig (the interface the Sound control panel
// itself uses, stable since Windows Vista) changes the default.
var (
	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	clsidPolicyConfig       = windows.GUID{Data1: 0x870AF99C, Data2: 0x171D, Data3: 0x4F9E, Data4: [8]byte{0xAF, 0x0D, 0xE6, 0x3D, 0xF4, 0x0C, 0x2B, 0xC9}}
	iidIPolicyConfig        = windows.GUID{Data1: 0xF8679F50, Data2: 0x850A, Data3: 0x41CF, Data4: [8]byte{0x9C, 0x72, 0x43, 0x0F, 0x29, 0x02, 0x90, 0xC8}}
	pkeyDeviceFriendlyName  = propertyKey{fmtid: windows.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD, Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, pid: 14}

	ole32            = windows.NewLazySystemDLL("ole32.dll")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	propVariantClear = ole32.NewProc("PropVariantClear")
)

const (
	clsctxAll         = 0x17
	deviceStateActive = 1
	stgmRead          = 0
	vtLPWSTR          = 31
	eRender, eCapture = 0, 1
	// vtable slots
	methodRelease                  = 2
	enumEnumAudioEndpoints         = 3
	enumGetDefaultAudioEndpoint    = 4
	collectionGetCount             = 3
	collectionItem                 = 4
	deviceOpenPropertyStore        = 4
	deviceGetID                    = 5
	propertyStoreGetValue          = 5
	policyConfigSetDefaultEndpoint = 13
)

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

type propVariant struct {
	vt       uint16
	reserved [3]uint16
	val      *uint16
	pad      uintptr
}

// comObject is a COM interface pointer: its first word is the vtable.
type comObject struct{ vtbl *[32]uintptr }

func (o *comObject) call(method int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() { o.call(methodRelease) }

// withCOM runs fn on a locked OS thread with COM initialized.
func withCOM(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil || err == syscall.Errno(1) /* S_FALSE */ {
		defer windows.CoUninitialize()
	}
	return fn()
}

func create(clsid, iid *windows.GUID) (*comObject, error) {
	var o *comObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&o)))
	if hr != 0 || o == nil {
		return nil, fmt.Errorf("CoCreateInstance: 0x%08x", uint32(hr))
	}
	return o, nil
}

func endpoints(output bool) ([]Endpoint, error) {
	flow := uintptr(eCapture)
	if output {
		flow = eRender
	}
	var eps []Endpoint
	err := withCOM(func() error {
		en, err := create(&clsidMMDeviceEnumerator, &iidIMMDeviceEnumerator)
		if err != nil {
			return err
		}
		defer en.release()
		defID := ""
		var def *comObject
		if en.call(enumGetDefaultAudioEndpoint, flow, 0 /* eConsole */, uintptr(unsafe.Pointer(&def))) == 0 && def != nil {
			defID = deviceID(def)
			def.release()
		}
		var coll *comObject
		if hr := en.call(enumEnumAudioEndpoints, flow, deviceStateActive, uintptr(unsafe.Pointer(&coll))); hr != 0 || coll == nil {
			return fmt.Errorf("EnumAudioEndpoints: 0x%08x", uint32(hr))
		}
		defer coll.release()
		var count uint32
		coll.call(collectionGetCount, uintptr(unsafe.Pointer(&count)))
		for i := uint32(0); i < count; i++ {
			var dev *comObject
			if coll.call(collectionItem, uintptr(i), uintptr(unsafe.Pointer(&dev))) != 0 || dev == nil {
				continue
			}
			id := deviceID(dev)
			eps = append(eps, Endpoint{ID: id, Name: friendlyName(dev), Default: id != "" && id == defID})
			dev.release()
		}
		return nil
	})
	return eps, err
}

func deviceID(dev *comObject) string {
	var s *uint16
	if dev.call(deviceGetID, uintptr(unsafe.Pointer(&s))) != 0 || s == nil {
		return ""
	}
	id := windows.UTF16PtrToString(s)
	windows.CoTaskMemFree(unsafe.Pointer(s))
	return id
}

func friendlyName(dev *comObject) string {
	var store *comObject
	if dev.call(deviceOpenPropertyStore, stgmRead, uintptr(unsafe.Pointer(&store))) != 0 || store == nil {
		return ""
	}
	defer store.release()
	var pv propVariant
	if store.call(propertyStoreGetValue, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv))) != 0 {
		return ""
	}
	defer propVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	if pv.vt != vtLPWSTR || pv.val == nil {
		return ""
	}
	return windows.UTF16PtrToString(pv.val)
}

// setDefault makes e the default for every role: console, multimedia and
// communications.
func setDefault(_ bool, e Endpoint) error {
	return withCOM(func() error {
		pc, err := create(&clsidPolicyConfig, &iidIPolicyConfig)
		if err != nil {
			return err
		}
		defer pc.release()
		id, err := windows.UTF16PtrFromString(e.ID)
		if err != nil {
			return err
		}
		for role := uintptr(0); role < 3; role++ {
			if hr := pc.call(policyConfigSetDefaultEndpoint, uintptr(unsafe.Pointer(id)), role); hr != 0 {
				return fmt.Errorf("set default device: 0x%08x", uint32(hr))
			}
		}
		return nil
	})
}
