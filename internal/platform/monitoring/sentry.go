package monitoring

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/config"
	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
)

func InitSentry(cfg config.SentryConfig) (func(), error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		TracesSampleRate: cfg.TracesSampleRate,
	}); err != nil {
		return func() {}, err
	}
	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

func NewFiberMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		hub := sentry.CurrentHub().Clone()
		if client := hub.Client(); client != nil {
			client.SetSDKIdentifier("sentry.go.fiber")
		}
		req := sentryRequest(c)
		transaction := sentry.StartTransaction(
			sentry.SetHubOnContext(c.UserContext(), hub),
			fmt.Sprintf("%s %s", c.Method(), c.Path()),
			sentry.ContinueTrace(hub, c.Get(sentry.SentryTraceHeader), c.Get(sentry.SentryBaggageHeader)),
			sentry.WithOpName("http.server"),
			sentry.WithTransactionSource(sentry.SourceURL),
			sentry.WithSpanOrigin(sentry.SpanOriginFiber),
		)
		c.SetUserContext(transaction.Context())
		sentryfiber.SetHubOnContext(c, hub)
		hub.Scope().SetRequest(req.WithContext(transaction.Context()))
		hub.Scope().SetRequestBody(c.Request().Body())

		defer func() {
			status := c.Response().StatusCode()
			transaction.Status = sentry.HTTPtoSpanStatus(status)
			transaction.SetData("http.response.status_code", status)
			transaction.Finish()
			if err := recover(); err != nil {
				hub.RecoverWithContext(c.UserContext(), err)
				panic(err)
			}
		}()

		return c.Next()
	}
}

func sentryRequest(c *fiber.Ctx) *http.Request {
	uri := c.Request().URI()
	rawURL := fmt.Sprintf("%s://%s%s", uri.Scheme(), uri.Host(), uri.RequestURI())
	parsed, err := url.Parse(rawURL)
	if err != nil {
		parsed = &url.URL{Path: c.Path(), RawQuery: string(uri.QueryString())}
	}
	req := &http.Request{
		Method:     c.Method(),
		URL:        parsed,
		Host:       c.Hostname(),
		Header:     make(http.Header),
		RemoteAddr: c.Context().RemoteAddr().String(),
	}
	c.Request().Header.VisitAll(func(key, value []byte) {
		name := string(key)
		if strings.EqualFold(name, fiber.HeaderCookie) || strings.EqualFold(name, fiber.HeaderAuthorization) {
			return
		}
		req.Header.Add(name, string(value))
	})
	req.Header.Set(fiber.HeaderHost, c.Hostname())
	return req
}

func EnrichFiberContext(c *fiber.Ctx) error {
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.Scope().SetTag("method", c.Method())
		hub.Scope().SetTag("path", c.Path())
		if rid, ok := c.Locals("requestid").(string); ok && rid != "" {
			hub.Scope().SetTag("request_id", rid)
		}
		if userID := c.Locals("user_id"); userID != nil {
			hub.Scope().SetUser(sentry.User{ID: fmt.Sprint(userID)})
		}
	}
	return c.Next()
}

func CaptureFiberError(c *fiber.Ctx, err error, status int) {
	if err == nil || status < fiber.StatusInternalServerError {
		return
	}
	capture := func(scope *sentry.Scope) {
		scope.SetTag("method", c.Method())
		scope.SetTag("path", c.Path())
		scope.SetTag("status", fmt.Sprintf("%d", status))
		if rid, ok := c.Locals("requestid").(string); ok && rid != "" {
			scope.SetTag("request_id", rid)
		}
		if userID := c.Locals("user_id"); userID != nil {
			scope.SetUser(sentry.User{ID: fmt.Sprint(userID)})
		}
		scope.SetContext("request", map[string]any{
			"url":    c.OriginalURL(),
			"method": c.Method(),
		})
	}
	if hub := sentryfiber.GetHubFromContext(c); hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			capture(scope)
			hub.CaptureException(err)
		})
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		capture(scope)
		sentry.CaptureException(err)
	})
}
