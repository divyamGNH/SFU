package sfu

import (
	"fmt"

	"github.com/pion/webrtc/v3"
)

// create n tranceivers for audio and video each.
func (s *SFU) AddTransceivers(subscriberPc *webrtc.PeerConnection, n int) ([]*webrtc.RTPTransceiver, error) {
	out := make([]*webrtc.RTPTransceiver, 0, n*2)
	for i := 0; i < n; i++ {
		videoT, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeVideo,
			webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly},
		)

		if err != nil {
			return out, err // partial success — Grow decides how to handle
		}

		out = append(out, videoT)

		audioT, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeAudio,
			webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly},
		)

		if err != nil {
			return out, err
		}

		out = append(out, audioT)
	}

	return out, nil
}

// create one tranceiver for audio and video each.
func (s *SFU) AddTransceiver(pc *webrtc.PeerConnection, kind webrtc.MediaKind) (*webrtc.RTPTransceiver, error) {

	var codec webrtc.RTPCodecType

	// Decide video or audio
	switch kind {
	case webrtc.MediaKindVideo:
		codec = webrtc.RTPCodecTypeVideo
	case webrtc.MediaKindAudio:
		codec = webrtc.RTPCodecTypeAudio
	default:
		return nil, fmt.Errorf("unsupported media kind: %v", kind)
	}

	// Create the transceiver.
	return pc.AddTransceiverFromKind(
		codec,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		},
	)
}

// func (s *SFU) SetTranceiversAsSlots(client *models.Client) {

// 	client.Mu.RLock()
// 	transceivers := client.Subscriber.PendingTransceiver
// 	client.Mu.RUnlock()

// 	log.Println("Pending transceivers:", len(client.Subscriber.PendingTransceiver))
// 	// Put each transceiver in a slot after the negotiation as the Mids are not stable before that which can cause problems.
// 	for _, t := range transceivers {

// 		// If the tranceiver is not send only skip it.
// 		if t.Direction() != webrtc.RTPTransceiverDirectionSendonly {
// 			continue
// 		}

// 		log.Println(
// 			"TRANSCEIVER:",
// 			t.Mid(),
// 			t.Sender() != nil,
// 			t.Direction(),
// 			t.Kind(),
// 		)

// 		switch t.Kind() {

// 		case webrtc.RTPCodecTypeVideo:
// 			client.Subscriber.Mu.Lock()
// 			client.Subscriber.VideoSlots = append(client.Subscriber.VideoSlots, &models.MediaSlot{
// 				Transceiver: t,
// 				Occupied:    false,
// 				Kind:        webrtc.RTPCodecTypeVideo,
// 			})
// 			client.Subscriber.Mu.Unlock()
// 			log.Println("Added one to the VS")

// 		case webrtc.RTPCodecTypeAudio:
// 			client.Subscriber.Mu.Lock()
// 			client.Subscriber.AudioSlots = append(client.Subscriber.AudioSlots, &models.MediaSlot{
// 				Transceiver: t,
// 				Occupied:    false,
// 				Kind:        webrtc.RTPCodecTypeAudio,
// 			})
// 			client.Subscriber.Mu.Unlock()
// 			log.Println("Added one to the AS")
// 		}
// 	}

// 	client.Subscriber.PendingTransceiver = nil
// }
