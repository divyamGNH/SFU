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

// Remove the occupied bool and decide if a slot is free from the publisherId = "" then free else not.
// We have single video and audio pool right now if we in future create different pool for different codecs then we can make a array of pools.
// We should funnel every function or stuff related to Media slot through the Pool so we dont need a Mu in mediaSlot. But the RTCP like DrainRTCP etc need drect access so we can just use atomics instead of Mu.
// The options are use aotmics or strictly use MediaSlot Mu only when i access it directly and not through the Pool.
type MediaSlot struct {
	Transceiver *webrtc.RTPTransceiver
	// Occupied         bool
	PublisherId      string
	Kind             webrtc.RTPCodecType
	DrainRTCPStarted bool
	TrackID          string
	Index            int

	Mu sync.RWMutex
}

func (p *Pool) Acquire() {

}

func (p *Pool) Release() {

}

func (p *Pool) Grow() {

}

func (m *MediaSlot) Acquire() {

}

func (m *MediaSlot) Release() {

}
