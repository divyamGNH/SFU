// TODO : i am chasing right now is that a new user wont trigger a re negotitation what u are saying is that on a forceful renegotitation we use the same PC saving us trouble for transceiver making again ?

package sfu

import (
	"backend/models"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type SFU struct {
	ConnToClient              map[*websocket.Conn]*models.Client
	UserIdToPublishedTracks   map[string][]*models.PublishedTrack
	SubscriberToOccupiedSlots map[string][]*models.MediaSlot
	rm                        RoomManager
	mu                        sync.RWMutex
}

func NewSFU(roomManager RoomManager) *SFU {
	return &SFU{
		ConnToClient:              make(map[*websocket.Conn]*models.Client),
		UserIdToPublishedTracks:   make(map[string][]*models.PublishedTrack),
		SubscriberToOccupiedSlots: make(map[string][]*models.MediaSlot),
		rm:                        roomManager,
	}
}

func (s *SFU) FlushICECandidateQueue(client *models.Client) {
	client.Publisher.Mu.Lock()

	if !client.Publisher.RemoteDescSet {
		client.Publisher.Mu.Unlock()

		log.Println("[SFU] Remote Description not set yet. Cannot flush the ICE Candidate queue.")
		return
	}

	// candidate is already of type ICECandidateInit
	candidates := append([]models.ICECandidateMessage(nil), client.Publisher.PendingCandidates...)

	//Empty the queue.
	client.Publisher.PendingCandidates = nil

	client.Publisher.Mu.Unlock()

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := client.Publisher.PC.AddICECandidate(candidate.ICECandidate)
		if err != nil {
			log.Println("[HandleICECandidate] Error adding ICE candidate to the queue:", err)
			return
		}
	}
}

func (s *SFU) FlushSubscriberICECandidateQueue(client *models.Client) {
	client.Subscriber.Mu.Lock()

	if !client.Subscriber.RemoteDescSet {
		client.Subscriber.Mu.Unlock()

		log.Println("[SFU] Remote Description for subscriber not set yet. Cannot flush the ICE Candidate queue for subscriber.")
		return
	}

	// candidate is already of type ICECandidateInit
	candidates := append([]models.ICECandidateMessage(nil), client.Subscriber.PendingCandidates...)

	//Empty the queue.
	client.Subscriber.PendingCandidates = nil

	client.Subscriber.Mu.Unlock()

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := client.Subscriber.PC.AddICECandidate(candidate.ICECandidate)
		if err != nil {
			log.Println("[HandleICECandidate] Error adding subscriber ICE candidate to the queue:", err)
			return
		}
	}
}

func (s *SFU) HandleOffer(signal models.SignalMessage, conn *websocket.Conn, client *models.Client) {

	log.Println("[SFU] Received offer")

	//Create a new PeerConnection object
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
			},
		},
	})
	if err != nil {
		log.Println("[HandleOffer] Error creating PeerConnection:", err)
		return
	}

	// log.Println("[HandleOffer] PeerConnection created successfully")

	// Earlier approach - Create the client object
	// We already get the client from the WS handler now

	// Add the PC to the client received
	publisher := &models.Publisher{
		PC:                pc,
		RemoteDescSet:     false,
		PendingCandidates: make([]models.ICECandidateMessage, 0, 256),
	}
	client.Publisher = publisher

	//Set up pc events onTrack, onICECandidates, onConnectionStateChange
	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

		log.Println("[SFU] Received new media track")

		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			remoteTrack.Codec().RTPCodecCapability,
			remoteTrack.ID(),
			remoteTrack.StreamID(),
		)

		if err != nil {
			log.Println("[SFU] Error creating localTrack:", err)
			return
		}

		// Create a publishedTrack instance.
		publishedTrack := &models.PublishedTrack{
			PublisherID: client.UserId,
			TrackID:     remoteTrack.ID(),
			StreamID:    remoteTrack.StreamID(),
			SSRC:        remoteTrack.SSRC(),
			Kind:        remoteTrack.Kind(),
			LocalTrack:  localTrack,
		}

		// Store the published track in the SFU struct map.
		s.mu.Lock()
		s.UserIdToPublishedTracks[client.UserId] = append(s.UserIdToPublishedTracks[client.UserId], publishedTrack)
		s.mu.Unlock()

		s.SendLocalMediaToRemotePeers(client)

		// TODO : Use RTCP packets for various things like bitrate etc instead of just draining them.

		//Manually handle each rtp packet
		//Can change and monitor everything such as codecs etc.
		for {

			//We are using ReadRTP here but another approach is using Read() with a buffer.
			packet, _, err := remoteTrack.ReadRTP()

			if err != nil {
				log.Println("[SFU] Error reading RTP packet:", err)
				break
			}

			//send each packet to through the pipeline made for media transfer using AddTrack
			err = localTrack.WriteRTP(packet)
			if err != nil {
				log.Println("[SFU] Error forwarding RTP packet:", err)
				break
			}
		}

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
	err = pc.SetRemoteDescription(signal.SDP)
	if err != nil {
		log.Println("[SFU] Error setting remote description:", err)
		pc.Close()
		return
	}

	// Set the boolean true so that we can start flushing the ice candidate queue.
	client.Publisher.Mu.Lock()
	client.Publisher.RemoteDescSet = true
	client.Publisher.Mu.Unlock()

	s.FlushICECandidateQueue(client)

	//Create answer
	localSDP, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println("[SFU] Error creating answer:", err)
		pc.Close()
		return
	}

	// log.Println("[SFU] Answer created successfully")

	answer := models.SignalMessage{
		Type: "answer",
		SDP:  localSDP,
	}

	//Set up local description
	err = pc.SetLocalDescription(localSDP)
	if err != nil {
		log.Println("[SFU] Error setting local description:", err)
		pc.Close()
		return
	}

	// log.Println("[SFU] Local description set successfully")

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {

		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateJSON := candidate.ToJSON()

		// Create a object to send to the frontend
		msg := models.ICECandidateMessage{
			Type:         "ice-candidate",
			ICECandidate: candidateJSON,
		}

		// Emit a socket event for frontend to catch this ice candidate
		client.SafeSend(msg)
	})

	s.mu.Lock()
	s.ConnToClient[conn] = client
	s.mu.Unlock()

	// log.Println("[SFU] Client added to ConnToClient map")

	client.SafeSend(answer)

	s.SendSubscriberOffer(client)

	log.Println("[SFU] Answer pushed into WritePump channel")
	log.Println("[SFU] Offer handling completed successfully")
}

// Implement a queue to prevent drop of ice candidates as they might arrive before or after the setDescription
func (s *SFU) HandleICECandidate(candidate models.ICECandidateMessage, client *models.Client) {

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

func (s *SFU) HandleSubscriberAnswer(answer *models.SubscriberAnswerMessage, client *models.Client) {
	remoteSDP := answer.SDP

	subscriberPc := client.Subscriber.PC

	err := subscriberPc.SetRemoteDescription(remoteSDP)
	if err != nil {
		log.Println("Error setting the remote description for subscriber PC : ", err)
	}

	client.Subscriber.Mu.Lock()
	client.Subscriber.RemoteDescSet = true
	client.Subscriber.Mu.Unlock()

	// Get all the tranceivers.
	s.SetTranceiversAsSlots(client)

	// Flush the ICE candidate queue as the remote desc is not set.
	s.FlushSubscriberICECandidateQueue(client)
	s.SendRemoteMediaToLocalPeer(client)
}

func (s *SFU) HandleSubscriberIce(iceMessage models.ICECandidateMessage, client *models.Client) {

	if client.Subscriber == nil {
		log.Println("Client subscriber is nil")
		return
	}

	client.Subscriber.Mu.Lock()
	if client.Subscriber == nil || client.Subscriber.PC == nil || !client.Subscriber.RemoteDescSet {
		client.Subscriber.PendingCandidates = append(client.Subscriber.PendingCandidates, iceMessage)
		client.Subscriber.Mu.Unlock()
		return
	}

	subscriberPc := client.Subscriber.PC
	iceCandidate := iceMessage.ICECandidate
	client.Subscriber.Mu.Unlock()

	err := subscriberPc.AddICECandidate(iceCandidate)
	if err != nil {
		log.Println("Error adding ice candidate on the subscriber PC : ", err)
		return
	}
}

func (s *SFU) SendSubscriberOffer(client *models.Client) {
	log.Println("Subscriber offer triggered")
	// Create the subscriber pc
	subscriberPc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
			},
		},
	})
	if err != nil {
		log.Println("[SFU] Error creating subscriber PC : ", err)
	}

	subscriber := &models.Subscriber{
		PC:                subscriberPc,
		RemoteDescSet:     false,
		PendingCandidates: make([]models.ICECandidateMessage, 0, 256),
		VideoSlots:        make([]*models.MediaSlot, 0, 10),
		AudioSlots:        make([]*models.MediaSlot, 0, 10),
	}

	client.Subscriber = subscriber

	if client.Subscriber == nil {
		log.Println("Client subscriber is nil")
		return
	}
	client.Subscriber.PC = subscriberPc

	// Pre define transceivers here. 10 for video and 10 for audio.
	err = s.CreateTranceivers(client)
	if err != nil {
		log.Println("Error creating tranceivers", err)
		return
	}

	subscriberPc.OnICECandidate(func(candidate *webrtc.ICECandidate) {

		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateJSON := candidate.ToJSON()

		// Create a object to send to the frontend
		msg := models.ICECandidateMessage{
			Type:         "subscriber-ice-candidate",
			ICECandidate: candidateJSON,
		}

		// Emit a socket event for frontend to catch this ice candidate
		client.SafeSend(msg)
	})

	subscriberPc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("[SFU] Subscriber Connection state is:", state)
		//implement cleanup and re connection logic here
	})

	subscriberPc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Println("[SFU] Subscriber ICE state is:", state)
	})

	subscriberPc.OnNegotiationNeeded(func() {
		log.Println("NEGOTIATION NEEDED")
	})

	// Create subscriber offer.
	subscriberOffer, err := subscriberPc.CreateOffer(nil)
	if err != nil {
		log.Println("Error creating Offer for subscriber PC : ", err)
		return
	}

	// Set local descriptionn.
	err = subscriberPc.SetLocalDescription(subscriberOffer)
	if err != nil {
		log.Println("Error setting subscriber offer as local description : ", err)
		return
	}

	// Create the ws message.
	subscriberOfferMsg := &models.SubscriberOfferMessage{
		Type: "subscriber-offer",
		SDP:  subscriberOffer,
	}

	// Send the ws message.
	client.SafeSend(subscriberOfferMsg)
}

func (s *SFU) RenegotiateSubscriberOffer(client *models.Client) {
	// Get the subscriber pc.
	subscriberPc := client.Subscriber.PC

	// Create the new Offer(SDP).
	reoffer, err := subscriberPc.CreateOffer(nil)
	if err != nil {
		log.Printf("Error creating re negotiation offer : %w", err)
		return
	}

	// Set local description.
	err = subscriberPc.SetLocalDescription(reoffer)
	if err != nil {
		log.Printf("Error setting the renegotiated subscriber offer as local description")
		return
	}

	// Create the ws message.
	message := models.SubscriberOfferMessage{
		Type: "subscriber-offer",
		SDP:  reoffer,
	}

	// Send the ws event.
	client.SafeSend(message)
}

func (s *SFU) RenegotiateSubscriberAnswer(answer *models.SubscriberAnswerMessage, client *models.Client) {
	reanswer := answer.SDP
	subscriberPc := client.Subscriber.PC

	// Set the new answer as remote desc.
	err := subscriberPc.SetRemoteDescription(reanswer)
	if err != nil {
		log.Printf("Error setting the renegotiated subscriber answer as remote description")
		return
	}

	// Set the new transceivers as slots.
	s.SetTranceiversAsSlots(client)
}
