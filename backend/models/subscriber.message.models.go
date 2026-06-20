package models

import "github.com/pion/webrtc/v3"

type SubscriberOfferMessage struct {
	Type string                    `json:"type"`
	SDP  webrtc.SessionDescription `json:"sdp"`
}

type SubscriberAnswerMessage struct {
	Type string                    `json:"type"`
	SDP  webrtc.SessionDescription `json:"sdp"`
}

type SubscriberICECandidateMessage struct {
	Type         string                  `json:"type"`
	ICECandidate webrtc.ICECandidateInit `json:"iceCandidate"`
}
