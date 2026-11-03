package vcam

import (
	"testing"
	"unsafe"
)

func TestV4L2StructSizes(t *testing.T) {
	if s := unsafe.Sizeof(v4l2Capability{}); s != 104 {
		t.Fatalf("v4l2_capability is %d bytes, want 104", s)
	}
	if s := unsafe.Sizeof(v4l2Format{}); s != 208 {
		t.Fatalf("v4l2_format is %d bytes, want 208", s)
	}
}
