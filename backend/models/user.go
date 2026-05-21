package models

import (
	"time"

	"gorm.io/gorm"
)

// User is the core identity record for every account in the system.
// Id is similar to _.id in mongoDB but we will use that ID as our UserId only.
// fields of email are indexed already for faster retrival.
// Primary keys are automatically indexed we don't need to index them seperately.
// CHANGE : changed DeletedAt field from *time.Time to gorm.DeletedAt to enable soft deletes automatically.
type User struct {
	ID           string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email        *string        `gorm:"type:text;uniqueIndex:idx_users_email,where:email IS NOT NULL" json:"email,omitempty"`
	PasswordHash *string        `gorm:"type:text" json:"-"` // nil for OAuth-only accounts
	FirstName    *string        `gorm:"type:text" json:"firstName,omitempty"`
	LastName     *string        `gorm:"type:text" json:"lastName,omitempty"`
	Role         string         `gorm:"type:text;not null;default:'user';index:idx_users_role" json:"role"`
	LastLoginAt  *time.Time     `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
