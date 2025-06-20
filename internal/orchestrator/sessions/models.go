package sessions

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a conversation session
type Session struct {
	UUID          uuid.UUID `json:"uuid"`
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	SessionName   *string   `json:"session_name,omitempty"`
	SessionTypeID string    `json:"session_type_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateSessionRequest represents a request to create a new session
type CreateSessionRequest struct {
	SessionID     string  `json:"session_id"`
	UserID        string  `json:"user_id"`
	SessionName   *string `json:"session_name,omitempty"`
	SessionTypeID string  `json:"session_type_id"`
}

// DeleteSessionRequest represents a request to delete a session
type DeleteSessionRequest struct {
	SessionID string `json:"session_id"`
}
