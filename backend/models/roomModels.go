package models

type CreateRoomRequest struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type JoinRoomRequest struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type LeaveRoomRequest struct {
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}

type ViewRoomRequest struct {
	RoomId string `json:"roomId"`
}

type ViewRoomResponse struct {
	OtherPeers []string `json:"otherPeers"`
}
