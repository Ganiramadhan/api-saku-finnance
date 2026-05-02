package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func newAppWithErrorHandler() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
}

func decode(t *testing.T, body io.Reader) dto.APIResponse {
	t.Helper()
	var resp dto.APIResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestAuthRequired(t *testing.T) {
	mgr := jwt.New("test-secret", time.Hour)
	uid := uuid.New()
	validToken, _ := mgr.Generate(uid, "john@example.com", "user")

	app := newAppWithErrorHandler()
	app.Use(AuthRequired(mgr))
	app.Get("/me", func(c *fiber.Ctx) error {
		got, _ := c.Locals(httpx.LocalUserID).(uuid.UUID)
		if got != uid {
			t.Errorf("UserID local = %s, want %s", got, uid)
		}
		return c.SendStatus(http.StatusOK)
	})

	cases := []struct {
		name   string
		header string
		status int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"missing prefix", validToken, http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
		{"invalid token", "Bearer not-a-jwt", http.StatusUnauthorized},
		{"valid", "Bearer " + validToken, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	app := newAppWithErrorHandler()
	app.Get("/admin", func(c *fiber.Ctx) error {
		c.Locals(httpx.LocalRole, c.Query("role"))
		return RequireAdmin(c)
	}, func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	t.Run("non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin?role=user", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d", resp.StatusCode)
		}
	})
	t.Run("admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin?role=admin", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d", resp.StatusCode)
		}
	})
}

func TestErrorHandler_DomainErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"already exists", domain.ErrAlreadyExists, http.StatusConflict},
		{"invalid credentials", domain.ErrInvalidCredentials, http.StatusUnauthorized},
		{"unauthorized", domain.ErrUnauthorized, http.StatusUnauthorized},
		{"invalid input", domain.ErrInvalidInput, http.StatusBadRequest},
		{"unknown -> 500", errors.New("boom"), http.StatusInternalServerError},
		{"fiber error passthrough", fiber.NewError(http.StatusTeapot, "tea"), http.StatusTeapot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newAppWithErrorHandler()
			app.Get("/", func(c *fiber.Ctx) error { return tc.err })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			resp, _ := app.Test(req)
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			body := decode(t, resp.Body)
			if body.Status != "error" {
				t.Errorf("envelope status = %q", body.Status)
			}
			if body.Code != tc.status {
				t.Errorf("envelope code = %d, want %d", body.Code, tc.status)
			}
		})
	}
}
