package sfu

import (
	"log"

	"github.com/pion/webrtc/v3"
)

// create 10 tranceivers for audio and video each.
func (s *SFU) CreateTranceivers(subscriberPc *webrtc.PeerConnection) {
	for i := 0; i < 10; i++ {
		_, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeVideo,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making video tranceivers", err)
			return
		}

		_, err = subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeAudio,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making audio tranceivers", err)
			return
		}
	}

}
