package participant

import (
	"sync"
	"time"
)

type Debouncer struct {
	pendingVideos int
	pendingAudios int
	timer         *time.Timer
	running       bool

	mu sync.Mutex
}

func (d *Debouncer) RequestVidio() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pendingVideos++

	if d.timer != nil {
		return
	}

	d.timer = time.AfterFunc(50*time.Millisecond, d.FlushDebouncer)
}

func (d *Debouncer) RequestAudio() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pendingAudios++

	if d.timer != nil {
		return
	}

	d.timer = time.AfterFunc(50*time.Millisecond, d.FlushDebouncer)
}

func (d *Debouncer) FlushDebouncer() {
	d.mu.Lock()

	video := d.pendingVideos
	audio := d.pendingAudios

	// Reset the timer.
	d.timer = nil
	d.mu.Unlock()

	// Return if no extra slots needed.
	if video == 0 && audio == 0 {
		return
	}

	// Call the Grow functions to increase the slots and transceivers.

}
