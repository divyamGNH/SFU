package models

import (
	"github.com/pion/webrtc/v3"
)

type BaseMessage struct {
	Type string `json:"type"`
}

type SignalMessage struct {
	Type string                    `json:"type"`
	SDP  webrtc.SessionDescription `json:"sdp"`
}

type ICECandidateMessage struct {
	Type string `json:"type"`
	//There are 2 such types webrtc.ICECandidateInit and webrtc.ICECandidate. We use webrtc.ICECandidateInit as the other one is the complete candidate object used by pion internally we dont wanna send all that data to the frontend over sockets its redundant.
	Candidate webrtc.ICECandidateInit `json:"ICECandidate"`
}

type JoinRoomMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type CreateRoomMessage struct {
	Type   string `json:"type"`
	UserId string `json:"userId"`
}

type LeaveRoomMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type ErrorMessage struct {
	Type       string `json:"type"`
	Error      string `json:"error"`
	StatusCode string `json:"statusCode`
}

type JoinRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type CreateRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}
