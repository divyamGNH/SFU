package participant

import (
	"backend/sfu/pool"
	"backend/types"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

type Subscriber struct {
	PC                 *webrtc.PeerConnection
	client             *Client
	RemoteDescSet      bool
	PendingCandidates  []types.ICECandidateMessage
	currentGeneration  uint64
	inflightGeneration uint64
	VideoPool          *pool.Pool
	AudioPool          *pool.Pool
	callbacks          SubscriberCallbacks
	// VideoDebouncer     *Debouncer
	// AudioDebouncer     *Debouncer

	negotiatingDebouncer *Debouncer
	negotiating          bool
	renegotiate          bool

	Mu          sync.RWMutex
	negotiateMu sync.Mutex
}

// All the callback functions are required so none of them must be nil.
func validateSubscriberCallbacks(callbacks SubscriberCallbacks) error {
	if callbacks.OnNegotiationCompleted == nil {
		return fmt.Errorf("participant: OnTrackPublished callback is required")
	}

	return nil
}

// We do not send any initial offer here we trust the negotiate to send the offer automatically when tracks arrive using pool.Grow function.
func NewSubscriber(iceServers []webrtc.ICEServer, callbacks SubscriberCallbacks, client *Client) (*Subscriber, error) {
	// Validate all the callbacks we receive.
	err := validateSubscriberCallbacks(callbacks)
	if err != nil {
		return nil, err
	}

	// Create a new Peer Connection.
	pc, err := NewPeerConnection(&PcConfig{iceServers: iceServers})
	if err != nil {
		return nil, err
	}

	sub := &Subscriber{
		PC:                 pc,
		client:             client,
		RemoteDescSet:      false,
		PendingCandidates:  make([]types.ICECandidateMessage, 0, 256),
		currentGeneration:  1,
		inflightGeneration: 1,
		VideoPool:          pool.NewPool(),
		AudioPool:          pool.NewPool(),
		callbacks:          callbacks,
	}

	// Set the negotiating debouncer.
	sub.negotiatingDebouncer = NewDebouncer(
		50*time.Millisecond,
		sub.negotiate,
	)

	sub.PC.OnICECandidate(func(candidate *webrtc.ICECandidate) {

		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateJSON := candidate.ToJSON()

		// Create a object to send to the frontend
		msg := types.ICECandidateMessage{
			Type:         "subscriber-ice-candidate",
			ICECandidate: candidateJSON,
		}

		// Emit a socket event for frontend to catch this ice candidate
		client.SafeSend(msg)
	})

	sub.PC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("[Subscriber] Subscriber Connection state is:", state)
		//implement cleanup and re connection logic here
	})

	sub.PC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Println("[Subscriber] Subscriber ICE state is:", state)
	})

	sub.PC.OnNegotiationNeeded(func() {
		log.Println("NEGOTIATION NEEDED")
		sub.RequestNegotiate()
	})

	return sub, nil
}

func (s *Subscriber) RequestNegotiate() {
	s.negotiatingDebouncer.Fire()
}

func (s *Subscriber) negotiate() {
	s.negotiateMu.Lock()

	if s.negotiating {
		s.renegotiate = true
		s.negotiateMu.Unlock()
		return
	}

	s.negotiating = true
	s.negotiateMu.Unlock()
	s.createAndSendOffer()
}

func (s *Subscriber) createAndSendOffer() {
	subPC := s.PC

	// Snapshot the current generation for this inflight offer
	s.inflightGeneration = s.currentGeneration
	// Increment generation so new GrowPool calls batch into the next generation
	s.currentGeneration++

	offer, err := subPC.CreateOffer(nil)
	if err != nil {
		log.Println("[Subscriber] Error creating offer:", err)
		s.clearNegotiating()
		s.CleanUpSubscriber()
		return
	}

	if err := subPC.SetLocalDescription(offer); err != nil {
		log.Println("[Subscriber] Error setting local description:", err)
		s.clearNegotiating()
		s.CleanUpSubscriber()
		return
	}

	msg := types.SubscriberOfferMessage{
		Type: "subscriber-offer",
		SDP:  offer,
	}
	s.client.SafeSend(msg)
}

func (s *Subscriber) clearNegotiating() {
	s.negotiateMu.Lock()
	s.negotiating = false
	s.negotiateMu.Unlock()
}

func (s *Subscriber) FlushICECandidateQueue() {
	s.Mu.Lock()

	if !s.RemoteDescSet {
		s.Mu.Unlock()

		log.Println("[Subscriber] Remote Description for subscriber not set yet. Cannot flush the ICE Candidate queue for subscriber.")
		return
	}

	// candidate is already of type ICECandidateInit
	candidates := append([]types.ICECandidateMessage(nil), s.PendingCandidates...)

	//Empty the queue.
	s.PendingCandidates = nil

	s.Mu.Unlock()

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := s.PC.AddICECandidate(candidate.ICECandidate)
		if err != nil {
			log.Println("[HandleICECandidate] Error adding subscriber ICE candidate to the queue:", err)
			continue
		}
	}
}

func (s *Subscriber) HandleAnswer(answer *types.SubscriberAnswerMessage) error {
	log.Printf("Subscriber answer from %s", s.client.UserId)
	remoteSDP := answer.SDP

	subscriberPc := s.PC

	err := subscriberPc.SetRemoteDescription(remoteSDP)
	if err != nil {
		return err
	}

	s.Mu.Lock()
	s.RemoteDescSet = true
	s.Mu.Unlock()

	s.VideoPool.ActivateGeneration(s.inflightGeneration)
	s.AudioPool.ActivateGeneration(s.inflightGeneration)

	// Set negotiation complete to notify the room handler to retry the track it couldnt send before Growing the pool.
	s.callbacks.OnNegotiationCompleted(s.client)

	// Flush the ICE candidate queue as the remote desc is not set.
	s.FlushICECandidateQueue()

	// This is the media handling will be routed by room handlers to the sfu(media engine).
	// s.SendRemoteMediaToLocalPeer(client)

	s.negotiateMu.Lock()
	s.negotiating = false
	fireAgain := s.renegotiate
	s.renegotiate = false
	s.negotiateMu.Unlock()

	if fireAgain {
		s.negotiate()
	}

	// return no error.
	return nil
}

func (s *Subscriber) HandleIce(iceMessage types.ICECandidateMessage) {
	s.Mu.Lock()
	if s == nil || s.PC == nil || !s.RemoteDescSet {
		s.PendingCandidates = append(s.PendingCandidates, iceMessage)
		s.Mu.Unlock()
		return
	}

	subscriberPc := s.PC
	iceCandidate := iceMessage.ICECandidate
	s.Mu.Unlock()

	err := subscriberPc.AddICECandidate(iceCandidate)
	if err != nil {
		log.Println("Error adding ice candidate on the subscriber PC : ", err)
		return
	}
}

func (s *Subscriber) AddTransceiver(kind webrtc.RTPCodecType) (*webrtc.RTPTransceiver, error) {
	pc := s.PC

	// Create the transceiver.
	return pc.AddTransceiverFromKind(
		kind,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		},
	)
}

func (s *Subscriber) GrowPool(kind webrtc.RTPCodecType) error {
	t, err := s.AddTransceiver(kind)
	if err != nil {
		return err
	}

	if kind == webrtc.RTPCodecTypeVideo {
		s.VideoPool.Grow(t, s.currentGeneration)
	} else {
		s.AudioPool.Grow(t, s.currentGeneration)
	}

	return nil
}

func (s *Subscriber) SubscribeToTrack(track *PublishedTrack) (bool, bool, *pool.MediaSlot, error) {
	var trackPool *pool.Pool

	// Find which pool does the track belong to.
	if track.Kind == webrtc.RTPCodecTypeVideo {
		trackPool = s.VideoPool
	} else {
		trackPool = s.AudioPool
	}

	// Check if the track is already published or not. Return if yes.
	if trackPool.ContainsTrack(track.PublisherID, track.TrackID) {
		return false, true, nil, nil
	}

	// Acquire a slot.
	slot, _, _, ok := trackPool.Acquire(track.PublisherID, track.TrackID, track.Kind)
	if !ok {
		// Needs negotiation
		return true, false, nil, nil
	}

	// Replace the track from the slot.
	err := slot.Transceiver.Sender().ReplaceTrack(track.LocalTrack)
	if err != nil {
		trackPool.Release(slot.Index)
		return false, false, nil, err
	}

	return false, false, slot, nil
}

func (s *Subscriber) CleanUpSubscriber() {

	// Important to clean up external resources.
	if s.PC != nil {
		err := s.PC.Close()
		if err != nil {
			log.Println("Error closing Subscriber PC : ", err)
		}

		// We are not nulling the PC here is because there can be some other go-routines can be using this PC so just Close it and the garbage collector will clean up the PC once the publisher goes out of scope and all the routines die down.
		// s.PC = nil
	}

	// Optional to clean up internal resources but better practice and more safe to clean them.
	s.VideoPool = nil
	s.AudioPool = nil
	s.PendingCandidates = nil
}
