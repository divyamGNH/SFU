package participant

import (
	"sync"
	"time"
)

// A generic, payload-free trailing debounce — this can replace whatever
// count-based Debouncer you had, since there's no longer a count to carry.
type Debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	delay time.Duration
	fn    func()
}

func NewDebouncer(delay time.Duration, fn func()) *Debouncer {
	return &Debouncer{delay: delay, fn: fn}
}

func (d *Debouncer) Fire() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, d.fn)
}
