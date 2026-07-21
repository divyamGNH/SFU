package types

// HTTP messages

type CreateRoomResponse struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type JoinRoomResponse struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomResponse struct {
	Message string `json:"message"`
}

type ViewRoomResponse struct {
	OtherPeers []string `json:"otherPeers"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type RoomWriteJSON struct {
}
