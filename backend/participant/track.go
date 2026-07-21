package participant

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

// Remove the Mutex from here as this value is fully initilized once and then only passed and read from and is never mutated.
type PublishedTrack struct {
	PublisherID string
	TrackID     string
	StreamID    string
	SSRC        webrtc.SSRC
	Kind        webrtc.RTPCodecType
	LocalTrack  *webrtc.TrackLocalStaticRTP

	Mu sync.RWMutex
}
