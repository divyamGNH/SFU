package auth

import (
	"backend/models"
)

type AuthRepository interface {
	CreateUser(user *models.User) (*models.User, error)
	GetUserByID(id string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	UpdateUser(id string, updates map[string]interface{}) (*models.User, error)
}
