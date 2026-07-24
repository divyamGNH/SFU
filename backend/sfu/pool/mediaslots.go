package pool

import (
	"sync"
	"sync/atomic"

	"github.com/pion/webrtc/v3"
)

// We have single video and audio pool right now if we in future create different pool for different codecs then we can make a array of pools.

// Slotstate is immutable once made never edit. Every time replace with a new object and atomic swap it.
// As we never mutate this we dont need a mutex.
// Generation is incremented every time it's state changes in Assign and Clear.
type SlotState struct {
	PublisherId string
	TrackID     string
	Kind        webrtc.RTPCodecType
	Generation  uint64
}

// state has a atomic pointer to slotState and slotState itself does not have any Mutex or Atomic.
// ready is a bool whihc is set true once the transceiver of the slot is stable that is only after the renegotiation is complete and the remoteSDP is set. Untill then it says in a deactivated state and can not be Acquired or Assigned
type MediaSlot struct {
	Transceiver           *webrtc.RTPTransceiver
	Index                 int
	NegotiationGeneration uint64

	// This ready is currently redundant as we use a free and a pending slice and only ever loop up the free slice while acquiring.
	ready            atomic.Bool // Decides if a slot is ready to be Acquired/Assigned or not. False by default
	drainRTCPStarted uint32
	state            atomic.Pointer[SlotState] // lock-free reads for RTCP
	doneCh           chan struct{}
	mu               sync.RWMutex
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
	defer m.mu.Unlock()

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

func (m *MediaSlot) Activate() {
	// Activate the slot.
	m.ready.Store(true)
}

// TryStartDrainRTCP returns true exactly once for this slot's entire lifetime —
// the first caller to win the CAS (Compare and Swap) is the one that should launch the RTCP reader.
// Every call after that returns false, forever.
func (m *MediaSlot) TryStartDrainRTCP() bool {
	return atomic.CompareAndSwapUint32(&m.drainRTCPStarted, 0, 1)
}
