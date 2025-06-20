package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// UserSchema represents the users table schema in PostgreSQL
type UserSchema struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	UUID      uuid.UUID  `bun:"uuid,pk,type:uuid,default:gen_random_uuid()" json:"uuid"`
	UserID    string     `bun:"user_id,notnull,unique" json:"user_id"`
	Name      *string    `bun:"name" json:"name,omitempty"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// UserStoreImpl implements the UserStore interface
type UserStoreImpl struct {
	db *bun.DB
}

// NewUserStore creates a new user store instance
func NewUserStore(db *bun.DB) *UserStoreImpl {
	return &UserStoreImpl{
		db: db,
	}
}

// CreateUser creates a new user
func (s *UserStoreImpl) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id cannot be empty")
	}

	user := &User{
		UUID:      uuid.New(),
		UserID:    req.UserID,
		Name:      req.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	userSchema := UserToUserSchema(user)

	_, err := s.db.NewInsert().
		Model(&userSchema).
		Returning("*").
		Exec(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") ||
			strings.Contains(err.Error(), "users_user_id_key") {
			return nil, fmt.Errorf("user already exists with user_id: %s", req.UserID)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	*user = *UserSchemaToUser(userSchema)
	return user, nil
}

// DeleteUser deletes a user (soft delete)
func (s *UserStoreImpl) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	_, err := s.db.NewUpdate().
		Model((*UserSchema)(nil)).
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Set("deleted_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Helper conversion functions
func UserSchemaToUser(schema UserSchema) *User {
	user := &User{
		UUID:      schema.UUID,
		UserID:    schema.UserID,
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		DeletedAt: schema.DeletedAt,
	}

	if schema.Name != nil {
		user.Name = *schema.Name
	}

	return user
}

func UserToUserSchema(user *User) UserSchema {
	var name *string
	if user.Name != "" {
		name = &user.Name
	}

	return UserSchema{
		UUID:      user.UUID,
		UserID:    user.UserID,
		Name:      name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		DeletedAt: user.DeletedAt,
	}
}
