package config

import "github.com/pion/webrtc/v3"

func FetchICEServers() []webrtc.ICEServer {
	return []webrtc.ICEServer{
		{
			URLs: []string{
				"stun:stun.l.google.com:19302",
			},
		},
	}
}
