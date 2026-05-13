package sfu

import "backend/models"

type RoomManager interface {
	RoomIdForUser(string) (string, bool)
	GetOtherPeersFromARoom(string, string) ([]*models.Client, bool)
}
