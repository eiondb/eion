package users

import (
	"context"
)

// UserStore defines the interface for user storage operations
type UserStore interface {
	CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, userID string) error
}

// UserService defines the interface for user service operations
type UserService interface {
	UserStore
}
