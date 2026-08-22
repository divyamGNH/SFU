// THIS PACKAGE HAS BEEN DEGRADED AND SHOULD NO LONGER BE USED.
// IT IS KEPT JUST FOR THE SAKE OF BUILDING THE BROKEN BLOCKS CORRECTLY THEN THIS WILL BE REMOVED

package types

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
