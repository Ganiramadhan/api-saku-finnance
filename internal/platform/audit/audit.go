package audit

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ctxKey struct{}

var actorCtxKey = ctxKey{}

type Actor struct {
	UserID    *uuid.UUID
	IP        string
	UserAgent string
	RequestID string
}

func Attach(c *fiber.Ctx, a Actor) {
	c.Locals(actorCtxKey, a)
}

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey, a)
}

func ActorFromContext(ctx context.Context) Actor {
	if ctx == nil {
		return Actor{}
	}
	if a, ok := ctx.Value(actorCtxKey).(Actor); ok {
		return a
	}
	return Actor{}
}

func ApplySessionContext(tx *gorm.DB, a Actor) error {
	actorID := ""
	if a.UserID != nil {
		actorID = a.UserID.String()
	}
	return tx.Exec(
		`SELECT set_config('app.current_user_id', ?, true),
		        set_config('app.client_ip', ?, true),
		        set_config('app.user_agent', ?, true),
		        set_config('app.request_id', ?, true)`,
		actorID, strings.TrimSpace(a.IP), a.UserAgent, a.RequestID,
	).Error
}

func WithTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	actor := ActorFromContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := ApplySessionContext(tx, actor); err != nil {
			return err
		}
		return fn(tx)
	})
}
