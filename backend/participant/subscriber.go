package participant

import (
	"backend/sfu/pool"
	"backend/types"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

type Subscriber struct {
	PC                 *webrtc.PeerConnection
	RemoteDescSet      bool
	PendingCandidates  []types.ICECandidateMessage
	PendingTransceiver []*webrtc.RTPTransceiver
	VideoPool          *pool.Pool
	AudioPool          *pool.Pool
	VideoDebouncer     *Debouncer
	AudioDebouncer     *Debouncer

	Mu sync.RWMutex
}

func (s *Subscriber) CleanUpSubscriber() {

	// Important to clean up external resources.
	if s.PC != nil {
		err := s.PC.Close()
		if err != nil {
			log.Println("Error closing Subscriber PC : ", err)
		}
		s.PC = nil
	}

	// Optional to clean up internal resources but better practice and more safe to clean them.
	s.VideoPool = nil
	s.AudioPool = nil
	s.PendingCandidates = nil
}
