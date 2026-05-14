package sfu

import (
	"backend/models"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type SFU struct {
	ConnToClient map[*websocket.Conn]*models.Client
	rm           RoomManager
	mu           sync.RWMutex
}

func NewSFU(roomManager RoomManager) *SFU {

	log.Println("[SFU] Creating new SFU instance")

	return &SFU{
		ConnToClient: make(map[*websocket.Conn]*models.Client),
		rm:           roomManager,
	}
}

func (s *SFU) DrainRTCP(sender *webrtc.RTPSender) {
	// Create a new go routine so this is basically a readPump
	go func() {
		rtcpBuf := make([]byte, 256)

		//We are just reading this not using any of the RTCP packet actually just draining so the buffer does not crash the code.
		for {
			_, _, err := sender.Read(rtcpBuf)
			if err != nil {
				log.Println("[SFU] error in reading RTCP packet sender closed:", err)
				return
			}
		}
	}()
}

func (s *SFU) HandleOffer(signal models.SignalMessage, conn *websocket.Conn, client *models.Client) {

	log.Println("[HandleOffer] Received offer")

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

	log.Println("[HandleOffer] PeerConnection created successfully")

	// Earlier approach - Create the client object
	// We already get the client from the WS handler now

	// Add the PC to the client received
	client.PC = pc

	log.Println("[HandleOffer] Client object created")

	go client.WritePump()

	log.Println("[HandleOffer] WritePump started")

	//Set up pc events onTrack, onICECandidates, onConnectionStateChange
	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

		log.Println("[OnTrack] Received new media track")

		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			remoteTrack.Codec().RTPCodecCapability,
			remoteTrack.ID(),
			remoteTrack.StreamID(),
		)
		if err != nil {
			log.Println("[OnTrack] Error creating localTrack:", err)
			return
		}

		// var otherPeers []*models.Client
		otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
		if !ok {
			log.Println("[SFU] Error getting the other peers in the room")
			return
		}

		// TODO : Use RTCP packets for various things like bitrate etc instead of just draining them.
		for _, peer := range otherPeers {
			sender, err := peer.PC.AddTrack(localTrack)
			if err != nil {
				log.Println("Error adding track:", err)
				continue
			}

			s.DrainRTCP(sender)
		}

		//Manually handle each rtp packet
		//Can change and monitor everything such as codecs etc.
		for {

			//We are using ReadRTP here but another approach is using Read() with a buffer.
			packet, _, err := remoteTrack.ReadRTP()

			if err != nil {
				log.Println("[OnTrack] Error reading RTP packet:", err)
				break
			}

			//send each packet to through the pipeline made for media transfer using AddTrack
			err = localTrack.WriteRTP(packet)
			if err != nil {
				log.Println("[OnTrack] Error forwarding RTP packet:", err)
				break
			}
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("[Connection]", state)
		//implement cleanup and re connection logic here
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Println("[ICE]", state)
	})

	//Set up the received remote SDP
	//trickle ice is started as soon i set any description local or remote
	err = pc.SetRemoteDescription(signal.SDP)
	if err != nil {
		log.Println("[HandleOffer] Error setting remote description:", err)
		pc.Close()
		return
	}

	log.Println("[HandleOffer] Remote description set successfully")

	//Create answer
	localSDP, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println("[HandleOffer] Error creating answer:", err)
		pc.Close()
		return
	}

	log.Println("[HandleOffer] Answer created successfully")

	answer := models.SignalMessage{
		Type: "answer",
		SDP:  localSDP,
	}

	//Set up local description
	err = pc.SetLocalDescription(localSDP)
	if err != nil {
		log.Println("[HandleOffer] Error setting local description:", err)
		pc.Close()
		return
	}

	log.Println("[HandleOffer] Local description set successfully")

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {

		if candidate == nil {
			return
		}

		//Convert webrtc.ICECandidate to webrtc.ICECandidateInit
		candidateJSON := candidate.ToJSON()

		//Create a object to send to the frontend
		msg := models.ICECandidateMessage{
			Type:      "ice-candidate",
			Candidate: candidateJSON,
		}

		//Emit a socket event for frontend to catch this ice candidate
		client.Send <- msg
	})

	s.mu.Lock()
	s.ConnToClient[conn] = client
	s.mu.Unlock()

	log.Println("[HandleOffer] Client added to ConnToClient map")

	client.Send <- answer

	log.Println("[HandleOffer] Answer pushed into WritePump channel")

	log.Println("[HandleOffer] Offer handling completed successfully")
}

// Implement a queue to prevent drop of ice candidates as they might arrive before or after the setDescription
func (s *SFU) HandleICECandidate(candidate models.ICECandidateMessage, conn *websocket.Conn) {

	//candidate is a object that containes Candidate
	//peer is of type Client from the models file

	s.mu.RLock()
	peer := s.ConnToClient[conn]
	s.mu.RUnlock()

	if peer == nil {
		log.Println("[HandleICECandidate] Peer not found")
		return
	}

	err := peer.PC.AddICECandidate(candidate.Candidate)
	if err != nil {
		log.Println("[HandleICECandidate] Error adding ICE candidate:", err)
		return
	}
}
