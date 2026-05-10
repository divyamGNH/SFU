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
