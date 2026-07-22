package participant

import (
	"sync"
	"time"
)

type Debouncer struct {
	pending int
	timer   *time.Timer
	onFlush func(n int)

	mu sync.Mutex
}

func NewDebouncer(onFlush func(n int)) *Debouncer {
	return &Debouncer{
		onFlush: onFlush,
	}
}

func (d *Debouncer) Request() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Increment the pending.
	d.pending++

	// Return if the timer already started.
	if d.timer != nil {
		return
	}

	// Start the timer if it was not started.
	d.timer = time.AfterFunc(50*time.Millisecond, d.FlushDebouncer)
}

func (d *Debouncer) FlushDebouncer() {
	d.mu.Lock()

	// Number of tracks waiting that could not find a slot.
	pending := d.pending

	// Reset the timer.
	d.timer = nil
	d.mu.Unlock()

	// Return if no extra slots needed.
	if pending == 0 {
		return
	}

	// Call the Grow functions to increase the slots and transceivers.
	d.onFlush(pending)
}
