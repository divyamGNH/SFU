package service

import (
	"backend/logger"
	"backend/room"
	control "proto-contracts"

	"github.com/pion/webrtc/v3"
)

// Implemented by the signalling package.
type MessageSender interface {
	SendMessageToIris(msg *control.Message) error
}

type Service struct {
	msgSender   MessageSender
	roomHandler *room.RoomHandler
}

// Create a new service without the message sender.
func NewService(roomHandler *room.RoomHandler) *Service {
	return &Service{
		roomHandler: roomHandler,
	}
}

func (s *Service) SetUpRoomHandlerCallbacks() {
	callbacks := &room.RoomCallbacks{
		OnMediaPublished:           s.OnMediaPublished,
		SendPublisherICECandidate:  s.SendPublisherICECandidate,
		SendSubscriberICECandidate: s.SendSubscriberICECandidate,
		SendSubscriberOffer:        s.SendSubscriberOffer,
	}

	s.roomHandler.SetCallbacks(*callbacks)
}

// HELPER FUNCTIONS

// Turn control.ICECandidate to webrtc.ICECandidateInit.
func MapProtoCandidateToWebRTC(protoCandidate *control.ICECandidate) webrtc.ICECandidateInit {
	candidateInit := webrtc.ICECandidateInit{
		Candidate: protoCandidate.Candidate,
	}

	if protoCandidate.SdpMid != "" {
		sdpMid := protoCandidate.SdpMid
		candidateInit.SDPMid = &sdpMid
	}
	if protoCandidate.SdpMlineIndex != 0 {
		mLineIndex := uint16(protoCandidate.SdpMlineIndex)
		candidateInit.SDPMLineIndex = &mLineIndex
	}

	return candidateInit
}

// Turn webrtc.ICECandidateInit to control.ICECandidate.
func MapWebRTCCandidateToProto(candidate webrtc.ICECandidateInit) *control.ICECandidate {
	protoCandidate := &control.ICECandidate{
		Candidate: candidate.Candidate,
	}

	if candidate.SDPMid != nil {
		protoCandidate.SdpMid = *candidate.SDPMid
	}

	if candidate.SDPMLineIndex != nil {
		protoCandidate.SdpMlineIndex = int32(*candidate.SDPMLineIndex)
	}

	return protoCandidate
}

// We first make the struct with a nil message sender then create a sender in main.go then attach that explictly in main.go only.
func (s *Service) SetMessageSender(msgSender MessageSender) {
	s.msgSender = msgSender
}

// RECEIVING FUNCTIONS

func (s *Service) OnJoinRoom(msg *control.JoinRoomRequest) {
	roomId := msg.RoomId
	clientId := msg.ClientId

	// Call the respective functions.
	err := s.roomHandler.JoinRoom(roomId, clientId)
	if err != nil {
		logger.Errorf("Error joining room : %v for client : %v", roomId, clientId)

		// Send a false ack to notify Iris.
		s.SendJoinRoomAck(roomId, clientId, false, err.Error())
		return
	}

	// Send a successfull ack to notify Iris.
	s.SendJoinRoomAck(roomId, clientId, true, "")
}

func (s *Service) OnLeaveRoom(msg *control.LeaveRoomRequest) {
	roomId := msg.RoomId
	clientId := msg.ClientId

	// Call the respective functions.
	err := s.roomHandler.LeaveRoom(roomId, clientId)

	if err != nil {
		logger.Errorf("Error leaving the room : %v by client : %v", roomId, clientId)

		// Send a false ack to notify there was some error in leaving the room.
		s.SendLeaveRoomAck(roomId, clientId, false, err.Error())
		return
	}

	// Send a success ack to notify Iris.
	s.SendLeaveRoomAck(roomId, clientId, true, "")
}

func (s *Service) OnPublisherICECandidate(msg *control.ICECandidatePayload) {
	clientId := msg.ClientId

	// Convert the control.IceCandidate to webrtc.IceCandidate
	candidateInit := MapProtoCandidateToWebRTC(msg.Candidate)

	s.roomHandler.HandlePublisherICECandidate(clientId, candidateInit)
}

func (s *Service) OnSubscriberICECandidate(msg *control.ICECandidatePayload) {
	clientId := msg.ClientId

	// Convert the control.IceCandidate to webrtc.IceCandidate
	candidateInit := MapProtoCandidateToWebRTC(msg.Candidate)

	s.roomHandler.HandleSubscriberICECandidate(clientId, candidateInit)
}

func (s *Service) OnPublisherOffer(msg *control.PublisherOffer) {
	clientId := msg.ClientId
	roomId := msg.RoomId
	requestId := msg.RequestId
	sdpString := msg.Sdp.Sdp

	// The answer message we send now acts as the answer as well the ack for the offer itself if the offer failed we send a false in the success bool acting as the ack.
	answer, err := s.roomHandler.HandleOffer(clientId, sdpString)
	if err != nil {
		logger.Errorf("Failed to handle publisher offer: %v", err)

		// Send a failed ack to Iris.
		s.SendPublisherAnswer(requestId, roomId, clientId, false, err.Error(), webrtc.SessionDescription{})
		return
	}

	// Send the answer with a successful ack to Iris.
	s.SendPublisherAnswer(requestId, roomId, clientId, true, "", answer)
}

func (s *Service) OnSubscriberAnswer(msg *control.SubscriberAnswer) {
	roomId := msg.RoomId
	clientId := msg.ClientId
	sdpString := msg.Sdp.Sdp

	err := s.roomHandler.HandleAnswer(clientId, sdpString)
	if err != nil {
		logger.Errorf("Failed to handle subscriber answer for room %v: %v", roomId, err)
		s.SendSubscriberAnswerAck(roomId, clientId, false, err.Error())
		return
	}

	s.SendSubscriberAnswerAck(roomId, clientId, true, "")
}

// SENDING FUNCTIONS

func (s *Service) SendSubscriberOffer(roomId string, clientId string, offer webrtc.SessionDescription) {
	msg := &control.Message{
		Payload: &control.Message_SubscriberOffer{
			SubscriberOffer: &control.SubscriberOffer{
				RoomId:   roomId,
				ClientId: clientId,
				Sdp: &control.SDPPayload{
					Type: offer.Type.String(),
					Sdp:  offer.SDP,
				},
			},
		},
	}

	s.msgSender.SendMessageToIris(msg)
}

func (s *Service) SendPublisherAnswer(requestId, roomId, clientId string, success bool, errorMsg string, answer webrtc.SessionDescription) {
	responseMsg := &control.Message{
		Payload: &control.Message_PublisherAnswer{
			PublisherAnswer: &control.PublisherAnswer{
				RequestId:    requestId,
				RoomId:       roomId,
				ClientId:     clientId,
				Success:      success,
				ErrorMessage: errorMsg,
				Sdp: &control.SDPPayload{
					Type: answer.Type.String(),
					Sdp:  answer.SDP,
				},
			},
		},
	}

	s.msgSender.SendMessageToIris(responseMsg)
}

func (s *Service) SendSubscriberAnswerAck(roomId, clientId string, success bool, errorMsg string) {
	msg := &control.Message{
		Payload: &control.Message_SubscriberAnswerAck{
			SubscriberAnswerAck: &control.SubscriberAnswerAck{
				RoomId:       roomId,
				ClientId:     clientId,
				Success:      success,
				ErrorMessage: errorMsg,
			},
		},
	}

	s.msgSender.SendMessageToIris(msg)
}

func (s *Service) SendPublisherICECandidate(roomId string, clientId string, candidate webrtc.ICECandidateInit) {
	msg := &control.Message{
		Payload: &control.Message_SfuPublisherIce{
			SfuPublisherIce: &control.ICECandidatePayload{
				RoomId:    roomId,
				ClientId:  clientId,
				Candidate: MapWebRTCCandidateToProto(candidate),
			},
		},
	}

	s.msgSender.SendMessageToIris(msg)
}

func (s *Service) SendSubscriberICECandidate(roomId string, clientId string, candidate webrtc.ICECandidateInit) {
	msg := &control.Message{
		Payload: &control.Message_SfuSubscriberIce{
			SfuSubscriberIce: &control.ICECandidatePayload{
				RoomId:    roomId,
				ClientId:  clientId,
				Candidate: MapWebRTCCandidateToProto(candidate),
			},
		},
	}

	s.msgSender.SendMessageToIris(msg)
}

func (s *Service) SendLeaveRoomAck(roomId string, userId string, flag bool, errorMessage string) {
	msg := &control.Message{
		Payload: &control.Message_LeaveRoomAck{
			LeaveRoomAck: &control.LeaveRoomAck{
				RoomId:       roomId,
				ClientId:     userId,
				Success:      flag,
				ErrorMessage: errorMessage,
			},
		},
	}

	err := s.msgSender.SendMessageToIris(msg)
	if err != nil {
		logger.Errorf("Error sending leaveRoomAck to Iris for room : %v and client : %v", roomId, userId)
	}
}

func (s *Service) SendJoinRoomAck(roomId string, userId string, flag bool, errorMessage string) {
	msg := &control.Message{
		Payload: &control.Message_JoinRoomAck{
			JoinRoomAck: &control.JoinRoomAck{
				RoomId:       roomId,
				ClientId:     userId,
				Success:      flag,
				ErrorMessage: errorMessage,
			},
		},
	}

	err := s.msgSender.SendMessageToIris(msg)
	if err != nil {
		logger.Errorf("Error sending leaveRoomAck to Iris for room : %v and client : %v", roomId, userId)
	}
}

func (s *Service) OnMediaPublished(clientId string, mid string, publisherId string) {
	roomId, ok := s.roomHandler.RoomIdForUser(clientId)
	if !ok {
		logger.Errorf("Could not find room for client with clientId : %v", clientId)
		return
	}

	msg := &control.Message{
		Payload: &control.Message_MediaPublished{
			MediaPublished: &control.MediaPublishedEvent{
				RoomId:      roomId,
				ClientId:    clientId,
				Mid:         mid,
				PublisherId: publisherId,
			},
		},
	}

	err := s.msgSender.SendMessageToIris(msg)
	if err != nil {
		logger.Error("Failed to send mediaPublished event : ", err)
	}
}
