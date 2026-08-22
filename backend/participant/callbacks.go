package participant

import "github.com/pion/webrtc/v3"

type PublisherCallbacks struct {
	OnTrackPublished          func(track *PublishedTrack, client *Client)
	SendPublisherICECandidate func(client *Client, candidate webrtc.ICECandidateInit)
	// add more fields here later as new events show up — OnTrackUnpublished, etc.
}

type SubscriberCallbacks struct {
	OnNegotiationCompleted     func(client *Client)
	SendSubscriberICECandidate func(client *Client, candidate webrtc.ICECandidateInit)
	SendSubscriberOffer        func(client *Client, offer webrtc.SessionDescription)
}

type ClientCallbacks struct {
	GetIceServers func() []webrtc.ICEServer
	PubCallbacks  PublisherCallbacks
	SubCallbacks  SubscriberCallbacks
}
