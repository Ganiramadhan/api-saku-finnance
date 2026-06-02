package auth

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/constants"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
)

const sessionCookieName = "saku_session"

type Handler struct {
	validator *validator.Validator
	service   Service
	turnstile *turnstileVerifier
}

func NewHandler(s Service, v *validator.Validator, turnstileSecret string) *Handler {
	return &Handler{service: s, validator: v, turnstile: newTurnstileVerifier(turnstileSecret)}
}

// Login godoc
// @Summary   Login
// @Tags      Auth
// @Accept    json
// @Produce   json
// @Param     request  body  dto.LoginRequest  true  "Credentials"
// @Success   200  {object}  dto.APIResponse{data=dto.AuthResponse}
// @Failure   401  {object}  dto.APIResponse
// @Router    /api/v1/auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.turnstile.Verify(c.Context(), req.TurnstileToken, c.IP()); err != nil {
		return err
	}
	// println("Login attempt for email:", req.Email) // Debug log
	resp, err := h.service.Login(c.Context(), req)
	if err != nil {
		return err
	}
	setSessionCookie(c, resp.Token)
	return httpx.OK(c, constants.MsgLogin, resp)
}

// Register godoc
// @Summary   Register a new account
// @Tags      Auth
// @Accept    json
// @Produce   json
// @Param     request  body  dto.RegisterRequest  true  "Registration data"
// @Success   201  {object}  dto.APIResponse{data=dto.AuthResponse}
// @Failure   409  {object}  dto.APIResponse
// @Router    /api/v1/auth/register [post]
func (h *Handler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.turnstile.Verify(c.Context(), req.TurnstileToken, c.IP()); err != nil {
		return err
	}
	resp, err := h.service.Register(c.Context(), req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgRegister, resp)
}

func (h *Handler) VerifyRegistration(c *fiber.Ctx) error {
	var req dto.VerifyRegistrationRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	resp, err := h.service.VerifyRegistration(c.Context(), req)
	if err != nil {
		return err
	}
	setSessionCookie(c, resp.Token)
	return httpx.OK(c, "Account verified successfully.", resp)
}

func (h *Handler) ResendRegistrationOTP(c *fiber.Ctx) error {
	var req dto.ResendRegistrationOTPRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.service.ResendRegistrationOTP(c.Context(), req); err != nil {
		return err
	}
	return httpx.OK(c, "Verification code sent.", nil)
}

// ForgotPassword godoc
// @Summary   Validate account for password recovery
// @Tags      Auth
// @Accept    json
// @Produce   json
// @Param     request  body  dto.ForgotPasswordRequest  true  "Email"
// @Success   200  {object}  dto.APIResponse
// @Router    /api/v1/auth/forgot-password [post]
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if !req.Resend {
		if err := h.turnstile.Verify(c.Context(), req.TurnstileToken, c.IP()); err != nil {
			return err
		}
	} else if req.TurnstileToken != "" {
		if err := h.turnstile.Verify(c.Context(), req.TurnstileToken, c.IP()); err != nil {
			return err
		}
	}
	if err := h.service.ForgotPassword(c.Context(), req); err != nil {
		return err
	}
	return httpx.OK(c, "Kode OTP pemulihan password sudah dikirim ke email.", nil)
}

func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.service.ResetPassword(c.Context(), req); err != nil {
		return err
	}
	return httpx.OK(c, "Password berhasil diperbarui.", nil)
}

// ChangePassword godoc
// @Summary   Change my password
// @Tags      Auth
// @Accept    json
// @Produce   json
// @Param     request  body  dto.ChangePasswordRequest  true  "Old + new password"
// @Success   200  {object}  dto.APIResponse
// @Failure   401  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/auth/change-password [post]
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.ChangePasswordRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.service.ChangePassword(c.Context(), uid, req); err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgChangePassword, nil)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	clearSessionCookie(c)
	return httpx.OK(c, "Logged out successfully.", nil)
}

// GoogleLogin godoc
// @Summary   Login with Google
// @Tags      Auth
// @Accept    json
// @Produce   json
// @Param     request  body  dto.GoogleLoginRequest  true  "Google ID token"
// @Success   200  {object}  dto.APIResponse{data=dto.AuthResponse}
// @Failure   401  {object}  dto.APIResponse
// @Router    /api/v1/auth/google [post]
func (h *Handler) GoogleLogin(c *fiber.Ctx) error {
	var req dto.GoogleLoginRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	resp, err := h.service.GoogleLogin(c.Context(), req)
	if err != nil {
		return err
	}
	setSessionCookie(c, resp.Token)
	return httpx.OK(c, constants.MsgLogin, resp)
}

func setSessionCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

func clearSessionCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
