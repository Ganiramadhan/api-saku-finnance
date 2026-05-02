package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestParseUUID(t *testing.T) {
	app := fiber.New()
	app.Get("/u/:id", func(c *fiber.Ctx) error {
		id, err := ParseUUID(c, "id")
		if err != nil {
			return err
		}
		return c.SendString(id.String())
	})

	t.Run("valid", func(t *testing.T) {
		id := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/u/"+id.String(), nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d", resp.StatusCode)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/u/not-a-uuid", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

type bindBody struct {
	Name string `json:"name" validate:"required,min=2"`
}

func TestBind(t *testing.T) {
	v := validator.New()
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		var b bindBody
		if err := Bind(c, v, &b); err != nil {
			return err
		}
		return c.SendString(b.Name)
	})

	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"ok", `{"name":"Jane"}`, http.StatusOK},
		{"invalid json", `{"name":`, http.StatusBadRequest},
		{"validation fails", `{"name":""}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
		})
	}
}

func TestUserID_MissingLocals(t *testing.T) {
	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		_, err := UserID(c)
		return err
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestUserID_AndRole_WithLocals(t *testing.T) {
	uid := uuid.New()
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(LocalUserID, uid)
		c.Locals(LocalRole, "admin")
		return c.Next()
	})
	app.Get("/me", func(c *fiber.Ctx) error {
		got, err := UserID(c)
		if err != nil {
			return err
		}
		if got != uid {
			t.Errorf("UserID = %s, want %s", got, uid)
		}
		if Role(c) != "admin" {
			t.Errorf("Role = %s", Role(c))
		}
		return c.SendStatus(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
