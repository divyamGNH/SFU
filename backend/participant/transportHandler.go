package participant

import "github.com/pion/webrtc/v3"

type TransportHandler interface {
	OnTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, client *Client)
}
