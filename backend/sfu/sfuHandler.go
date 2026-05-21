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

		// var otherPeers []*models.Client
		otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
		if !ok {
			log.Println("[SFU] Error getting the other peers in the room")
			return
		}

		// TODO : Use RTCP packets for various things like bitrate etc instead of just draining them.
		for _, peer := range otherPeers {
			sender, err := peer.SFUPeer.PC.AddTrack(localTrack)
			if err != nil {
				log.Println("[SFU] Error adding track:", err)
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

	// log.Println("[SFU] Remote description set successfully")

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
