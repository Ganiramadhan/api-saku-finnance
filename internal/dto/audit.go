package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLogResponse struct {
	ID       uuid.UUID  `json:"id"`
	ActorID  *uuid.UUID `json:"actor_id"`
	DBUser   string     `json:"db_user"`
	Action   string     `json:"action"`
	Table    string     `json:"table_name"`
	RecordID string     `json:"record_id"`

	OldData     json.RawMessage `json:"old_data,omitempty"`
	NewData     json.RawMessage `json:"new_data,omitempty"`
	ChangedData json.RawMessage `json:"changed_data,omitempty"`

	IPAddress        *string   `json:"ip_address,omitempty"`
	UserAgent        *string   `json:"user_agent,omitempty"`
	RequestID        *string   `json:"request_id,omitempty"`
	IsDirectDBChange bool      `json:"is_direct_db_change"`
	CreatedAt        time.Time `json:"created_at"`
}

type AuditLogListQuery struct {
	Table      string `query:"table"`
	RecordID   string `query:"record_id"`
	ActorID    string `query:"actor_id"`
	RequestID  string `query:"request_id"`
	Action     string `query:"action"`
	DirectOnly bool   `query:"direct_only"`
	From       string `query:"from"`
	To         string `query:"to"`
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
}
