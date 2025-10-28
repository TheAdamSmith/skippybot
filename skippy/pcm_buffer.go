package skippy

import "sync"

type PCMBuffer struct {
	mu  sync.Mutex
	buf []int16
}

func NewPCMBuffer() *PCMBuffer {
	return &PCMBuffer{
		buf: make([]int16, 0),
	}
}

func (b *PCMBuffer) Write(samples []int16) {
	b.mu.Lock()
	b.buf = append(b.buf, samples...)
	b.mu.Unlock()
}

// ReadFrame reads and removes a frame of length frameSize from the buffer.
// It returns the frame and true if enough samples were available, otherwise nil and false.
func (b *PCMBuffer) ReadFrame(frameSize int) ([]int16, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buf) < frameSize {
		return nil, false
	}

	frame := make([]int16, frameSize)
	copy(frame, b.buf[:frameSize])
	b.buf = b.buf[frameSize:]
	return frame, true
}

func (b *PCMBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf)
}
