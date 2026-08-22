package room

import (
	"backend/logger"
	"backend/participant"
)

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

	room.Mu.RLock()
	for id, client := range room.UserIdToClient {
		if id == userId {
			continue
		}
		otherUsers = append(otherUsers, client)
	}
	room.Mu.RUnlock()

	return otherUsers, true
}
