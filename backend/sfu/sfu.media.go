package sfu

import (
	"backend/models"
	"backend/sfu/pool"
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
func (s *SFU) DrainRTCP(slot *pool.MediaSlot) {
	// Create a new go routine so this is basically a readPump
	log.Println("inside drain rtcp")

	sender := slot.Transceiver.Sender()

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

		st := slot.Load()
		if st.PublisherId == "" {
			continue
		}

		currentTrack := s.TrackIdToPublishedTracks[st.TrackID]
		if currentTrack == nil {
			continue
		}

		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication:
				log.Println("[SFU] PLI received.")
				s.SendPLIToPublisher(currentTrack)

			case *rtcp.FullIntraRequest:
				log.Println("[SFU] FIR received")
				s.SendPLIToPublisher(currentTrack)

			case *rtcp.TransportLayerNack:
				log.Println("[SFU] Transport layer NACK received")

			case *rtcp.ReceiverReport:
				// log.Println("[SFU] Receviver Report received")
			}
		}
	}
}

// Send video realted media to a single client specified in the function.
// return bool needNegotiation, bool alreadyPublished, error
func (s *SFU) PublishVideoStream(client *models.Client, publishedTrack *models.PublishedTrack) (bool, bool, error) {

	// Get the video pool.
	videoPool := client.Subscriber.VideoPool

	// First check if the track is already been published or not.
	if videoPool.ContainsTrack(publishedTrack.PublisherID, publishedTrack.TrackID) {
		return false, true, nil
	}

	// We get slot, generation, doneCh, boolean flag
	slot, _, _, ok := client.Subscriber.VideoPool.Acquire(
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
		publishedTrack.Kind,
	)

	if !ok {
		// true means negotiation and Grow() is needed we need more transceivers.
		return true, false, nil
	}

	// Attach the intended track to the sender
	err := slot.Transceiver.Sender().ReplaceTrack(publishedTrack.LocalTrack)
	if err != nil {
		// There was a error replacing the track so free the slot we acquired to prevent slot leakage.
		client.Subscriber.VideoPool.Release(slot.Index)
		log.Printf("Error relacing video track : %v", err)
		return false, false, err
	}

	// Check if the DrainRTCP is started for this slot or not. If not start it.
	if !slot.TryStartDrainRTCP() {
		// DrainRTCP already spins up a new go-routine internally.
		go s.DrainRTCP(slot)
	}

	// Get the Mid of the transceiver for frontend mapping.
	mid := slot.Transceiver.Mid()

	// Create WS msg for sending the MID mapping so the frontend can map which track is from which user.
	msg := models.PublishMediaMessage{
		Type:      "media-published",
		Mid:       mid,
		Publisher: publishedTrack.PublisherID,
	}

	// Send the WS msg.
	client.SafeSend(msg)
	return false, false, nil
}

// Send audio related media to a single client specified in the function.
// return bool needNegotiation, bool alreadyPublished, error
func (s *SFU) PublishAudioStream(client *models.Client, publishedTrack *models.PublishedTrack) (bool, bool, error) {

	// Get the audioPool
	audioPool := client.Subscriber.AudioPool

	// First check if the track is already been published or not.
	if audioPool.ContainsTrack(publishedTrack.PublisherID, publishedTrack.TrackID) {
		return false, true, nil
	}

	// We get slot, generation, doneCh, boolean flag
	slot, _, _, ok := client.Subscriber.AudioPool.Acquire(
		publishedTrack.PublisherID,
		publishedTrack.TrackID,
		publishedTrack.Kind,
	)

	if !ok {
		// true means negotiation and Grow() is needed we need more transceivers.
		return true, false, nil
	}

	// Attach the intended track to the sender
	err := slot.Transceiver.Sender().ReplaceTrack(publishedTrack.LocalTrack)
	if err != nil {
		// There was a error replacing the track so free the slot we acquired to prevent slot leakage.
		client.Subscriber.AudioPool.Release(slot.Index)
		log.Printf("Error relacing video track : %w", err)
		return false, false, err
	}

	// Check if the DrainRTCP is started for this slot or not. If not start it.
	if !slot.TryStartDrainRTCP() {
		// DrainRTCP already spins up a new go-routine internally.
		go s.DrainRTCP(slot)
	}

	// Get the Mid of the transceiver for frontend mapping.
	mid := slot.Transceiver.Mid()

	// Create WS msg for sending the MID mapping so the frontend can map which track is from which user.
	msg := models.PublishMediaMessage{
		Type:      "media-published",
		Mid:       mid,
		Publisher: publishedTrack.PublisherID,
	}

	// Send the WS msg.
	client.SafeSend(msg)
	return false, false, nil
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
				needVideoReNegotiation, alreadyPublished, err := s.PublishVideoStream(peer, localtrack)

				if needVideoReNegotiation {
					// Grow logic and debouncing logic here
				} else if err != nil {
					log.Printf("Error publishing the videoTrack %v", err)
				} else if alreadyPublished {
					log.Println("Video Track already published")
				}

			case webrtc.RTPCodecTypeAudio:
				needAudioReNegotiation, alreadyPublished, err := s.PublishAudioStream(peer, localtrack)

				if needAudioReNegotiation {
					// Grow logic and debouncing logic here
				} else if err != nil {
					log.Printf("Error publishing the videoTrack %v", err)
				} else if alreadyPublished {
					log.Println("Video Track already published")
				}
			}
		}

		// log.Printf("Negotiation needed or not status is : %t", needNegotiation)
		// // Any of the video or audio failed to get a slot so basically we assign new transceivers for both of them
		// if needNegotiation {
		// 	// I need to re define 10 tranceivers for video and audio each.
		// 	log.Printf("[SFU] No free AUDIO or VIDEO slot found. Creating more tranceivers")

		// 	err := s.CreateTranceivers(peer)
		// 	if err != nil {
		// 		log.Printf("Existing transceivers are full error creating new ones : %s", err)
		// 	}

		// 	// Re-negotiate
		// 	log.Printf("Renegotiating the new transceivers %s", peer.UserId)

		// 	// Do re-negotitation for all the users in the room
		// 	s.RenegotiateSubscriberOffer(peer)

		// 	needNegotiation = false
		// }
	}
}

func (s *SFU) SendRemoteMediaToLocalPeer(client *models.Client) {
	otherPeers, ok := s.rm.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		log.Println("[SFU] Error getting the other peers in the room")
		return
	}

	if client.Subscriber == nil {
		log.Printf("[SFU] Subscriber PC for videoTrack is nil subscriber=%v", client.UserId)
		return
	}

	if !client.Subscriber.RemoteDescSet {
		log.Printf("[SFU][AUDIO] Remote description not set subscriber=%v", client.UserId)
		return
	}

	for _, peer := range otherPeers {
		tracks := s.UserIdToPublishedTracks[peer.UserId]

		for _, publishedTrack := range tracks {

			switch publishedTrack.Kind {

			case webrtc.RTPCodecTypeVideo:
				needVideoReNegotiation, alreadyPublished, err := s.PublishVideoStream(client, publishedTrack)

				if needVideoReNegotiation {
					// Grow logic and debouncing logic here
				} else if err != nil {
					log.Printf("Error publishing the videoTrack %v", err)
				} else if alreadyPublished {
					log.Println("Video Track already published")
				}

			case webrtc.RTPCodecTypeAudio:
				needAudioReNegotiation, alreadyPublished, err := s.PublishAudioStream(client, publishedTrack)

				if needAudioReNegotiation {
					// Grow logic and debouncing logic here
				} else if err != nil {
					log.Printf("Error publishing the videoTrack %v", err)
				} else if alreadyPublished {
					log.Println("Video Track already published")
				}
			}
		}
	}

	// log.Printf("Negotiation needed or not status is : %t", needNegotiation)
	// // Any of the video or audio failed to get a slot so basically we assign new transceivers for both of them
	// if needNegotiation {
	// 	// I need to re define 10 tranceivers for video and audio each.
	// 	log.Printf("[SFU] No free AUDIO or VIDEO slot found. Creating more tranceivers")

	// 	err := s.CreateTranceivers(client)
	// 	if err != nil {
	// 		log.Printf("Existing transceivers are full error creating new ones : %s", err)
	// 	}

	// 	// Re-negotiate
	// 	log.Printf("Renegotiating the new transceivers %s", client.UserId)

	// 	// Do re-negotitation for all the users in the room
	// 	s.RenegotiateSubscriberOffer(client)

	// 	needNegotiation = false
	// }
}
