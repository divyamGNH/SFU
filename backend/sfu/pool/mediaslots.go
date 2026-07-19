package pool

import (
	"sync"
	"sync/atomic"

	"github.com/pion/webrtc/v3"
)

// Remove the occupied bool and decide if a slot is free from the publisherId = "" then free else not.
// We have single video and audio pool right now if we in future create different pool for different codecs then we can make a array of pools.
// We should funnel every function or stuff related to Media slot through the Pool so we dont need a Mu in mediaSlot. But the RTCP like DrainRTCP etc need drect access so we can just use atomics instead of Mu.
// The options are use atomics or strictly use MediaSlot Mu only when i access it directly and not through the Pool.

// Slotstate is immutable once made never edit. Every time replace with a new object and atomic swap it.
// As we never mutate this we dont need a mutex.
// Generation is incremented every time it's state changes in Assign and Clear.
type SlotState struct {
	PublisherId      string
	TrackID          string
	Kind             webrtc.RTPCodecType
	DrainRTCPStarted bool
	Generation       uint64
}

// state has a atomic pointer to slotState and slotState itself does not have any Mutex or Atomic.
type MediaSlot struct {
	Transceiver *webrtc.RTPTransceiver
	Index       int

	state  atomic.Pointer[SlotState] // lock-free reads for RTCP
	doneCh chan struct{}
	mu     sync.RWMutex
}

// Called by pool.Acquire
func (m *MediaSlot) Assign(publisherId, trackId string, kind webrtc.RTPCodecType) (uint64, chan struct{}) {
	m.mu.Lock()

	// Close the channel if it is not null yet.
	if m.doneCh != nil {
		close(m.doneCh)
	}

	// Set the generation to previous+1.
	var gen uint64 = 1
	if prev := m.state.Load(); prev != nil {
		gen = prev.Generation + 1
	}

	// SlotStates are immutable so each time do a atomic swap .
	m.state.Store(&SlotState{
		PublisherId: publisherId,
		TrackID:     trackId,
		Kind:        kind,
		Generation:  gen,
	})

	// Create a channel to allow RTCP reading again for the new track.
	m.doneCh = make(chan struct{})
	m.mu.Unlock()
	return gen, m.doneCh
}

// Called by pool.Release
func (m *MediaSlot) Clear() {
	m.mu.Lock()

	// Close the channel and mark it null.
	if m.doneCh != nil {
		close(m.doneCh)
		m.doneCh = nil
	}

	// Increment the generation of the slot.
	var gen uint64 = 1
	if prev := m.state.Load(); prev != nil {
		gen = prev.Generation + 1
	}

	// Atomic swap a new slot state with only the generation.
	m.state.Store(&SlotState{
		Generation: gen,
	})
}

func (m *MediaSlot) Load() *SlotState {
	return m.state.Load()
}

func (m *MediaSlot) MarkOrVerifyDrainRTCPStarted(expectedGen uint64) bool {
	for {
		current := m.state.Load()

		// Same generation check to verify if the slot has been reused or still in use.
		if current.Generation != expectedGen {
			return false
		}

		// If the drain has already started.
		if current.DrainRTCPStarted {
			return true
		}

		// If it was not started and is started now so flip the bool in the SlotState to true.
		next := *current
		next.DrainRTCPStarted = true
		if m.state.CompareAndSwap(current, &next) {
			return true
		}

		// If the CAS(Compare and Swap) fails for some reason like lets assume another routine came just in between the Load() and COmpareAndSwap() function and updated the "current" slotState then the "current" slotState we hold is outdated and we can not verify using this. Hence loop again for the new updated "current".
	}
}
