package room

import (
	"backend/logger"
	"backend/participant"
	"backend/sfu"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
)

type RoomCallbacks struct {
	OnMediaPublished           func(clientId string, mid string, publisherId string)
	SendPublisherICECandidate  func(roomId string, clientId string, candidate webrtc.ICECandidateInit)
	SendSubscriberICECandidate func(roomId string, clientId string, candidate webrtc.ICECandidateInit)
	SendSubscriberOffer        func(roomId string, clientId string, offer webrtc.SessionDescription)
}

type RoomHandler struct {
	RoomIdToRoom   map[string]*Room
	UserIdToRoomId map[string]string
	Mu             sync.RWMutex
	callbacks      RoomCallbacks
}

type Room struct {
	RoomId                   string
	UserIdToClient           map[string]*participant.Client
	UserIds                  []string
	UserIdToPublishedTracks  map[string][]*participant.PublishedTrack
	TrackIdToPublishedTracks map[string]*participant.PublishedTrack
	TrackIdToReceiver        map[string]*sfu.Receiver

	Mu sync.RWMutex
}

// Create a new room handler and return it as well.
func NewRoomHandler() *RoomHandler {
	return &RoomHandler{
		RoomIdToRoom:   make(map[string]*Room),
		UserIdToRoomId: make(map[string]string),
	}
}

func (rh *RoomHandler) SetCallbacks(callbacks RoomCallbacks) {
	rh.callbacks = callbacks
}

// Iris calls this function when a client requests create-room then iris gets the room and calls joinroom on it that is why we dont actually put any data realted to user in the create-room function.
// Iris sends the roomId so we dont need to create roomId anymore.
func (rh *RoomHandler) CreateRoom(roomId string) *Room {

	room := &Room{
		RoomId:                   roomId,
		UserIdToClient:           make(map[string]*participant.Client),
		UserIds:                  []string{},
		UserIdToPublishedTracks:  make(map[string][]*participant.PublishedTrack),
		TrackIdToPublishedTracks: make(map[string]*participant.PublishedTrack),
		TrackIdToReceiver:        make(map[string]*sfu.Receiver),
	}

	// Add roomId -> room
	rh.Mu.Lock()
	rh.RoomIdToRoom[roomId] = room
	rh.Mu.Unlock()

	return room
}

func (rh *RoomHandler) JoinRoom(roomId string, clientId string) error {
	// Get the room
	room, ok := rh.GetRoom(roomId)
	if !ok {
		return fmt.Errorf("No such room with roomId : %s exists", roomId)
	}

	// Define the callbacks.
	// Some callbacks we passing come from room callbacks that come from the service file in the grpc package.
	callbacks := participant.ClientCallbacks{
		GetIceServers: func() []webrtc.ICEServer {
			return []webrtc.ICEServer{} // TODO: Fetch from config later
		},

		PubCallbacks: participant.PublisherCallbacks{
			OnTrackPublished: func(track *participant.PublishedTrack, client *participant.Client) {
				rh.OnTrackPublished(track, client)
			},
			SendPublisherICECandidate: func(client *participant.Client, candidate webrtc.ICECandidateInit) {
				if rh.callbacks.SendPublisherICECandidate != nil {
					rh.callbacks.SendPublisherICECandidate(roomId, client.UserId, candidate)
				}
			},
		},

		SubCallbacks: participant.SubscriberCallbacks{
			OnNegotiationCompleted: func(client *participant.Client) {
				rh.OnNegotiationCompleted(client)
			},
			SendSubscriberICECandidate: func(client *participant.Client, candidate webrtc.ICECandidateInit) {
				if rh.callbacks.SendSubscriberICECandidate != nil {
					rh.callbacks.SendSubscriberICECandidate(roomId, client.UserId, candidate)
				}
			},
			SendSubscriberOffer: func(client *participant.Client, offer webrtc.SessionDescription) {
				if rh.callbacks.SendSubscriberOffer != nil {
					rh.callbacks.SendSubscriberOffer(roomId, client.UserId, offer)
				}
			},
		},
	}

	// Create a new client.
	newClient, err := participant.NewClient(roomId, clientId, callbacks)
	if err != nil {
		return err
	}

	// Update the maps.
	room.Mu.Lock()
	room.UserIdToClient[clientId] = newClient
	room.UserIds = append(room.UserIds, clientId)
	room.Mu.Unlock()

	rh.Mu.Lock()
	rh.UserIdToRoomId[clientId] = roomId
	rh.Mu.Unlock()

	return nil
}

func (rh *RoomHandler) LeaveRoom(roomId string, clientId string) error {
	// Clear user related maps.
	err := rh.RemoveClient(roomId, clientId)
	if err != nil {
		return err
	}

	return nil
}

func (rh *RoomHandler) HandleOffer(clientId string, offerString string) (webrtc.SessionDescription, error) {
	client, ok := rh.GetClientFromUserId(clientId)
	if !ok {
		return webrtc.SessionDescription{}, fmt.Errorf("Client with clientId : %s not found in the room", clientId)
	}

	// Call the Publisher.
	answer, err := client.Publisher.HandleOffer(offerString)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

func (rh *RoomHandler) HandleAnswer(clientId string, answerString string) error {
	// Get the client.
	client, ok := rh.GetClientFromUserId(clientId)
	if !ok {
		return fmt.Errorf("Client with clientId : %s not found in the room", clientId)
	}

	// create answer
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerString,
	}

	// Pass it to the subscriber!
	err := client.Subscriber.HandleAnswer(answer)
	if err != nil {
		return err
	}
	return nil
}

func (r *Room) AddPublishedTracks(track *participant.PublishedTrack, receiver *sfu.Receiver) {
	r.Mu.Lock()

	r.UserIdToPublishedTracks[track.PublisherID] = append(r.UserIdToPublishedTracks[track.PublisherID], track)
	r.TrackIdToPublishedTracks[track.TrackID] = track
	r.TrackIdToReceiver[track.TrackID] = receiver

	r.Mu.Unlock()
}

func (rh *RoomHandler) OnTrackPublished(track *participant.PublishedTrack, client *participant.Client) {
	room, ok := rh.GetRoom(client.RoomId)
	if !ok {
		logger.Error("[RoomHandler] Error: Room not found for OnTrack")
		return
	}

	receiver := sfu.NewReceiver(track.RemoteTrack, track.LocalTrack, client.Publisher)

	room.AddPublishedTracks(track, receiver)

	// Broadcast the newly published track to all other peers in the room
	rh.SendLocalMediaToRemotePeers(client)
}

func (rh *RoomHandler) publishTrackToSubscriber(subscriber *participant.Client, track *participant.PublishedTrack) (bool, bool, error) {
	needsNegotiation, alreadyPublished, slot, err := subscriber.Subscriber.SubscribeToTrack(track)

	if needsNegotiation || alreadyPublished || err != nil {
		return needsNegotiation, alreadyPublished, err
	}

	// Get the room.
	room, _ := rh.GetRoom(subscriber.RoomId)

	// Look up the receiver from the room state
	room.Mu.RLock()
	rx := room.TrackIdToReceiver[track.TrackID]
	room.Mu.RUnlock()

	// Start the RTCPDrain if not started already.
	if rx != nil && slot.TryStartDrainRTCP() {
		fwd := sfu.NewForwarder(rx, slot.Transceiver.Sender())
		// Start a new go-routine for draining the RTCP.
		go fwd.DrainRTCP()
	}

	// Trigger the callback to service.go to notify the Iris about the status so that it can actually publish media-published event with mid mapping.
	if rh.callbacks.OnMediaPublished != nil {
		rh.callbacks.OnMediaPublished(subscriber.UserId, slot.Transceiver.Mid(), track.PublisherID)
	}

	return false, false, nil
}

func (rh *RoomHandler) SendLocalMediaToRemotePeers(client *participant.Client) {
	// Get the other peers.
	otherPeers, ok := rh.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		return
	}

	// Get the room.
	room, ok := rh.GetRoom(client.RoomId)
	if !ok {
		return
	}

	// Get the localTracks.
	room.Mu.RLock()
	localTracks := append([]*participant.PublishedTrack(nil), room.UserIdToPublishedTracks[client.UserId]...)
	room.Mu.RUnlock()

	// Send each localTrack to each remotePeer in the room.
	for _, peer := range otherPeers {
		for _, localTrack := range localTracks {
			needNegotiation, _, err := rh.publishTrackToSubscriber(peer, localTrack)
			if needNegotiation {
				// Trigger Subscriber grow
				kind := localTrack.Kind
				err := peer.Subscriber.GrowPool(kind)
				if err == nil {
					peer.Subscriber.RequestNegotiate()
				}
			} else if err != nil {
				logger.Error("[RoomHandler] Error publishing stream:", err)
			}
		}
	}
}

func (rh *RoomHandler) SendRemoteMediaToLocalPeer(client *participant.Client) {
	// Get the other peers in the room.
	otherPeers, ok := rh.GetOtherPeersFromARoom(client.RoomId, client.UserId)
	if !ok {
		return
	}

	// Get the room.
	room, ok := rh.GetRoom(client.RoomId)
	if !ok {
		return
	}

	// Send each remote peers each track to the local peer.
	for _, peer := range otherPeers {
		room.Mu.RLock()
		tracks := room.UserIdToPublishedTracks[peer.UserId]
		room.Mu.RUnlock()

		for _, publishedTrack := range tracks {
			needNegotiation, _, err := rh.publishTrackToSubscriber(client, publishedTrack)
			if needNegotiation {
				kind := publishedTrack.Kind
				err := client.Subscriber.GrowPool(kind)
				if err == nil {
					client.Subscriber.RequestNegotiate()
				}
			} else if err != nil {
				logger.Error("[RoomHandler] Error publishing stream:", err)
			}
		}
	}
}

// Handle all the cleanup
func (rh *RoomHandler) CleanRoom(roomId string) {
	//first check if the room has any users left or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		logger.Info("Room with this roomid does not exist")
		return
	}

	room.Mu.RLock()
	isEmpty := len(room.UserIds) == 0
	room.Mu.RUnlock()

	if !isEmpty {
		logger.Info("This room still has clients can not clean it")
		return
	}

	rh.Mu.Lock()
	delete(rh.RoomIdToRoom, roomId)
	rh.Mu.Unlock()

	logger.Infof("[Room] Room with roomid : %v has been deleted", roomId)
}

func (rh *RoomHandler) RemoveClient(roomId string, userId string) error {
	// Get the room
	room, ok := rh.GetRoom(roomId)
	if !ok {
		return fmt.Errorf("No room with roomId : %s found", roomId)
	}

	room.Mu.Lock()

	client, clientExists := room.UserIdToClient[userId]

	for i, id := range room.UserIds {
		if id == userId {
			room.UserIds = append(room.UserIds[:i], room.UserIds[i+1:]...)
			break
		}
	}

	delete(room.UserIdToClient, userId)

	if tracks, exists := room.UserIdToPublishedTracks[userId]; exists {
		for _, track := range tracks {
			delete(room.TrackIdToPublishedTracks, track.TrackID)
			delete(room.TrackIdToReceiver, track.TrackID)
		}
		delete(room.UserIdToPublishedTracks, userId)
	}

	room.Mu.Unlock()

	// Clean the client up to prevent memory leaks.
	if clientExists {
		client.CleanUpClient()
	}

	rh.Mu.Lock()
	delete(rh.UserIdToRoomId, userId)
	rh.Mu.Unlock()

	rh.CleanRoom(roomId)
	return nil
}

// OnNegotiationCompleted is called when a subscriber finishes its WebRTC offer/answer negotiation.
// We trigger SendRemoteMediaToLocalPeer to retry publishing any tracks that were blocked waiting for a transceiver.
func (rh *RoomHandler) OnNegotiationCompleted(client *participant.Client) {
	rh.SendRemoteMediaToLocalPeer(client)
}

func (rh *RoomHandler) HandlePublisherICECandidate(clientId string, candidate webrtc.ICECandidateInit) error {
	client, ok := rh.GetClientFromUserId(clientId)
	if !ok {
		return fmt.Errorf("Client with id : %v was not found", clientId)
	}

	client.Publisher.HandleICECandidate(candidate)
	return nil
}

func (rh *RoomHandler) HandleSubscriberICECandidate(clientId string, candidate webrtc.ICECandidateInit) error {
	client, ok := rh.GetClientFromUserId(clientId)
	if !ok {
		return fmt.Errorf("Client with id : %v was not found", clientId)
	}

	client.Subscriber.HandleIce(candidate)
	return nil
}
