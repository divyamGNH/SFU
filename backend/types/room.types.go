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

type PeerState struct {
	UserId    string `json:"userId"`
	AudioBool bool   `json:"audioBool"`
	VideoBool bool   `json:"videoBool"`
}

type ViewRoomResponse struct {
	OtherPeers []PeerState `json:"otherPeers"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type RoomWriteJSON struct {
}
