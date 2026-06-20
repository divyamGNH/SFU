package sfu

import (
	"backend/models"
	"log"

	"github.com/pion/webrtc/v3"
)

func (s *SFU) CleanUpSFU(client *models.Client) {
	log.Printf("SFU for client with userId: %v triggered", client.UserId)

	// get client Conn and UserId.
	conn := client.Conn
	userId := client.UserId

	s.mu.Lock()
	delete(s.ConnToClient, conn)
	delete(s.UserIdToPublishedTracks, userId)
	s.mu.Unlock()

	for _, slot := range s.SubscriberToOccupiedSlots[userId] {
		switch slot.Kind {
		case webrtc.RTPCodecTypeAudio:
			err := slot.Transceiver.Sender().ReplaceTrack(nil)
			if err != nil {
				log.Printf("Error replacing track to nil while cleaning up Video Tranceiver slots for client left userId : %v", userId)
			}

			slot.Mu.Lock()
			slot.Occupied = false
			slot.PublisherId = ""
			slot.TrackID = ""
			slot.DrainRTCPStarted = false
			slot.Mu.Unlock()

		case webrtc.RTPCodecTypeVideo:
			err := slot.Transceiver.Sender().ReplaceTrack(nil)
			if err != nil {
				log.Printf("Error replacing track to nil while cleaning up Audio Tranceiver slots for client left userId : %v", userId)
			}

			slot.Mu.Lock()
			slot.Occupied = false
			slot.PublisherId = ""
			slot.TrackID = ""
			slot.DrainRTCPStarted = false
			slot.Mu.Unlock()
		}
	}

	s.mu.Lock()
	delete(s.SubscriberToOccupiedSlots, userId)
	s.mu.Unlock()

	client.CleanUpClient()
}
