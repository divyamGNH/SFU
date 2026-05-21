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
		http.Error(w, "Room not found", http.StatusNotFound)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := models.ViewRoomResponse{
		OtherPeers: otherPeers,
	}

	json.NewEncoder(w).Encode(response)
}

// IMPTODO : A huge mistake here the frontend is completely dependent on the backend to send the userId/clientId and the backend expects the frontend to send it so fix that architectural issue.

func (rh *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	response := models.CreateRoomSuccessMessage{
		RoomId: roomId,
		UserId: clientId,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("[CreateRoom] Error encoding response:", err)
		return
	}
}

func (rh *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "Method not allowed",
		})

		if err != nil {
			log.Println("[JoinRoom] Error encoding response:", err)
		}

		return
	}

	vars := mux.Vars(r)

	roomId := vars["roomId"]

	clientId := uuid.NewString()

	if roomId == "" {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "roomId is required",
		})

		if err != nil {
			log.Println("[JoinRoom] Error encoding response:", err)
		}

		return
	}

	// Get the room
	room, ok := rh.GetRoom(roomId)
	if !ok {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "No such room with this roomId exists",
		})

		if err != nil {
			log.Println("[JoinRoom] Error encoding response:", err)
		}

		return
	}

	room.Mu.Lock()

	_, exists := room.UserIdToClient[clientId]

	if exists {
		room.Mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "User with userId already exists",
		})

		if err != nil {
			log.Println("[JoinRoom] Error encoding response:", err)
		}

		return
	}

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(successMsg)

	if err != nil {
		log.Println("[JoinRoom] Error encoding response:", err)
		return
	}
}

// triggers on listening to "leaveroom" event
func (rh *RoomHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {

	// Ensure the method is POST method only.
	// Once we connect to the DB we need to make this POST to DELETE
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "Method not allowed",
		})

		if err != nil {
			log.Println("[LeaveRoom] Error encoding response:", err)
		}

		return
	}

	// Get the variables from the link.
	vars := mux.Vars(r)
	roomId := vars["roomId"]
	userId := vars["clientId"]

	//Check if the room even exists or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		err := json.NewEncoder(w).Encode(models.ErrorResponse{
			Message: "No such room with this roomId exists",
		})

		if err != nil {
			log.Println("[LeaveRoom] Error encoding response:", err)
		}
		return
	}

	//delete the client from the room
	room.Mu.Lock()
	delete(room.UserIdToClient, userId)

	for i, id := range room.UserIds {
		if id == userId {
			room.UserIds = append(room.UserIds[:i], room.UserIds[i+1:]...)
			break
		}
	}

	room.Mu.Unlock()

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(successMsg)

	if err != nil {
		log.Println("[LeaveRoom] Error encoding response:", err)
		return
	}
}

// Handle all the cleanup
// TODO : Implement this also keep in mind to make this either http based route or a helper function.
func (rh *RoomHandler) CleanRoom(roomId string) {
	//first check if the room has any users left or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		log.Println("Room with this roomid does not exist")
		return
	}

	room.Mu.RLock()
	isEmpty := len(room.UserIdToClient) == 0
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
