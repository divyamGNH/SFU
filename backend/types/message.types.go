// THIS PACKAGE HAS BEEN DEGRADED AND SHOULD NO LONGER BE USED.
// IT IS KEPT JUST FOR THE SAKE OF BUILDING THE BROKEN BLOCKS CORRECTLY THEN THIS WILL BE REMOVED

package types

import (
	"github.com/pion/webrtc/v3"
)

// WS message for Signalling events.

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

// WS message for SFU events.

type PublishMediaMessage struct {
	Type      string `json:"type"`
	Mid       string `json:"mid"`
	Publisher string `json:"publisher"`
}

// WS messages for Room events.

type JoinRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomSuccessMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type PopulateRoomMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type AudioToggleMessage struct {
	Type  string `json:"type"`
	Muted bool   `json:"muted"`
}

type VideoToggleMessage struct {
	Type  string `json:"type"`
	Muted bool   `json:"muted"`
}

type AudioToggleMessageRes struct {
	Type   string `json:"type"`
	UserId string `json:"userId"`
	Muted  bool   `json:"muted"`
}

type VideoToggleMessageRes struct {
	Type   string `json:"type"`
	UserId string `json:"userId"`
	Muted  bool   `json:"muted"`
}
