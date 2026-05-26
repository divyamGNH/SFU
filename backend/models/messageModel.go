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
	ICECandidate webrtc.ICECandidateInit `json:"iceCandidate"`
}

// user creates or joins the room he/she is eventually entering the room so only one event to just add the roomId to the user struct
type PopulateRoomMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
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
	StatusCode string `json:"statusCode"`
}

type JoinRoomResponse struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type JoinRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type CreateRoomSuccessMessage struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type PublishMediaMessage struct {
	Type      string `json:"type"`
	Mid       string `json:"mid"`
	Publisher string `json:"publisher"`
}
