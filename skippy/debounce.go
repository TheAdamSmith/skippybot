package skippy

import (
	"sync"
	"time"
)

type Debouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration
}

func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{
		timers: make(map[string]*time.Timer),
		delay:  delay,
	}
}

func (d *Debouncer) Debounce(timerId string, callback func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	timer, exists := d.timers[timerId]
	if exists {
		timer.Stop()
	}
	d.timers[timerId] = time.AfterFunc(d.delay, func() {
		callback()
		delete(d.timers, timerId)
	})
}
