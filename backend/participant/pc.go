package participant

import (
	"github.com/pion/webrtc/v3"
)

type PcConfig struct {
	iceServers []webrtc.ICEServer
}

func NewPeerConnection(cfg *PcConfig) (*webrtc.PeerConnection, error) {
	return webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: cfg.iceServers,
	})
}
