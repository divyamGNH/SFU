package room

import (
	"backend/logger"
	"backend/participant"
	"backend/sfu"
	"backend/types"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type RoomHandler struct {
	RoomIdToRoom   map[string]*Room
	UserIdToRoomId map[string]string
	Mu             sync.RWMutex
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

//Helper functions.

func (rh *RoomHandler) BroadcastMessage(msg any, client *participant.Client) {
	roomId := client.RoomId
	userId := client.UserId

	// Get other peers from the room
	otherPeers, ok := rh.GetOtherPeersFromARoom(roomId, userId)
	if !ok {
		logger.Errorf("Error braodcasting socket event roomId : %s and userId : %s", roomId, userId)
		return
	}

	for _, peer := range otherPeers {
		ok := peer.SafeSend(msg)
		if !ok {
			logger.Errorf("Error sending the broadcast message to peer : %s from cliendId : %s", peer.UserId, userId)
		}
	}
}

func (rh *RoomHandler) WriteJSON(w http.ResponseWriter, message any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(message)

	if err != nil {
		logger.Error("[JoinRoom] Error encoding error response:", err)
	}
}

func (rh *RoomHandler) WriteError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(types.ErrorResponse{
		Message: message,
	})

	if err != nil {
		logger.Error("[JoinRoom] Error encoding error response:", err)
	}
}

// Generate a unique roomId and return it as well.
func (rh *RoomHandler) RoomIdGenerator() string {
	return uuid.NewString()
}

// Find the room using its roomId.
func (rh *RoomHandler) GetRoom(roomId string) (*Room, bool) {
	rh.Mu.RLock()
	room, ok := rh.RoomIdToRoom[roomId]
	rh.Mu.RUnlock()
	return room, ok
}

// Find the roomId for a given userId
func (rh *RoomHandler) RoomIdForUser(userId string) (string, bool) {
	rh.Mu.RLock()
	roomId, ok := rh.UserIdToRoomId[userId]
	rh.Mu.RUnlock()

	return roomId, ok
}

func (rh *RoomHandler) GetClientFromUserId(userId string) (*participant.Client, bool) {
	// Get the roomId for this user
	roomId, ok := rh.RoomIdForUser(userId)
	if !ok {
		logger.Info("[ROOM] No room found for this userId")
		return nil, false
	}

	// Get the room using the roomId
	room, ok := rh.GetRoom(roomId)
	if !ok {
		logger.Info("[ROOM] Room does not exist")
		return nil, false
	}

	// Get the client from the room
	room.Mu.RLock()
	client, exists := room.UserIdToClient[userId]
	room.Mu.RUnlock()

	if !exists {
		logger.Info("[ROOM] Client not found in room")
		return nil, false
	}

	return client, true
}

func (rh *RoomHandler) GetOtherPeersFromARoom(roomId string, userId string) ([]*participant.Client, bool) {
	//get the room
	logger.Info("Getting peers from the room")
	room, ok := rh.GetRoom(roomId)

	if !ok {
		logger.Error("Error while getting other peers from a room")
		return nil, false
	}

	//read the clients map from the room.UserIdToRoomId and return all the users except the userId from the parameters

	var otherUsers []*participant.Client

	room.Mu.Lock()
	for id, client := range room.UserIdToClient {
		if id == userId {
			continue
		}
		otherUsers = append(otherUsers, client)
	}
	room.Mu.Unlock()

	return otherUsers, true
}

//Actual routed and ws functions.

// route will be /viewroom.
func (rh *RoomHandler) ViewRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	roomId := vars["roomId"]
	// clientId := vars["clientId"]

	room, ok := rh.GetRoom(roomId)
	if !ok {
		logger.Error("[RoomH] Error while getting other peers from a room")
		rh.WriteError(w, "Room not found", http.StatusNotFound)
		return
	}

	var otherPeers []types.PeerState
	room.Mu.RLock()
	for _, client := range room.UserIdToClient {
		// if client.UserId == clientId {
		// 	continue
		// }

		client.Mu.RLock()
		state := types.PeerState{
			UserId:    client.UserId,
			AudioBool: client.AudioBool,
			VideoBool: client.VideoBool,
		}
		client.Mu.RUnlock()

		otherPeers = append(otherPeers, state)
	}
	room.Mu.RUnlock()

	response := types.ViewRoomResponse{
		OtherPeers: otherPeers,
	}

	rh.WriteJSON(w, response, http.StatusOK)
}

func (rh *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		rh.WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientId := uuid.NewString()
	roomId := rh.RoomIdGenerator()

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

	// Add userId -> roomId
	rh.UserIdToRoomId[clientId] = roomId
	rh.Mu.Unlock()

	// Store the creator in room membership list
	room.Mu.Lock()
	room.UserIds = append(room.UserIds, clientId)
	room.Mu.Unlock()

	response := types.CreateRoomResponse{
		RoomId: roomId,
		UserId: clientId,
	}

	rh.WriteJSON(w, response, http.StatusCreated)
}

func (rh *RoomHandler) JoinRoom(roomId string, clientId string) error {

	// Get the room
	room, ok := rh.GetRoom(roomId)
	if !ok {
		return fmt.Errorf("No such room with roomId : %s exists", roomId)
	}

	room.Mu.Lock()
	room.UserIds = append(room.UserIds, clientId)
	room.Mu.Unlock()

	// Add mapping userId -> roomId
	rh.Mu.Lock()
	rh.UserIdToRoomId[clientId] = roomId
	rh.Mu.Unlock()

	// Iris handles the peer-joined event being sent.

	// // emit the peer-joined event to all the other connected peers in the room.
	// peerJoinedMsg := types.JoinRoomSuccessMessage{
	// 	Type:   "peer-joined",
	// 	RoomId: roomId,
	// 	UserId: clientId,
	// }

	// room.Mu.RLock()

	// for id, client := range room.UserIdToClient {
	// 	// do not emit the event to the user itself
	// 	if id == clientId {
	// 		continue
	// 	}
	// 	client.Send <- peerJoinedMsg
	// }

	// room.Mu.RUnlock()

	return nil
}

func (rh *RoomHandler) LeaveRoom(roomId string, clientId string) error {

	err := rh.RemoveClient(roomId, clientId)
	if err != nil {
		return err
	}

	// Iris handles all the ws stuff

	// // Emit the peer-left event to all the remaining connected peers in the room.
	// peerLeftMsg := types.LeaveRoomSuccessMessage{
	// 	Type:   "peer-left",
	// 	RoomId: roomId,
	// 	UserId: clientId,
	// }

	// room.Mu.RLock()

	// for id, client := range room.UserIdToClient {

	// 	// Do not emit the event to the leaving user itself
	// 	if id == userId {
	// 		continue
	// 	}

	// 	client.Send <- peerLeftMsg
	// }

	// room.Mu.RUnlock()

	return nil
}

func (rh *RoomHandler) HandleOffer(roomId string, clientId string, offerString string) (string, error) {

	// Get the room.
	room, ok := rh.GetRoom(roomId)
	if !ok {
		return "", fmt.Errorf("Room with roomId : %s not found", roomId)
	}

	// Find the client in the room.
	room.Mu.RLock()
	client, ok := room.UserIdToClient[clientId]
	room.Mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("Client with clientId : %s not found in the room", clientId)
	}

	// Call the Publisher.
	answer, err := client.Publisher.HandleOffer(offerString)
	if err != nil {
		return "", err
	}

	return answer.SDP, nil
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

func (rh *RoomHandler) HandleToggleAudio(muted bool, client *participant.Client) {
	client.Mu.Lock()
	client.AudioBool = muted
	client.Mu.Unlock()

	msg := &types.AudioToggleMessageRes{
		Type:   "audio-toggle",
		UserId: client.UserId,
		Muted:  client.AudioBool,
	}

	// Send a event to the other peers in the room so that they can update their UI.
	rh.BroadcastMessage(msg, client)
}

func (rh *RoomHandler) HandleToggleVideo(muted bool, client *participant.Client) {
	client.Mu.Lock()
	client.VideoBool = muted
	client.Mu.Unlock()

	msg := &types.VideoToggleMessageRes{
		Type:   "video-toggle",
		UserId: client.UserId,
		Muted:  client.VideoBool,
	}

	// Send a event to the other peers in the room so that they can update their UI.
	rh.BroadcastMessage(msg, client)
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

	// Send the MID mapping to the frontend.
	msg := types.PublishMediaMessage{
		Type:      "media-published",
		Mid:       slot.Transceiver.Mid(),
		Publisher: track.PublisherID,
	}
	subscriber.SafeSend(msg)

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

	// Send a peer-left ws message.
	peerLeftMsg := types.LeaveRoomSuccessMessage{
		Type:   "peer-left",
		RoomId: roomId,
		UserId: userId,
	}

	if clientExists {
		rh.BroadcastMessage(peerLeftMsg, client)
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
