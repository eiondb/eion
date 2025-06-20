package users

import (
	"context"
	"fmt"
)

// UserServiceImpl implements the UserService interface
type UserServiceImpl struct {
	store UserStore
}

// NewUserService creates a new user service instance
func NewUserService(store UserStore) *UserServiceImpl {
	return &UserServiceImpl{
		store: store,
	}
}

// CreateUser creates a new user
func (s *UserServiceImpl) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	return s.store.CreateUser(ctx, req)
}

// DeleteUser deletes a user
func (s *UserServiceImpl) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	return s.store.DeleteUser(ctx, userID)
}
