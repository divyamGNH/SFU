// TODO : i am chasing right now is that a new user wont trigger a re negotitation what u are saying is that on a forceful renegotitation we use the same PC saving us trouble for transceiver making again ?

package sfu

import (
	"backend/models"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type SFU struct {
	ConnToClient            map[*websocket.Conn]*models.Client
	UserIdToPublishedTracks map[string][]*models.PublishedTrack
	rm                      RoomManager
	mu                      sync.RWMutex
}

func NewSFU(roomManager RoomManager) *SFU {
	return &SFU{
		ConnToClient:            make(map[*websocket.Conn]*models.Client),
		UserIdToPublishedTracks: make(map[string][]*models.PublishedTrack),
		rm:                      roomManager,
	}
}

// Send the PLI upstream to the owner of the track.
func (s *SFU) SendPLIToPublisher(publishedTrack *models.PublishedTrack) {
	publisherClient, ok := s.rm.GetClientFromUserId(publishedTrack.PublisherID)
	if !ok {
		log.Println("[SFU] publisher client not found")
		return
	}

	if publisherClient.SFUPeer == nil || publisherClient.SFUPeer.PC == nil {
		log.Println("[SFU] publisher PC missing")
		return
	}

	err := publisherClient.SFUPeer.PC.WriteRTCP(
		[]rtcp.Packet{
			&rtcp.PictureLossIndication{
				MediaSSRC: uint32(
					publishedTrack.SSRC,
				),
			},
		},
	)

	if err != nil {
		log.Println("Error sending PLI upstream")
		return
	}

	log.Println("Succesfully sent PLI upstream")
}

// Drain and read RTCP packets to get various informations like PLI, NACK, FIR etc.
func (s *SFU) DrainRTCP(slot *models.MediaSlot, publishedTrack *models.PublishedTrack) {
	// Create a new go routine so this is basically a readPump
	log.Println("inside drain rtcp")
	slot.Mu.Lock()
	if slot.DrainRTCPStarted {
		slot.Mu.Unlock()
		return
	}
	slot.DrainRTCPStarted = true
	slot.Mu.Unlock()
	sender := slot.Transceiver.Sender()

	go func() {
		rtcpBuf := make([]byte, 256)

		//We are just reading this not using any of the RTCP packet actually just draining so the buffer does not crash the code.
		for {
			n, _, err := sender.Read(rtcpBuf)
			if err != nil {
				log.Println("[SFU] error in reading RTCP packet sender closed:", err)
				return
			}

			packets, err := rtcp.Unmarshal(rtcpBuf[:n])
			if err != nil {
				log.Println("[SFU] error in unmarshalling the rtcp buffer into packets", err)
				return
			}

			for _, packet := range packets {
				switch packet.(type) {
				case *rtcp.PictureLossIndication:
					log.Println("[SFU] PLI received.")
					s.SendPLIToPublisher(publishedTrack)

				case *rtcp.FullIntraRequest:
					log.Println("[SFU] FIR received")
					s.SendPLIToPublisher(publishedTrack)

				case *rtcp.TransportLayerNack:
					log.Println("[SFU] Transport layer NACK received")

				case *rtcp.ReceiverReport:
					log.Println("[SFU] Receviver Report received")
				}
			}
		}
	}()
}

func (s *SFU) FlushICECandidateQueue(client *models.Client) {
	client.SFUPeer.Mu.Lock()

	if !client.SFUPeer.RemoteDescSet {
		client.SFUPeer.Mu.Unlock()

		log.Println("[SFU] Remote Description not set yet. Cannot flush the ICE Candidate queue.")
		return
	}

	// candidate is already of type ICECandidateInit
	candidates := append([]models.ICECandidateMessage(nil), client.SFUPeer.PendingCandidates...)

	//Empty the queue.
	client.SFUPeer.PendingCandidates = nil

	client.SFUPeer.Mu.Unlock()

	for _, candidate := range candidates {
		//Add the Ice candidate to the queue and wait for the remote description to set.
		err := client.SFUPeer.PC.AddICECandidate(candidate.ICECandidate)
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

func IsTrackAlreadyPublished(slots []*models.MediaSlot, publishedTrack *models.PublishedTrack) bool {

	for _, slot := range slots {

		slot.Mu.Lock()

		alreadyPublished :=
			slot.Occupied &&
				slot.PublisherId ==
					publishedTrack.PublisherID &&
				slot.TrackID ==
					publishedTrack.TrackID

		slot.Mu.Unlock()

		if alreadyPublished {
			return true
		}
	}

	return false
}

// Send media to a single client specified in the function.
func (s *SFU) PublishVideoStream(client *models.Client, publishedTrack *models.PublishedTrack) {

	log.Printf(
		"[SFU][VIDEO] Publish requested subscriber=%v publisher=%v trackID=%v",
		client.UserId,
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
	)

	if client.Subscriber == nil {
		log.Printf(
			"[SFU][VIDEO] Subscriber PC is nil subscriber=%v",
			client.UserId,
		)
		return
	}

	if !client.Subscriber.RemoteDescSet {
		log.Printf(
			"[SFU][VIDEO] Remote description not set subscriber=%v",
			client.UserId,
		)
		return
	}

	if IsTrackAlreadyPublished(client.Subscriber.VideoSlots, publishedTrack) {

		log.Printf(
			"[SFU][VIDEO] Track already published subscriber=%v publisher=%v trackID=%v",
			client.UserId,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
		)

		return
	}

	log.Printf(
		"[SFU][VIDEO] Searching for free slot subscriber=%v totalSlots=%d",
		client.UserId,
		len(client.Subscriber.VideoSlots),
	)

	for index, slot := range client.Subscriber.VideoSlots {

		slot.Mu.Lock()

		mid := slot.Transceiver.Mid()

		if mid == "" {
			log.Println("[SFU] MID is empty for video")
			slot.Mu.Unlock()
			continue
		}

		log.Printf(
			"[SFU][VIDEO] Checking slot=%d MID=%v occupied=%v",
			index,
			mid,
			slot.Occupied,
		)

		if slot.Occupied {

			log.Printf(
				"[SFU][VIDEO] Slot already occupied slot=%d MID=%v publisher=%v trackID=%v",
				index,
				mid,
				slot.PublisherId,
				slot.TrackID,
			)

			slot.Mu.Unlock()
			continue
		}

		sender := slot.Transceiver.Sender()

		if sender == nil {

			log.Printf(
				"[SFU][VIDEO] Sender is nil slot=%d MID=%v",
				index,
				mid,
			)

			slot.Mu.Unlock()
			continue
		}

		log.Printf(
			"[SFU][VIDEO] Replacing track slot=%d MID=%v publisher=%v trackID=%v",
			index,
			mid,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
		)

		log.Printf(
			"[SFU][VIDEO] ReplaceTrack success slot=%d MID=%v",
			index,
			mid,
		)

		log.Printf(
			"[SFU][VIDEO] Starting RTCP drain slot=%d MID=%v",
			index,
			mid,
		)

		slot.Occupied = true
		slot.PublisherId = publishedTrack.PublisherID
		slot.TrackID = publishedTrack.TrackID

		log.Println("Video Slot secured succesfully")

		slot.Mu.Unlock()

		// TODO : currently the order is not right we mark the slots as taken before replace track succeeds fix this order.
		err := sender.ReplaceTrack(publishedTrack.LocalTrack)
		s.SendPLIToPublisher(publishedTrack)

		if err != nil {
			log.Println("[SFU][VIDEO] ReplaceTrack error:", err)
			continue
		}

		s.DrainRTCP(slot, publishedTrack)
		log.Println("DrainRTCP done for video")

		client.Mu.Lock()
		client.MidToPublisher[mid] = publishedTrack.PublisherID
		client.Mu.Unlock()

		log.Printf(
			"[SFU][VIDEO] MID mapping added MID=%v publisher=%v",
			mid,
			publishedTrack.PublisherID,
		)

		log.Printf(
			"[SFU][VIDEO] Sending media-published event MID=%v publisher=%v subscriber=%v",
			mid,
			publishedTrack.PublisherID,
			client.UserId,
		)

		msg := models.PublishMediaMessage{
			Type:      "media-published",
			Mid:       mid,
			Publisher: publishedTrack.PublisherID,
		}

		log.Println("media-published message made for video")

		client.Send <- msg

		log.Printf(
			"[SFU][VIDEO] Successfully published VIDEO track subscriber=%v publisher=%v trackID=%v MID=%v",
			client.UserId,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
			mid,
		)

		return
	}

	log.Printf(
		"[SFU][VIDEO] No free VIDEO slot found subscriber=%v publisher=%v trackID=%v",
		client.UserId,
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
	)
}

func (s *SFU) PublishAudioStream(client *models.Client, publishedTrack *models.PublishedTrack) {

	log.Printf(
		"[SFU][AUDIO] Publish requested subscriber=%v publisher=%v trackID=%v",
		client.UserId,
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
	)

	if client.Subscriber == nil {

		log.Printf(
			"[SFU][AUDIO] Subscriber PC is nil subscriber=%v",
			client.UserId,
		)

		return
	}

	if !client.Subscriber.RemoteDescSet {

		log.Printf(
			"[SFU][AUDIO] Remote description not set subscriber=%v",
			client.UserId,
		)

		return
	}

	if IsTrackAlreadyPublished(client.Subscriber.AudioSlots, publishedTrack) {

		log.Printf(
			"[SFU][AUDIO] Track already published subscriber=%v publisher=%v trackID=%v",
			client.UserId,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
		)

		return
	}

	log.Printf(
		"[SFU][AUDIO] Searching for free slot subscriber=%v totalSlots=%d",
		client.UserId,
		len(client.Subscriber.AudioSlots),
	)

	for index, slot := range client.Subscriber.AudioSlots {

		slot.Mu.Lock()

		mid := slot.Transceiver.Mid()

		if mid == "" {
			log.Println("[SFU] MID is empty for video")
			slot.Mu.Unlock()
			continue
		}

		log.Printf(
			"[SFU][AUDIO] Checking slot=%d MID=%v occupied=%v",
			index,
			mid,
			slot.Occupied,
		)

		if slot.Occupied {

			log.Printf(
				"[SFU][AUDIO] Slot already occupied slot=%d MID=%v publisher=%v trackID=%v",
				index,
				mid,
				slot.PublisherId,
				slot.TrackID,
			)

			slot.Mu.Unlock()
			continue
		}

		sender := slot.Transceiver.Sender()

		if sender == nil {

			log.Printf(
				"[SFU][AUDIO] Sender is nil slot=%d MID=%v",
				index,
				mid,
			)

			slot.Mu.Unlock()
			continue
		}

		log.Printf(
			"[SFU][AUDIO] Replacing track slot=%d MID=%v publisher=%v trackID=%v",
			index,
			mid,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
		)

		err := sender.ReplaceTrack(
			publishedTrack.LocalTrack,
		)

		if err != nil {

			log.Println(
				"[SFU][AUDIO] ReplaceTrack error:",
				err,
			)

			slot.Mu.Unlock()
			continue
		}

		log.Printf(
			"[SFU][AUDIO] ReplaceTrack success slot=%d MID=%v",
			index,
			mid,
		)

		log.Printf(
			"[SFU][AUDIO] Starting RTCP drain slot=%d MID=%v",
			index,
			mid,
		)

		slot.Occupied = true
		slot.PublisherId = publishedTrack.PublisherID
		slot.TrackID = publishedTrack.TrackID

		log.Println("Audio slot secured succesfully")

		slot.Mu.Unlock()

		s.DrainRTCP(slot, publishedTrack)
		log.Println("Outisde drain rtcp audio")

		client.Mu.Lock()
		client.MidToPublisher[mid] = publishedTrack.PublisherID
		client.Mu.Unlock()

		log.Printf(
			"[SFU][AUDIO] MID mapping added MID=%v publisher=%v",
			mid,
			publishedTrack.PublisherID,
		)

		log.Printf(
			"[SFU][AUDIO] Sending media-published event MID=%v publisher=%v subscriber=%v",
			mid,
			publishedTrack.PublisherID,
			client.UserId,
		)

		msg := models.PublishMediaMessage{
			Type:      "media-published",
			Mid:       mid,
			Publisher: publishedTrack.PublisherID,
		}
		log.Println("media-published message made for audio")

		client.Send <- msg

		log.Printf(
			"[SFU][AUDIO] Successfully published AUDIO track subscriber=%v publisher=%v trackID=%v MID=%v",
			client.UserId,
			publishedTrack.PublisherID,
			publishedTrack.TrackID,
			mid,
		)

		return
	}

	log.Printf(
		"[SFU][AUDIO] No free AUDIO slot found subscriber=%v publisher=%v trackID=%v",
		client.UserId,
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
	)
}

func (s *SFU) SendLocalMediaToRemotePeers(client *models.Client) {
	otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		log.Println("[SFU] Error getting the other peers in the room")
		return
	}

	s.mu.RLock()
	localTracks := append([]*models.PublishedTrack(nil), s.UserIdToPublishedTracks[client.UserId]...)
	s.mu.RUnlock()
	for _, peer := range otherPeers {
		for _, localtrack := range localTracks {
			switch localtrack.Kind {
			case webrtc.RTPCodecTypeVideo:
				s.PublishVideoStream(peer, localtrack)

			case webrtc.RTPCodecTypeAudio:
				s.PublishAudioStream(peer, localtrack)
			}
		}
	}
}

func (s *SFU) SendRemoteMediaToLocalPeer(client *models.Client) {
	otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		log.Println("[SFU] Error getting the other peers in the room")
		return
	}

	for _, peer := range otherPeers {
		tracks := s.UserIdToPublishedTracks[peer.UserId]

		for _, publishedTrack := range tracks {

			switch publishedTrack.Kind {

			case webrtc.RTPCodecTypeVideo:
				s.PublishVideoStream(client, publishedTrack)

			case webrtc.RTPCodecTypeAudio:
				s.PublishAudioStream(client, publishedTrack)
			}
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
	sfuPeer := &models.SFUPeer{
		PC:                pc,
		RemoteDescSet:     false,
		PendingCandidates: make([]models.ICECandidateMessage, 0, 256),
	}
	client.SFUPeer = sfuPeer

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
	client.SFUPeer.Mu.Lock()
	client.SFUPeer.RemoteDescSet = true
	client.SFUPeer.Mu.Unlock()

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
		client.Send <- msg
	})

	s.mu.Lock()
	s.ConnToClient[conn] = client
	s.mu.Unlock()

	// log.Println("[SFU] Client added to ConnToClient map")

	client.Send <- answer

	s.SendSubscriberOffer(client)

	log.Println("[SFU] Answer pushed into WritePump channel")
	log.Println("[SFU] Offer handling completed successfully")
}

// Implement a queue to prevent drop of ice candidates as they might arrive before or after the setDescription
func (s *SFU) HandleICECandidate(candidate models.ICECandidateMessage, client *models.Client) {

	//candidate is a object that containes Candidate
	client.SFUPeer.Mu.Lock()

	if !client.SFUPeer.RemoteDescSet {
		client.SFUPeer.PendingCandidates = append(client.SFUPeer.PendingCandidates, candidate)
		client.SFUPeer.Mu.Unlock()
		return
	}
	client.SFUPeer.Mu.Unlock()

	err := client.SFUPeer.PC.AddICECandidate(candidate.ICECandidate)
	if err != nil {
		log.Println("[SFU] Error adding ICE candidate to the queue:", err)
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
	for i := 0; i < 10; i++ {
		_, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeVideo,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making video tranceivers", err)
			return
		}

		_, err = subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeAudio,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making audio tranceivers", err)
			return
		}
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
		client.Send <- msg
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
	client.Send <- subscriberOfferMsg
}

func (s *SFU) HandleSubscriberAnswer(answer models.SubscriberAnswerMessage, client *models.Client) {
	remoteSDP := answer.SDP

	subscriberPc := client.Subscriber.PC

	err := subscriberPc.SetRemoteDescription(remoteSDP)
	if err != nil {
		log.Println("Error setting the remote description for subscriber PC : ", err)
	}

	client.Subscriber.Mu.Lock()
	client.Subscriber.RemoteDescSet = true
	client.Subscriber.Mu.Unlock()

	transceivers := subscriberPc.GetTransceivers()
	log.Println("Got all the tranceivers")

	// Put each transceiver in a slot after the negotiation as the Mids are not stable before that which can cause problems.
	for _, t := range transceivers {

		if t.Direction() != webrtc.RTPTransceiverDirectionSendonly {
			continue
		}

		log.Println(
			"TRANSCEIVER:",
			t.Mid(),
			t.Sender() != nil,
			t.Direction(),
			t.Kind(),
		)

		switch t.Kind() {

		case webrtc.RTPCodecTypeVideo:
			client.Subscriber.Mu.Lock()
			client.Subscriber.VideoSlots = append(client.Subscriber.VideoSlots, &models.MediaSlot{
				Transceiver: t,
				Occupied:    false,
				Kind:        webrtc.RTPCodecTypeVideo,
			})
			client.Subscriber.Mu.Unlock()
			log.Println("Added one to the VS")

		case webrtc.RTPCodecTypeAudio:
			client.Subscriber.Mu.Lock()
			client.Subscriber.AudioSlots = append(client.Subscriber.AudioSlots, &models.MediaSlot{
				Transceiver: t,
				Occupied:    false,
				Kind:        webrtc.RTPCodecTypeAudio,
			})
			client.Subscriber.Mu.Unlock()
			log.Println("Added one to the AS")
		}
	}

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

// func (s *SFU) CleanupSFU(client *models.Client) {

// 	if client == nil {
// 		return
// 	}

// 	client.Mu.Lock()

// 	// prevent double cleanup
// 	if client.Closed {
// 		client.Mu.Unlock()
// 		return
// 	}

// 	client.Closed = true

// 	log.Printf(
// 		"[SFU] Cleaning up client: %s",
// 		client.UserId,
// 	)

// 	videoSlots := client.VideoSlots
// 	audioSlots := client.AudioSlots

// 	conn := client.Conn
// 	sendChan := client.Send

// 	var pc *webrtc.PeerConnection

// 	if client.SFUPeer != nil {
// 		pc = client.SFUPeer.PC
// 	}

// 	client.VideoSlots = nil
// 	client.AudioSlots = nil
// 	client.MidToPublisher = nil

// 	client.Mu.Unlock()

// 	// cleanup own video slots
// 	for _, slot := range videoSlots {

// 		if slot == nil || slot.Transceiver == nil {
// 			continue
// 		}

// 		sender := slot.Transceiver.Sender()

// 		if sender != nil {

// 			err := sender.ReplaceTrack(nil)

// 			if err != nil {
// 				log.Println(
// 					"[SFU] Error removing video track:",
// 					err,
// 				)
// 			}
// 		}

// 		slot.Occupied = false
// 		slot.PublisherId = ""
// 	}

// 	// cleanup own audio slots
// 	for _, slot := range audioSlots {

// 		if slot == nil || slot.Transceiver == nil {
// 			continue
// 		}

// 		sender := slot.Transceiver.Sender()

// 		if sender != nil {

// 			err := sender.ReplaceTrack(nil)

// 			if err != nil {
// 				log.Println(
// 					"[SFU] Error removing audio track:",
// 					err,
// 				)
// 			}
// 		}

// 		slot.Occupied = false
// 		slot.PublisherId = ""
// 	}

// 	// remove this publisher from other peers
// 	otherPeers, ok := s.rm.GetOtherPeersFromARoom(
// 		client.RoomId,
// 		client.UserId,
// 	)

// 	if ok {

// 		for _, peer := range otherPeers {

// 			if peer == nil {
// 				continue
// 			}

// 			peer.Mu.Lock()

// 			for _, slot := range peer.VideoSlots {

// 				if slot == nil || slot.Transceiver == nil {
// 					continue
// 				}

// 				if slot.PublisherId == client.UserId {

// 					sender := slot.Transceiver.Sender()

// 					if sender != nil {

// 						err := sender.ReplaceTrack(nil)

// 						if err != nil {
// 							log.Println(
// 								"[SFU] Error unsubscribing video slot:",
// 								err,
// 							)
// 						}
// 					}

// 					slot.Occupied = false

// 					mid := slot.Transceiver.Mid()

// 					delete(
// 						peer.MidToPublisher,
// 						mid,
// 					)

// 					slot.PublisherId = ""
// 				}
// 			}

// 			for _, slot := range peer.AudioSlots {

// 				if slot == nil || slot.Transceiver == nil {
// 					continue
// 				}

// 				if slot.PublisherId == client.UserId {

// 					sender := slot.Transceiver.Sender()

// 					if sender != nil {

// 						err := sender.ReplaceTrack(nil)

// 						if err != nil {
// 							log.Println(
// 								"[SFU] Error unsubscribing audio slot:",
// 								err,
// 							)
// 						}
// 					}

// 					slot.Occupied = false

// 					mid := slot.Transceiver.Mid()

// 					delete(
// 						peer.MidToPublisher,
// 						mid,
// 					)

// 					slot.PublisherId = ""
// 				}
// 			}

// 			peer.Mu.Unlock()
// 		}
// 	}

// 	// close peerconnection
// 	if pc != nil {

// 		err := pc.Close()

// 		if err != nil {
// 			log.Println(
// 				"[SFU] Error closing PeerConnection:",
// 				err,
// 			)
// 		}
// 	}

// 	// remove from global conn map
// 	if conn != nil {

// 		s.mu.Lock()
// 		delete(s.ConnToClient, conn)
// 		s.mu.Unlock()
// 	}

// 	// IMPORTANT:
// 	// close send channel BEFORE websocket
// 	// writepump exits naturally
// 	if sendChan != nil {

// 		defer func() {
// 			if recover() != nil {
// 				log.Println("[SFU] send channel already closed")
// 			}
// 		}()

// 		close(sendChan)
// 	}

// 	// close websocket
// 	if conn != nil {

// 		err := conn.Close()

// 		if err != nil {
// 			log.Println(
// 				"[SFU] Error closing websocket:",
// 				err,
// 			)
// 		}
// 	}

// 	log.Printf(
// 		"[SFU] Cleanup complete for client: %s",
// 		client.UserId,
// 	)
// }
