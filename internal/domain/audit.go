package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ActorID     *uuid.UUID `gorm:"type:uuid;column:actor_id"`
	DBUser      string     `gorm:"column:db_user"`
	Action      string     `gorm:"column:action"`
	Table       string     `gorm:"column:table_name"`
	RecordID    string     `gorm:"column:record_id"`
	OldData     *string    `gorm:"type:jsonb;column:old_data"`
	NewData     *string    `gorm:"type:jsonb;column:new_data"`
	ChangedData *string    `gorm:"type:jsonb;column:changed_data"`

	IPAddress *string `gorm:"column:ip_address"`
	UserAgent *string `gorm:"column:user_agent"`
	RequestID *string `gorm:"column:request_id"`

	IsDirectDBChange bool `gorm:"column:is_direct_db_change"`
	TransactionID    int64
	StatementTS      time.Time `gorm:"column:statement_ts"`
	CreatedAt        time.Time

	Actor *User `gorm:"foreignKey:ActorID;references:ID"`
}

func (AuditLog) TableName() string { return "audit_logs" }
