package handlers

import (
	"backend/models"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type RoomHandler struct {
	RoomIdToRoom map[string]*Room
	mu           sync.RWMutex
}

type Room struct {
	RoomId         string
	UserIdToClient map[string]*models.Client

	mu sync.RWMutex
}

// Create a new room handler and return it as well.
func NewRoomHandler() *RoomHandler {
	return &RoomHandler{
		RoomIdToRoom: make(map[string]*Room),
	}
}

func (rh *RoomHandler) RoomIdGenerator() string {
	return uuid.NewString()
}

// Find the room using its roomId
func (rh *RoomHandler) GetRoom(roomId string) (*Room, bool) {
	rh.mu.RLock()
	room, ok := rh.RoomIdToRoom[roomId]
	rh.mu.RUnlock()
	return room, ok
}

// route will be /create-room
func (rh *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	roomId := rh.RoomIdGenerator()

	var body models.CreateRoomRequest

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Println("Error decoding the create room request as json: ", err)
		return
	}

	room := &Room{
		RoomId:         roomId,
		UserIdToClient: make(map[string]*models.Client),
	}

	client := &models.Client{
		RoomId: roomId,
		UserId: body.UserId,
	}

	rh.mu.Lock()
	rh.RoomIdToRoom[roomId] = room
	rh.mu.Unlock()

	room.mu.Lock()
	room.UserIdToClient[body.UserId] = client
	room.mu.Unlock()

	//emit the create-room or join-room event
}

// route will be /join-room
func (rh *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	//Check if room even exists
	var body models.JoinRoomRequest

	//Decode the body of the request
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Println("Error decoding the join room request as json: ", err)
		return
	}

	//Get the room from roomId
	room, ok := rh.GetRoom(body.RoomId)
	if !ok {
		http.Error(w, "No such room with this roomId exists", http.StatusNotFound)
		return
	}

	room.mu.Lock()
	_, exists := room.UserIdToClient[body.UserId]
	room.mu.Unlock()

	if exists {
		http.Error(w, "user already exists", http.StatusBadRequest)
		return
	}

	//create a client object. This object is incomplete atp the other fields are null by default
	client := &models.Client{
		UserId: body.UserId,
		RoomId: body.RoomId,
	}

	//add the client in the room
	room.mu.Lock()
	room.UserIdToClient[body.UserId] = client
	room.mu.Unlock()

	//emit the join-room event

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// json.NewEncoder(w).Encode()
}

// route will be /leave-room
func (rh *RoomHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	var body models.LeaveRoomRequest

	//decode the body of the request
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Println("Error decoding the leave room request as json: ", err)
		return
	}

	//Check if the room even exists or not
	room, ok := rh.GetRoom(body.RoomId)
	if !ok {
		http.Error(w, "No such room with this roomId exists", http.StatusNotFound)
		return
	}

	//delete the client from the room
	room.mu.Lock()
	defer room.mu.Unlock()
	delete(room.UserIdToClient, body.UserId)

	rh.CleanRoom(body.RoomId)

	//emit the leave-room event
}

// route will be /view-room
func (rh *RoomHandler) ViewRoom(w http.ResponseWriter, r *http.Request) {

}

func (rh *RoomHandler) CleanRoom(roomId string) {
	//first check if the room has any users left or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		log.Println("Room with this roomid does not exist")
	}

	room.mu.Lock()
	isEmpty := len(room.UserIdToClient) == 0
	room.mu.Unlock()

	if !isEmpty {
		log.Println("This room still has clients can not clean it")
		return
	}

	rh.mu.Lock()
	delete(rh.RoomIdToRoom, roomId)
	rh.mu.Unlock()
}
