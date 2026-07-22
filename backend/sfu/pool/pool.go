package pool

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

// Free stores the index of the slots.
type Pool struct {
	Slots   []*MediaSlot // Holds all the slots activated and deactivated
	Free    []int        // Slots that are free and can be assigned
	Pending []int        // Slots that are de-activated and can not be assigned.

	Mu sync.Mutex
}

// Our debouncing approach starts with 0 transceivers there is no pre-allocation hence return null slots and free.
func NewPool() *Pool {
	return &Pool{
		Slots: nil,
		Free:  nil,
	}
}

// Call this directly and get a slot for the track.
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

// Mark a slot as empty and add it to the Free pool to be reused.
func (p *Pool) Release(idx int) {
	p.Mu.Lock()

	// Call the slot function to clear the slot and increment the generation.
	p.Slots[idx].Clear()

	// Append the Free array and add the index to the free list.
	p.Free = append(p.Free, idx)
	p.Mu.Unlock()
}

// Add more transceivers after a short period of debouncing and collecting how many transceivers are needed.
func (p *Pool) Grow(t *webrtc.RTPTransceiver) *MediaSlot {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	// Create slots for these transceivers for future use and return the array of slots.
	idx := len(p.Slots)

	// ready is false by default.
	slot := &MediaSlot{
		Transceiver: t,
		Index:       idx,
	}

	p.Slots = append(p.Slots, slot)
	p.Pending = append(p.Pending, idx)

	return slot
}

// Slots slice contains all the newSlots created in the related Grow() call.
// We pass slots and don't just traverse the Pending array as another routine can call Grow() and modify more slots in the pending array but I should not activate those with the first Grow call as they both have different renegotiation cycles.
func (p *Pool) ActivatePendingSlots(slots []*MediaSlot) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	// Add the pending slots for this current grow call in a map.
	activationBatch := make(map[int]struct{}, len(slots))
	for _, s := range slots {
		activationBatch[s.Index] = struct{}{}
	}

	// Reuse Pending's backing array with length 0.
	remaining := p.Pending[:0]

	// Traverse the actual pending array.
	for _, idx := range p.Pending {

		// If the slot is not in the map then it does not belong to this Grow call that we are currently processing else push in the remaining array.
		if _, ok := activationBatch[idx]; !ok {
			remaining = append(remaining, idx)
			continue
		}

		// Activate the slot and push in the free array.
		p.Slots[idx].Activate()
		p.Free = append(p.Free, idx)
	}

	// Update the pending array to only the remaining unactivated slots.
	p.Pending = remaining
}

// Before we publish any track to a new slot we should confirm if it is published already or not.
func (p *Pool) ContainsTrack(publisherID, trackID string) bool {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	for _, slot := range p.Slots {
		state := slot.Load()
		if state == nil {
			continue
		}

		if state.PublisherId == publisherID &&
			state.TrackID == trackID {
			return true
		}
	}

	return false
}
