package participant

import (
	"backend/types"
	"fmt"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

type Publisher struct {
	PC                *webrtc.PeerConnection
	client            *Client
	RemoteDescSet     bool
	PendingCandidates []types.ICECandidateMessage
	callbacks         PublisherCallbacks

	Mu sync.RWMutex
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
		PendingCandidates: make([]types.ICECandidateMessage, 0),
		callbacks:         callbacks,
	}

	//Set up pc events onTrack, onICECandidates, onConnectionStateChange
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Println("[Publisher] Received new media track")

		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			track.Codec().RTPCodecCapability,
			track.ID(),
			track.StreamID(),
		)

		if err != nil {
			log.Println("[Publisher] Error creating localTrack:", err)
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
		}

		p.callbacks.OnTrackPublished(publishedTrack, p.client)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("[Publisher] Connection state is:", state)
		//implement cleanup and re connection logic here
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Println("[Publisher] ICE state is:", state)
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {

		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateJSON := candidate.ToJSON()

		// Create a object to send to the frontend
		msg := types.ICECandidateMessage{
			Type:         "ice-candidate",
			ICECandidate: candidateJSON,
		}

		// Emit a socket event for frontend to catch this ice candidate
		p.client.SafeSend(msg)
	})

	return p, nil
}

func (p *Publisher) FlushICECandidateQueue() {
	p.Mu.Lock()

	if !p.RemoteDescSet {
		p.Mu.Unlock()

		log.Println("[Publisher] Remote Description not set yet. Cannot flush the ICE Candidate queue.")
		return
	}

	// candidate is already of type ICECandidateInit
	candidates := append([]types.ICECandidateMessage(nil), p.PendingCandidates...)

	//Empty the queue.
	p.PendingCandidates = nil

	p.Mu.Unlock()

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := p.PC.AddICECandidate(candidate.ICECandidate)
		if err != nil {
			log.Println("[HandleICECandidate] Error adding ICE candidate to the queue:", err)
			return
		}
	}
}

func (p *Publisher) HandleOffer(signal types.SignalMessage, client *Client) (webrtc.SessionDescription, error) {
	pc := p.PC

	//Set up the received remote SDP
	//trickle ice is started as soon i set any description local or remote
	err := pc.SetRemoteDescription(signal.SDP)
	if err != nil {
		log.Println("[Publisher] Error setting remote description:", err)
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
		log.Println("[Publisher] Error creating answer:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	//Set up local description
	err = pc.SetLocalDescription(answer)
	if err != nil {
		log.Println("[Publisher] Error setting local description:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

// Implement a queue to prevent drop of ice candidates as they might arrive before or after the setDescription
func (p *Publisher) HandleICECandidate(candidate types.ICECandidateMessage, client *Client) {

	//candidate is a object that containes Candidate
	p.Mu.Lock()

	if !p.RemoteDescSet {
		p.PendingCandidates = append(p.PendingCandidates, candidate)
		p.Mu.Unlock()
		return
	}
	p.Mu.Unlock()

	err := p.PC.AddICECandidate(candidate.ICECandidate)
	if err != nil {
		log.Println("[Publisher] Error adding ICE candidate to the queue:", err)
		return
	}
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
