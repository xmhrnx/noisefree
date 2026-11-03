package vcam

import (
	"testing"
	"unsafe"
)

func TestQueueHeaderMatchesOBS(t *testing.T) {
	if s := unsafe.Sizeof(queueHeader{}); s != headerSize {
		t.Fatalf("queue_header is %d bytes, want %d", s, headerSize)
	}
	if o := unsafe.Offsetof(queueHeader{}.Interval); o != 40 {
		t.Fatalf("interval at %d, want 40", o)
	}
}
