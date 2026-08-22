// THIS PACKAGE HAS BEEN DEGRADED AND SHOULD NO LONGER BE USED.
// IT IS KEPT JUST FOR THE SAKE OF BUILDING THE BROKEN BLOCKS CORRECTLY THEN THIS WILL BE REMOVED

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
