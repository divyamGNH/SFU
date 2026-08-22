package service

import (
	"backend/logger"
	"backend/room"
	control "proto-contracts"

	"github.com/pion/webrtc/v3"
)

type MessageSender interface {
	SendMessageToIris(msg *control.Message) error
}

type Service struct {
	msgSender   MessageSender
	roomHandler *room.RoomHandler
}

func NewService(msgSender MessageSender, roomHandler *room.RoomHandler) *Service {
	return &Service{
		msgSender:   msgSender,
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

func (s *Service) OnPublisherICECandidate(msg *control.IceCandidateRequest) {

}

func (s *Service) OnSubscriberICECandidate(msg *control.IceCandidateRequest) {

}

func (s *Service) OnPublisherOffer(msg *control.PublisherOfferRequest) {

}

func (s *Service) OnSubscriberAnswer(msg *control.SubscriberAnswerRequest) {

}

// SENDING FUNCTIONS

func (s *Service) SendSubscriberOffer(clientId string, offer webrtc.SessionDescription) {

}

func (s *Service) SendPublisherAnswer() {

}

func (s *Service) SendPublisherICECandidate(clientId string, candidate webrtc.ICECandidateInit) {

}

func (s *Service) SendSubscriberICECandidate(clientId string, candidate webrtc.ICECandidateInit) {

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
