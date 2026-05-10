package models

type CreateRoomRequest struct {
	RoomId string
	UserId string
}

type JoinRoomRequest struct {
	RoomId string
	UserId string
}

type LeaveRoomRequest struct {
	RoomId string
	UserId string
}
