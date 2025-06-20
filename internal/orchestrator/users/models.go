package users

import (
	"time"

	"github.com/google/uuid"
)

// User represents a human user in the system
type User struct {
	UUID      uuid.UUID  `json:"uuid"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name,omitempty"`
}
