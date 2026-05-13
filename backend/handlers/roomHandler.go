package handlers

import (
	"backend/models"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type RoomHandler struct {
	RoomIdToRoom   map[string]*Room
	UserIdToRoomId map[string]string
	mu             sync.RWMutex
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

// Generate a unique roomId and return it as well.
func (rh *RoomHandler) RoomIdGenerator() string {
	return uuid.NewString()
}

// Find the room using its roomId.
func (rh *RoomHandler) GetRoom(roomId string) (*Room, bool) {
	rh.mu.RLock()
	room, ok := rh.RoomIdToRoom[roomId]
	rh.mu.RUnlock()
	return room, ok
}

// Find the roomId for a given userId
func (rh *RoomHandler) RoomIdForUser(userId string) (string, bool) {
	rh.mu.RLock()
	roomId, ok := rh.UserIdToRoomId[userId]
	rh.mu.RUnlock()

	return roomId, ok
}

func (rh *RoomHandler) GetOtherPeersFromARoom(roomId string, userId string) ([]*models.Client, bool) {
	//get the room
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

// triggers on listening to "create-room" event
func (rh *RoomHandler) CreateRoom(createRoomMessage *models.CreateRoomMessage, client *models.Client) {
	roomId := rh.RoomIdGenerator()
	userId := createRoomMessage.UserId

	room := &Room{
		RoomId:         roomId,
		UserIdToClient: make(map[string]*models.Client),
	}

	client.RoomId = roomId
	client.UserId = userId

	// Add roomId->room
	rh.mu.Lock()
	rh.RoomIdToRoom[roomId] = room
	rh.mu.Unlock()

	// Add the userid->roomid
	rh.mu.Lock()
	rh.UserIdToRoomId[userId] = roomId
	rh.mu.Unlock()

	//Add UserId in the room map
	room.mu.Lock()
	room.UserIdToClient[userId] = client
	room.mu.Unlock()

	// emit the create-room event
	successMsg := models.CreateRoomSuccessMessage{
		Type:   "create-room-success",
		RoomId: roomId,
		UserId: userId,
	}

	client.SafeSend(client, successMsg)
}

// triggers on listening to "join-room" event
func (rh *RoomHandler) JoinRoom(joinRoomMessage *models.JoinRoomMessage, client *models.Client) {
	roomId := joinRoomMessage.RoomId
	userId := joinRoomMessage.UserId

	// Get the room from roomId and check if it even exists
	room, ok := rh.GetRoom(roomId)
	if !ok {
		// http.Error(w, "No such room with this roomId exists", http.StatusNotFound)
		// emit the error instead
		errMsg := models.ErrorMessage{
			Type:       "error",
			Error:      "No such room with this roomId exists",
			StatusCode: "ROOM_NOT_FOUND",
		}
		client.SafeSend(client, errMsg)
		return
	}

	room.mu.Lock()
	_, exists := room.UserIdToClient[userId]
	room.mu.Unlock()

	if exists {
		// http.Error(w, "user already exists", http.StatusBadRequest)
		// emit the error instead
		errMsg := models.ErrorMessage{
			Type:       "error",
			Error:      "User with userId already exists",
			StatusCode: "USER_EXISTS",
		}
		client.SafeSend(client, errMsg)
		return
	}

	//Use the client we get as the param only and update that only.
	client.UserId = userId
	client.RoomId = roomId

	//add the client in the room.
	room.mu.Lock()
	room.UserIdToClient[userId] = client
	room.mu.Unlock()

	//emit the join-room-success event to let the frontend know that the user was added to the room succesfully.
	successMsg := models.JoinRoomSuccessMessage{
		Type:   "join-room-success",
		RoomId: roomId,
		UserId: userId,
	}

	client.SafeSend(client, successMsg)
}

// triggers on listening to "leave-room" event
func (rh *RoomHandler) LeaveRoom(leaveRoomMessage *models.LeaveRoomMessage, client *models.Client) {
	roomId := leaveRoomMessage.RoomId
	userId := leaveRoomMessage.UserId

	//Check if the room even exists or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		// http.Error(w, "No such room with this roomId exists", http.StatusNotFound)
		// emit the error instead
		errMsg := models.ErrorMessage{
			Type:       "error",
			Error:      "No such room with this roomId exists",
			StatusCode: "ROOM_NOT_FOUND",
		}
		client.SafeSend(client, errMsg)
		return
	}

	//delete the client from the room
	room.mu.Lock()
	delete(room.UserIdToClient, userId)
	room.mu.Unlock()

	rh.CleanRoom(roomId)

	//emit the leave-room event
	successMsg := models.LeaveRoomSuccessMessage{
		Type:   "leave-room-success",
		RoomId: roomId,
		UserId: userId,
	}

	client.SafeSend(client, successMsg)
}

// route will be /view-room.
func (rh *RoomHandler) ViewRoom(w http.ResponseWriter, r *http.Request) {

}

func (rh *RoomHandler) CleanRoom(roomId string) {
	//first check if the room has any users left or not
	room, ok := rh.GetRoom(roomId)
	if !ok {
		log.Println("Room with this roomid does not exist")
		return
	}

	room.mu.RLock()
	isEmpty := len(room.UserIdToClient) == 0
	room.mu.RUnlock()

	if !isEmpty {
		log.Println("This room still has clients can not clean it")
		return
	}

	rh.mu.Lock()
	delete(rh.RoomIdToRoom, roomId)
	rh.mu.Unlock()
}
