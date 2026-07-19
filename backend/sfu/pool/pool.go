package pool

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

// Free stores the index of the slots.
type Pool struct {
	Slots []*MediaSlot
	Free  []int

	Mu sync.Mutex
}

// Call this directly and get a slot for the track. Secure abstraction
// returns *MediaSlot, generation, doneCh, a bool for wether found a slot or not true if found else false.
func (p *Pool) Acquire(publisherId, trackId string, kind webrtc.RTPCodecType) (*MediaSlot, uint64, <-chan struct{}, bool) {
	p.Mu.Lock()

	// Check if any slot is free or not.
	if len(p.Free) == 0 {
		p.Mu.Unlock()
		return nil, 0, nil, false
	}

	// Find the index of the free slot.
	idx := p.Free[len(p.Free)-1]
	p.Free = p.Free[:len(p.Free)-1]
	p.Mu.Unlock()

	// Assign the track to the selected slot.
	slot := p.Slots[idx]
	gen, done := slot.Assign(publisherId, trackId, kind)

	return slot, gen, done, true
}

func (p *Pool) Release(idx int) {
	// Call the slot function to clear the slot and increment the generation.
	p.Slots[idx].Clear()

	// Append the Free array and add the index to the free list.
	p.Mu.Lock()
	p.Free = append(p.Free, idx)
	p.Mu.Unlock()
}

func (p *Pool) Grow(n int, addTranceiver func(n int) ([]*webrtc.RTPTransceiver, error)) ([]*MediaSlot, error) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	// Create the n tranceivers.
	transceivers, err := addTranceiver(n)
	if err != nil {
		return nil, err
	}

	// Create slots for these transceivers for future use and return the array of slots.
	out := make([]*MediaSlot, 0, n)
	for _, t := range transceivers {
		idx := len(p.Slots)

		slot := &MediaSlot{
			Transceiver: t,
			Index:       idx,
		}

		p.Slots = append(p.Slots, slot)
		p.Free = append(p.Free, idx)
		out = append(out, slot)
	}

	return out, nil
}
