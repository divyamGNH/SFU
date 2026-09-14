// THIS PACKAGE HAS BEEN DEGRADED AND SHOULD NO LONGER BE USED.
// IT IS KEPT JUST FOR THE SAKE OF BUILDING THE BROKEN BLOCKS CORRECTLY THEN THIS WILL BE REMOVED

package types

// These types exactly match the HTTP requests and responses
// that iris-backend (backend/sfu/client.go) expects.

// JoinResponse is returned by POST /room/{roomId}/{clientId}/sfu/join
type JoinResponse struct {
	RoomID   string `json:"roomId"`
	ClientID string `json:"clientId"`
	SFUUrl   string `json:"sfuUrl"` // When we have multiple sfu nodes. Ignore for now.
}

// SDPPayload wraps an SDP object (type + sdp string).
type SDPPayload struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// OfferResponse is returned by POST /room/{roomId}/{clientId}/sfu/offer
// and GET /room/{roomId}/{clientId}/sfu/pending-offer
type OfferResponse struct {
	SDP SDPPayload `json:"sdp"`
}

// ICECandidate wraps a single ICE candidate.
// Used in POST /room/{roomId}/{clientId}/sfu/ice
type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// SubscribeResponse is returned by POST /subscribe and DELETE /unsubscribe
type SubscribeResponse struct {
	Status string `json:"status"`
}
