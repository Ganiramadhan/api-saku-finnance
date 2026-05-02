package routes

import (
	"github.com/ganiramadhan/starter-go/internal/middleware"
	"github.com/ganiramadhan/starter-go/internal/modules/auth"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

type Handlers struct {
	Auth *auth.Handler
	User *user.Handler
}

// Register wires all routes onto the Fiber app.
//
// Layout:
//   - Public :  /api/v1/...           (no auth)
//   - User   :  /api/v1/...           (auth required, any role)
//   - Admin  :  /api/v1/admin/...     (auth required, role=admin)
func Register(app *fiber.App, h Handlers, jwtMgr *jwt.Manager) {
	authRequired := middleware.AuthRequired(jwtMgr)

	v1 := app.Group("/api/v1")

	// ---------------- Public ----------------
	authPub := v1.Group("/auth")
	authPub.Post("/login", h.Auth.Login)
	authPub.Post("/register", h.Auth.Register)

	// ---------------- User (authenticated) ----------------
	authPriv := v1.Group("/auth", authRequired)
	authPriv.Post("/change-password", h.Auth.ChangePassword)

	usersMe := v1.Group("/users", authRequired)
	usersMe.Get("/me", h.User.Me)
	usersMe.Put("/me", h.User.UpdateMe)
	usersMe.Post("/upload-photo", h.User.UploadPhoto)
	usersMe.Delete("/me/photo", h.User.DeleteMyPhoto)

	// ---------------- Admin (auth + role=admin) ----------------
	admin := v1.Group("/admin", authRequired, middleware.RequireAdmin)
	adminUsers := admin.Group("/users")
	adminUsers.Get("", h.User.List)
	adminUsers.Get("/:id", h.User.Get)
	adminUsers.Post("", h.User.Create)
	adminUsers.Put("/:id", h.User.Update)
	adminUsers.Delete("/:id", h.User.Delete)
}
