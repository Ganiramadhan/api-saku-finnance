package audit

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

func TestActorFromContext_NoActorAttached(t *testing.T) {
	got := ActorFromContext(context.Background())
	if got.UserID != nil || got.IP != "" || got.UserAgent != "" || got.RequestID != "" {
		t.Fatalf("expected zero-value Actor, got %+v", got)
	}
}

func TestActorFromContext_NilContext(t *testing.T) {
	got := ActorFromContext(nil) //nolint:staticcheck // exercising the nil-safety guard explicitly
	if got.UserID != nil {
		t.Fatalf("expected zero-value Actor for nil context, got %+v", got)
	}
}

func TestAttach_RoundTripsThroughFasthttpValue(t *testing.T) {
	app := fiber.New()
	fctx := &fasthttp.RequestCtx{}
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)

	uid := uuid.New()
	want := Actor{
		UserID:    &uid,
		IP:        "203.0.113.7",
		UserAgent: "saku-mobile/1.0",
		RequestID: "req-123",
	}
	Attach(c, want)

	got := ActorFromContext(c.Context())
	if got.UserID == nil || *got.UserID != uid {
		t.Fatalf("UserID = %v, want %v", got.UserID, uid)
	}
	if got.IP != want.IP || got.UserAgent != want.UserAgent || got.RequestID != want.RequestID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
