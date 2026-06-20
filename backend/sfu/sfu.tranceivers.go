package sfu

import (
	"backend/models"
	"log"

	"github.com/pion/webrtc/v3"
)

// create 10 tranceivers for audio and video each.
func (s *SFU) CreateTranceivers(client *models.Client) error {
	subscriberPc := client.Subscriber.PC
	for i := 0; i < 10; i++ {
		videoT, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeVideo,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making video tranceivers", err)
			return err
		}

		// Add the video tranceiver to the array
		client.Mu.Lock()
		client.Subscriber.PendingTransceiver = append(client.Subscriber.PendingTransceiver, videoT)
		client.Mu.Unlock()

		audioT, err := subscriberPc.AddTransceiverFromKind(
			webrtc.RTPCodecTypeAudio,
			webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			},
		)

		if err != nil {
			log.Println("Error making audio tranceivers", err)
			return err
		}

		// Add the audio tranceiver to the array
		client.Mu.Lock()
		client.Subscriber.PendingTransceiver = append(client.Subscriber.PendingTransceiver, audioT)
		client.Mu.Unlock()
	}

	return nil
}

func (s *SFU) SetTranceiversAsSlots(client *models.Client) {

	client.Mu.RLock()
	transceivers := client.Subscriber.PendingTransceiver
	client.Mu.RUnlock()

	// Put each transceiver in a slot after the negotiation as the Mids are not stable before that which can cause problems.
	for _, t := range transceivers {

		// If the tranceiver is not send only skip it.
		if t.Direction() != webrtc.RTPTransceiverDirectionSendonly {
			continue
		}

		log.Println(
			"TRANSCEIVER:",
			t.Mid(),
			t.Sender() != nil,
			t.Direction(),
			t.Kind(),
		)

		switch t.Kind() {

		case webrtc.RTPCodecTypeVideo:
			client.Subscriber.Mu.Lock()
			client.Subscriber.VideoSlots = append(client.Subscriber.VideoSlots, &models.MediaSlot{
				Transceiver: t,
				Occupied:    false,
				Kind:        webrtc.RTPCodecTypeVideo,
			})
			client.Subscriber.Mu.Unlock()
			log.Println("Added one to the VS")

		case webrtc.RTPCodecTypeAudio:
			client.Subscriber.Mu.Lock()
			client.Subscriber.AudioSlots = append(client.Subscriber.AudioSlots, &models.MediaSlot{
				Transceiver: t,
				Occupied:    false,
				Kind:        webrtc.RTPCodecTypeAudio,
			})
			client.Subscriber.Mu.Unlock()
			log.Println("Added one to the AS")
		}
	}
}
