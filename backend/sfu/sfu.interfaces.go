package sfu

import "backend/models"

type RoomManager interface {
	RoomIdForUser(string) (string, bool)
	GetClientFromUserId(string) (*models.Client, bool)
	GetOtherPeersFromARoom(string, string) ([]*models.Client, bool)
}
