package participant

import (
	"backend/config"
	"backend/types"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Publisher struct {
	PC                *webrtc.PeerConnection
	RemoteDescSet     bool
	PendingCandidates []types.ICECandidateMessage
	handler           TransportHandler

	Mu sync.RWMutex
}

func NewPublisher(handler TransportHandler) (*Publisher, error) {
	iceServers := config.FetchICEServers()
	pcConfig := &PcConfig{
		iceServers: iceServers,
	}
	pc, err := NewPeerConnection(pcConfig)

	if err != nil {
		return nil, err
	}

	return &Publisher{
		PC:                pc,
		PendingCandidates: make([]types.ICECandidateMessage, 0),
		handler:           handler,
	}, nil
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

func (p *Publisher) HandleOffer(signal types.SignalMessage, conn *websocket.Conn, client *Client) (webrtc.SessionDescription, error) {
	pc := p.PC

	//Set up pc events onTrack, onICECandidates, onConnectionStateChange
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		p.handler.OnTrack(track, receiver, client)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("[SFU] Connection state is:", state)
		//implement cleanup and re connection logic here
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Println("[SFU] ICE state is:", state)
	})

	//Set up the received remote SDP
	//trickle ice is started as soon i set any description local or remote
	err := pc.SetRemoteDescription(signal.SDP)
	if err != nil {
		log.Println("[SFU] Error setting remote description:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	// Set the boolean true so that we can start flushing the ice candidate queue.
	client.Publisher.Mu.Lock()
	client.Publisher.RemoteDescSet = true
	client.Publisher.Mu.Unlock()

	p.FlushICECandidateQueue()

	//Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println("[SFU] Error creating answer:", err)
		pc.Close()
		return webrtc.SessionDescription{}, err
	}

	//Set up local description
	err = pc.SetLocalDescription(answer)
	if err != nil {
		log.Println("[SFU] Error setting local description:", err)
		pc.Close()
		return webrtc.SessionDescription{}, nil
	}

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
		client.SafeSend(msg)
	})

	return answer, nil
}

// Implement a queue to prevent drop of ice candidates as they might arrive before or after the setDescription
func (p *Publisher) HandleICECandidate(candidate types.ICECandidateMessage, client *Client) {

	//candidate is a object that containes Candidate
	client.Publisher.Mu.Lock()

	if !client.Publisher.RemoteDescSet {
		client.Publisher.PendingCandidates = append(client.Publisher.PendingCandidates, candidate)
		client.Publisher.Mu.Unlock()
		return
	}
	client.Publisher.Mu.Unlock()

	err := client.Publisher.PC.AddICECandidate(candidate.ICECandidate)
	if err != nil {
		log.Println("[SFU] Error adding ICE candidate to the queue:", err)
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
