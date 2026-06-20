package storage

import (
	"backend/models"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailAlreadyExists  = errors.New("email already registered")
	ErrUserBanned          = errors.New("user is banned")
	ErrUserInactive        = errors.New("user account is inactive")
	ErrRefreshTokenInvalid = errors.New("refresh token not found or revoked")
	ErrRefreshTokenExpired = errors.New("refresh token has expired")
	ErrOAuthProviderPaired = errors.New("oauth provider already linked to another account")
)

type authRepo struct {
	db *gorm.DB
}

func NewAuthRepo(db *gorm.DB) *authRepo {
	return &authRepo{
		db: db,
	}
}

func (ar *authRepo) CreateUser(user *models.User) (*models.User, error) {
	//Create the user by passing the struct/interface in the function.
	err := ar.db.Create(user).Error
	if err != nil {
		//Return the error simply the logging and handling will be done by the superior function that uses this.
		return nil, err
	}

	//Return the user made.
	return user, nil
}

func (ar *authRepo) GetUserById(id string) (*models.User, error) {
	var u *models.User
	err := ar.db.Where("id = ?", id).First(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return u, nil
}

func (ar *authRepo) GetUserByEmail(id string) (*models.User, error) {
	var u *models.User
	err := ar.db.Where("email = ?", id).First(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, err
}

//Update user function

func MigrateAuthModels(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
	)
}
