package participant

import (
	"backend/logger"
	"fmt"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Publisher struct {
	PC                *webrtc.PeerConnection
	client            *Client
	RemoteDescSet     bool
	PendingCandidates []webrtc.ICECandidateInit
	callbacks         PublisherCallbacks

	Mu sync.RWMutex
}

// WriteRTCP allows Publisher to implement sfu.RTCPWriter without exposing the raw PC
func (p *Publisher) WriteRTCP(packets []rtcp.Packet) error {
	return p.PC.WriteRTCP(packets)
}

// All the callback functions are required so none of them must be nil.
func validatePublisherCallbacks(callbacks PublisherCallbacks) error {
	if callbacks.OnTrackPublished == nil {
		return fmt.Errorf("participant: OnTrackPublished callback is required")
	}

	return nil
}

// Pass the ice server from wherever we call NewPublisher and NewSubscriber.
func NewPublisher(iceServers []webrtc.ICEServer, callbacks PublisherCallbacks, client *Client) (*Publisher, error) {
	err := validatePublisherCallbacks(callbacks)
	if err != nil {
		return nil, err
	}

	pc, err := NewPeerConnection(&PcConfig{iceServers: iceServers})
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		PC:                pc,
		client:            client,
		RemoteDescSet:     false,
		PendingCandidates: make([]webrtc.ICECandidateInit, 0),
		callbacks:         callbacks,
	}

	//Set up pc events onTrack, onICECandidates, onConnectionStateChange
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		logger.Info("[Publisher] Received new media track")

		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			track.Codec().RTPCodecCapability,
			track.ID(),
			track.StreamID(),
		)

		if err != nil {
			logger.Error("[Publisher] Error creating localTrack:", err)
			return
		}

		// Create a publishedTrack instance.
		publishedTrack := &PublishedTrack{
			PublisherID: p.client.UserId,
			TrackID:     track.ID(),
			StreamID:    track.StreamID(),
			SSRC:        track.SSRC(),
			Kind:        track.Kind(),
			LocalTrack:  localTrack,
			RemoteTrack: track,
		}

		p.callbacks.OnTrackPublished(publishedTrack, p.client)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		logger.Info("[Publisher] Connection state is:", state)
		//implement cleanup and re connection logic here
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		logger.Info("[Publisher] ICE state is:", state)
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateInit := candidate.ToJSON()

		p.callbacks.SendPublisherICECandidate(p.client, candidateInit)
	})

	return p, nil
}

func (p *Publisher) FlushICECandidateQueue() {
	p.Mu.Lock()

	if !p.RemoteDescSet {
		p.Mu.Unlock()
		logger.Info("[Publisher] Remote Description not set yet. Cannot flush the ICE Candidate queue.")
		return
	}

	candidates := append([]webrtc.ICECandidateInit(nil), p.PendingCandidates...)

	//Empty the queue.
	p.PendingCandidates = nil
	p.Mu.Unlock()

	logger.Infof("[Publisher] Flushing %d pending ICE candidates", len(candidates))

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := p.PC.AddICECandidate(candidate)
		if err != nil {
			logger.Error("[HandleICECandidate] Error adding ICE candidate to the queue:", err)
			continue
		}
		logger.Info("[Publisher] Successfully added queued ICE candidate")
	}
}

func (p *Publisher) HandleOffer(sdpString string) (webrtc.SessionDescription, error) {
	pc := p.PC

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpString,
	}

	//Set up the received remote SDP
	//trickle ice is started as soon i set any description local or remote
	err := pc.SetRemoteDescription(offer)
	if err != nil {
		logger.Error("[Publisher] Error setting remote description:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	// Set the boolean true so that we can start flushing the ice candidate queue.
	p.Mu.Lock()
	p.RemoteDescSet = true
	p.Mu.Unlock()

	p.FlushICECandidateQueue()

	//Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		logger.Error("[Publisher] Error creating answer:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	//Set up local description
	err = pc.SetLocalDescription(answer)
	if err != nil {
		logger.Error("[Publisher] Error setting local description:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

func (p *Publisher) HandleICECandidate(candidate webrtc.ICECandidateInit) {
	p.Mu.Lock()

	logger.Infof("[Publisher] Received ICE candidate: %+v", candidate)

	if !p.RemoteDescSet {
		logger.Info("[Publisher] Remote description not set yet, queuing candidate")
		p.PendingCandidates = append(p.PendingCandidates, candidate)
		p.Mu.Unlock()
		return
	}
	p.Mu.Unlock()

	err := p.PC.AddICECandidate(candidate)
	if err != nil {
		logger.Error("[Publisher] Error adding ICE candidate to the PC:", err)
		return
	}
	logger.Info("[Publisher] Successfully added ICE candidate directly")
}

func (p *Publisher) CleanUpPublisher() {

	// Important to clean up external resources.
	if p.PC != nil {
		err := p.PC.Close()
		if err != nil {
			logger.Error("Error closing Publisher PC : ", err)
		}

		// We are not nulling the PC here is because there can be some other go-routines can be using this PC so just Close it and the garbage collector will clean up the PC once the publisher goes out of scope and all the routines die down.
		// p.PC = nil
	}

	// Optional to clean up internal resources but better practice and more safe to clean them.
	p.PendingCandidates = nil
}
