package participant

import (
	"backend/types"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

type Publisher struct {
	PC                *webrtc.PeerConnection
	RemoteDescSet     bool
	PendingCandidates []types.ICECandidateMessage
	Mu                sync.RWMutex
}

func (p *Publisher) CleanUpPublisher() {

	// Important to clean up external resources.
	if p.PC != nil {
		err := p.PC.Close()
		if err != nil {
			log.Println("Error closing Publisher PC : ", err)
		}
		p.PC = nil
	}

	// Optional to clean up internal resources but better practice and more safe to clean them.
	p.PendingCandidates = nil
}
