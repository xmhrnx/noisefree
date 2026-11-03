package audio

import "sync/atomic"

// ring is a lock-free single-producer, single-consumer sample queue that
// carries processed audio from the main device to the monitor device.
type ring struct {
	buf  []float32
	w, r atomic.Uint64
}

func newRing(size int) *ring { return &ring{buf: make([]float32, size)} }

// write appends samples, dropping what doesn't fit (the monitor fell behind).
func (q *ring) write(s []float32) {
	w, r := q.w.Load(), q.r.Load()
	free := uint64(len(q.buf)) - (w - r)
	n := min(uint64(len(s)), free)
	for i := uint64(0); i < n; i++ {
		q.buf[(w+i)%uint64(len(q.buf))] = s[i]
	}
	q.w.Store(w + n)
}

// read fills s, padding with silence when the queue runs dry.
func (q *ring) read(s []float32) {
	w, r := q.w.Load(), q.r.Load()
	n := min(uint64(len(s)), w-r)
	for i := uint64(0); i < n; i++ {
		s[i] = q.buf[(r+i)%uint64(len(q.buf))]
	}
	clear(s[n:])
	q.r.Store(r + n)
}
