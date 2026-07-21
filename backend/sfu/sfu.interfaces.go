package sfu

import "backend/participant"

type RoomManager interface {
	RoomIdForUser(string) (string, bool)
	GetClientFromUserId(string) (*participant.Client, bool)
	GetOtherPeersFromARoom(string, string) ([]*participant.Client, bool)
}
