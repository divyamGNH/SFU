package sfu

import (
	"backend/models"
	"log"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

// Send the PLI upstream to the owner of the track.
func (s *SFU) SendPLIToPublisher(publishedTrack *models.PublishedTrack) {
	publisherClient, ok := s.rm.GetClientFromUserId(publishedTrack.PublisherID)
	if !ok {
		log.Println("[SFU] publisher client not found")
		return
	}

	if publisherClient.Publisher == nil || publisherClient.Publisher.PC == nil {
		log.Println("[SFU] publisher PC missing")
		return
	}

	err := publisherClient.Publisher.PC.WriteRTCP(
		[]rtcp.Packet{
			&rtcp.PictureLossIndication{
				MediaSSRC: uint32(
					publishedTrack.SSRC,
				),
			},
		},
	)

	if err != nil {
		log.Println("Error sending PLI upstream")
		return
	}

	log.Println("Succesfully sent PLI upstream")
}

// Drain and read RTCP packets to get various informations like PLI, NACK, FIR etc.
func (s *SFU) DrainRTCP(slot *models.MediaSlot, publishedTrack *models.PublishedTrack) {
	// Create a new go routine so this is basically a readPump
	log.Println("inside drain rtcp")
	slot.Mu.Lock()
	if slot.DrainRTCPStarted {
		slot.Mu.Unlock()
		return
	}
	slot.DrainRTCPStarted = true
	slot.Mu.Unlock()
	sender := slot.Transceiver.Sender()

	go func() {
		rtcpBuf := make([]byte, 256)

		//We are just reading this not using any of the RTCP packet actually just draining so the buffer does not crash the code.
		for {
			n, _, err := sender.Read(rtcpBuf)
			if err != nil {
				log.Println("[SFU] error in reading RTCP packet sender closed:", err)
				return
			}

			packets, err := rtcp.Unmarshal(rtcpBuf[:n])
			if err != nil {
				log.Println("[SFU] error in unmarshalling the rtcp buffer into packets", err)
				return
			}

			for _, packet := range packets {
				switch packet.(type) {
				case *rtcp.PictureLossIndication:
					log.Println("[SFU] PLI received.")
					s.SendPLIToPublisher(publishedTrack)

				case *rtcp.FullIntraRequest:
					log.Println("[SFU] FIR received")
					s.SendPLIToPublisher(publishedTrack)

				case *rtcp.TransportLayerNack:
					log.Println("[SFU] Transport layer NACK received")

				case *rtcp.ReceiverReport:
					// log.Println("[SFU] Receviver Report received")
				}
			}
		}
	}()
}

func IsTrackAlreadyPublished(slots []*models.MediaSlot, publishedTrack *models.PublishedTrack) bool {

	for _, slot := range slots {

		slot.Mu.RLock()

		alreadyPublished := slot.Occupied && slot.PublisherId == publishedTrack.PublisherID && slot.TrackID == publishedTrack.TrackID

		slot.Mu.RUnlock()

		if alreadyPublished {
			return true
		}
	}

	return false
}

// Send media to a single client specified in the function.
func (s *SFU) PublishVideoStream(client *models.Client, publishedTrack *models.PublishedTrack) {
	if client.Subscriber == nil {
		log.Printf("[SFU] Subscriber PC for videoTrack is nil subscriber=%v", client.UserId)
		return
	}

	if !client.Subscriber.RemoteDescSet {
		log.Printf("[SFU] Remote description not set subscriber=%v", client.UserId)
		return
	}

	if IsTrackAlreadyPublished(client.Subscriber.VideoSlots, publishedTrack) {
		log.Printf("[SFU] Track already published subscriber=%v publisher=%v trackID=%v", client.UserId, publishedTrack.PublisherID, publishedTrack.TrackID)
		return
	}

	// Search for free slots.
	for index, slot := range client.Subscriber.VideoSlots {

		slot.Mu.Lock()

		// Get the MID.
		mid := slot.Transceiver.Mid()
		if mid == "" {
			log.Println("[SFU] MID is empty for video")
			slot.Mu.Unlock()
			continue
		}
		log.Printf("[SFU][VIDEO] Checking slot=%d MID=%v occupied=%v", index, mid, slot.Occupied)

		// Check if the slot is already occupied or not.
		if slot.Occupied {
			log.Printf("[SFU][VIDEO] Slot already occupied slot=%d MID=%v publisher=%v trackID=%v", index, mid, slot.PublisherId, slot.TrackID)
			slot.Mu.Unlock()
			continue
		}

		// Find the sender.
		sender := slot.Transceiver.Sender()
		if sender == nil {
			log.Printf("[SFU][VIDEO] Sender is nil slot=%d MID=%v", index, mid)
			slot.Mu.Unlock()
			continue
		}

		slot.Mu.Unlock()

		// Replace the track to actually send the media.
		err := sender.ReplaceTrack(publishedTrack.LocalTrack)
		s.SendPLIToPublisher(publishedTrack)
		if err != nil {
			log.Println("[SFU] ReplaceTrack error for Video:", err)
			continue
		}
		log.Printf("[SFU] ReplaceTrack success for videoSlot=%d MID=%v", index, mid)

		slot.Mu.Lock()

		//Update the slot states.
		slot.Occupied = true
		slot.PublisherId = publishedTrack.PublisherID
		slot.TrackID = publishedTrack.TrackID
		log.Println("Video Slot secured succesfully")
		slot.Mu.Unlock()

		client.Mu.Lock()
		// store mid->publisher
		client.MidToPublisher[mid] = publishedTrack.PublisherID
		client.Mu.Unlock()

		s.mu.Lock()
		s.SubscriberToOccupiedSlots[client.UserId] = append(s.SubscriberToOccupiedSlots[client.UserId], slot)
		s.mu.Unlock()

		// Drain the RTCP packets for NACK, PLI etc.
		s.DrainRTCP(slot, publishedTrack)
		log.Println("DrainRTCP done for videoTrack")

		log.Printf("[SFU] Sending media-published event for videoTrack MID=%v publisher=%v subscriber=%v", mid, publishedTrack.PublisherID, client.UserId)

		// Create WS msg for sending the MID mapping so the frontend can map which track is from which user.
		msg := models.PublishMediaMessage{
			Type:      "media-published",
			Mid:       mid,
			Publisher: publishedTrack.PublisherID,
		}

		// Send the WS msg.
		client.SafeSend(msg)
		return
	}

	// I need to re define tranceivers and also re-negotiate.

	log.Printf("[SFU][VIDEO] No free VIDEO slot found subscriber=%v publisher=%v trackID=%v", client.UserId, publishedTrack.PublisherID, publishedTrack.TrackID)
}

func (s *SFU) PublishAudioStream(client *models.Client, publishedTrack *models.PublishedTrack) {

	if client.Subscriber == nil {
		log.Printf("[SFU][AUDIO] Subscriber PC is nil subscriber=%v", client.UserId)
		return
	}

	if !client.Subscriber.RemoteDescSet {
		log.Printf("[SFU][AUDIO] Remote description not set subscriber=%v", client.UserId)
		return
	}

	if IsTrackAlreadyPublished(client.Subscriber.AudioSlots, publishedTrack) {
		log.Printf("[SFU][AUDIO] Track already published subscriber=%v publisher=%v trackID=%v", client.UserId, publishedTrack.PublisherID, publishedTrack.TrackID)
		return
	}

	log.Printf("[SFU][AUDIO] Searching for free slot subscriber=%v totalSlots=%d", client.UserId, len(client.Subscriber.AudioSlots))

	for index, slot := range client.Subscriber.AudioSlots {

		slot.Mu.Lock()

		mid := slot.Transceiver.Mid()
		if mid == "" {
			log.Println("[SFU] MID is empty for video")
			slot.Mu.Unlock()
			continue
		}

		log.Printf("[SFU][AUDIO] Checking slot=%d MID=%v occupied=%v", index, mid, slot.Occupied)

		if slot.Occupied {
			log.Printf("[SFU][AUDIO] Slot already occupied slot=%d MID=%v publisher=%v trackID=%v", index, mid, slot.PublisherId, slot.TrackID)
			slot.Mu.Unlock()
			continue
		}

		sender := slot.Transceiver.Sender()

		if sender == nil {
			log.Printf("[SFU][AUDIO] Sender is nil slot=%d MID=%v", index, mid)
			slot.Mu.Unlock()
			continue
		}

		log.Printf("[SFU][AUDIO] Replacing track slot=%d MID=%v publisher=%v trackID=%v", index, mid, publishedTrack.PublisherID, publishedTrack.TrackID)

		slot.Mu.Unlock()

		err := sender.ReplaceTrack(publishedTrack.LocalTrack)

		if err != nil {
			log.Println("[SFU][AUDIO] ReplaceTrack error:", err)
			slot.Mu.Unlock()
			continue
		}

		log.Printf("[SFU][AUDIO] ReplaceTrack success slot=%d MID=%v", index, mid)

		slot.Mu.Lock()
		// Fill the slot states.
		slot.Occupied = true
		slot.PublisherId = publishedTrack.PublisherID
		slot.TrackID = publishedTrack.TrackID
		slot.Mu.Unlock()

		// Drain the RTCP.
		s.DrainRTCP(slot, publishedTrack)
		log.Println("RTCP Drain done for audio")

		// Update client states.
		client.Mu.Lock()
		client.MidToPublisher[mid] = publishedTrack.PublisherID
		client.Mu.Unlock()

		s.mu.Lock()
		s.SubscriberToOccupiedSlots[client.UserId] = append(s.SubscriberToOccupiedSlots[client.UserId], slot)
		s.mu.Unlock()

		log.Printf("[SFU][AUDIO] Sending media-published event MID=%v publisher=%v subscriber=%v", mid, publishedTrack.PublisherID, client.UserId)

		// Create the WS message for the frontend.
		msg := models.PublishMediaMessage{
			Type:      "media-published",
			Mid:       mid,
			Publisher: publishedTrack.PublisherID,
		}

		// Send the WS message.
		client.SafeSend(msg)
		return
	}

	log.Printf("[SFU][AUDIO] No free AUDIO slot found subscriber=%v publisher=%v trackID=%v", client.UserId, publishedTrack.PublisherID, publishedTrack.TrackID)
}

func (s *SFU) SendLocalMediaToRemotePeers(client *models.Client) {
	otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		log.Println("[SFU] Error getting the other peers in the room")
		return
	}

	s.mu.RLock()
	localTracks := append([]*models.PublishedTrack(nil), s.UserIdToPublishedTracks[client.UserId]...)
	s.mu.RUnlock()
	for _, peer := range otherPeers {
		for _, localtrack := range localTracks {
			switch localtrack.Kind {
			case webrtc.RTPCodecTypeVideo:
				s.PublishVideoStream(peer, localtrack)

			case webrtc.RTPCodecTypeAudio:
				s.PublishAudioStream(peer, localtrack)
			}
		}
	}
}

func (s *SFU) SendRemoteMediaToLocalPeer(client *models.Client) {
	otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		log.Println("[SFU] Error getting the other peers in the room")
		return
	}

	for _, peer := range otherPeers {
		tracks := s.UserIdToPublishedTracks[peer.UserId]

		for _, publishedTrack := range tracks {

			switch publishedTrack.Kind {

			case webrtc.RTPCodecTypeVideo:
				s.PublishVideoStream(client, publishedTrack)

			case webrtc.RTPCodecTypeAudio:
				s.PublishAudioStream(client, publishedTrack)
			}
		}
	}
}
