package room

import (
	"backend/models"
	"encoding/json"
	"log"
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
	RoomId         string
	UserIdToClient map[string]*models.Client
	UserIds        []string

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

func (rh *RoomHandler) WriteJSON(w http.ResponseWriter, message any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(message)

	if err != nil {
		log.Println("[JoinRoom] Error encoding error response:", err)
	}
}

func (rh *RoomHandler) WriteError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Message: message,
	})

	if err != nil {
		log.Println("[JoinRoom] Error encoding error response:", err)
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

func (rh *RoomHandler) GetClientFromUserId(userId string) (*models.Client, bool) {
	// Get the roomId for this user
	roomId, ok := rh.RoomIdForUser(userId)
	if !ok {
		log.Println("[ROOM] No room found for this userId")
		return nil, false
	}

	// Get the room using the roomId
	room, ok := rh.GetRoom(roomId)
	if !ok {
		log.Println("[ROOM] Room does not exist")
		return nil, false
	}

	// Get the client from the room
	room.Mu.RLock()
	client, exists := room.UserIdToClient[userId]
	room.Mu.RUnlock()

	if !exists {
		log.Println("[ROOM] Client not found in room")
		return nil, false
	}

	return client, true
}

func (rh *RoomHandler) GetOtherPeersFromARoom(roomId string, userId string) ([]*models.Client, bool) {
	//get the room
	log.Println("Getting peers from the room")
	room, ok := rh.GetRoom(roomId)

	if !ok {
		log.Println("Error while getting other peers from a room")
		return nil, false
	}

	//read the clients map from the room.UserIdToRoomId and return all the users except the userId from the parameters

	var otherUsers []*models.Client
	for id, client := range room.UserIdToClient {
		if id == userId {
			continue
		}
		otherUsers = append(otherUsers, client)
	}

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
		log.Println("[RoomH] Error while getting other peers from a room")
		rh.WriteError(w, "Room not found", http.StatusNotFound)
		return
	}

	// log.Println(clientId)
	var otherPeers []string
	room.Mu.RLock()
	for userId := range room.UserIdToClient {
		// if userId == clientId {
		// 	continue
		// }
		otherPeers = append(otherPeers, userId)
	}
	room.Mu.RUnlock()

	for userId := range otherPeers {
		log.Println(userId)
	}

	response := models.ViewRoomResponse{
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
		RoomId:         roomId,
		UserIdToClient: make(map[string]*models.Client),
		UserIds:        []string{},
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

	response := models.CreateRoomResponse{
		RoomId: roomId,
		UserId: clientId,
	}

	rh.WriteJSON(w, response, http.StatusCreated)
}

func (rh *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		rh.WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)

	roomId := vars["roomId"]

	clientId := uuid.NewString()

	if roomId == "" {

		rh.WriteError(w, "roomId is required", http.StatusBadRequest)

		return
	}

	// Get the room
	room, ok := rh.GetRoom(roomId)
	if !ok {

		rh.WriteError(w, "No such room with this roomId exists", http.StatusNotFound)

		return
	}

	room.Mu.Lock()

	// The clientId was generated 5  lines earlier it will never be duplicate.

	// _, exists := room.UserIdToClient[clientId]

	// if exists {
	// 	room.Mu.Unlock()

	// 	rh.WriteError(w, "User with userId already exists", http.StatusBadRequest)

	// 	return
	// }

	// User joins logically here
	room.UserIds = append(room.UserIds, clientId)

	room.Mu.Unlock()

	// Add mapping userId -> roomId
	rh.Mu.Lock()
	rh.UserIdToRoomId[clientId] = roomId
	rh.Mu.Unlock()

	// emit the peer-joined event to all the other connected peers in the room.
	peerJoinedMsg := models.JoinRoomSuccessMessage{
		Type:   "peer-joined",
		RoomId: roomId,
		UserId: clientId,
	}

	room.Mu.RLock()

	for id, client := range room.UserIdToClient {

		// do not emit the event to the user itself
		if id == clientId {
			continue
		}

		client.Send <- peerJoinedMsg
	}

	room.Mu.RUnlock()

	successMsg := models.JoinRoomResponse{
		RoomId: roomId,
		UserId: clientId,
	}

	rh.WriteJSON(w, successMsg, http.StatusOK)
}

func (rh *RoomHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {

	// Ensure the method is POST method only.
	// Once we connect to the DB we need to make this POST to DELETE
	if r.Method != http.MethodPost {
		rh.WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the variables from the link.
	vars := mux.Vars(r)
	roomId := vars["roomId"]
	userId := vars["clientId"]

	//Check if the room even exists or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		rh.WriteError(w, "No such room with this roomId exists", http.StatusNotFound)
		return
	}

	//delete the client from the room
	room.Mu.Lock()

	// Check if the userId sent by the frontend actually exists before cleaning this up.
	found := false
	for _, id := range room.UserIds {
		if id == userId {
			found = true
			break
		}
	}

	if !found {
		room.Mu.Unlock()

		rh.WriteError(w, "User is not a member of this room", http.StatusNotFound)
		return
	}

	delete(room.UserIdToClient, userId)

	for i, id := range room.UserIds {
		if id == userId {
			room.UserIds = append(room.UserIds[:i], room.UserIds[i+1:]...)
			break
		}
	}

	room.Mu.Unlock()

	rh.Mu.Lock()
	delete(rh.UserIdToRoomId, userId)
	rh.Mu.Unlock()

	rh.CleanRoom(roomId)

	// emit the peer-left event to all the remaining connected peers in the room.
	peerLeftMsg := models.LeaveRoomSuccessMessage{
		Type:   "peer-left",
		RoomId: roomId,
		UserId: userId,
	}

	room.Mu.RLock()

	for id, client := range room.UserIdToClient {

		// do not emit the event to the leaving user itself
		if id == userId {
			continue
		}

		client.Send <- peerLeftMsg
	}

	room.Mu.RUnlock()

	//emit the leave-room-success response
	successMsg := models.LeaveRoomResponse{
		Message: "Left room successfully",
	}

	rh.WriteJSON(w, successMsg, http.StatusOK)
}

// Handle all the cleanup
func (rh *RoomHandler) CleanRoom(roomId string) {
	//first check if the room has any users left or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		log.Println("Room with this roomid does not exist")
		return
	}

	room.Mu.RLock()
	isEmpty := len(room.UserIds) == 0
	room.Mu.RUnlock()

	if !isEmpty {
		log.Println("This room still has clients can not clean it")
		return
	}

	rh.Mu.Lock()
	delete(rh.RoomIdToRoom, roomId)
	rh.Mu.Unlock()

	log.Printf("[Room] Room with roomid : %v has been deleted", roomId)
}
