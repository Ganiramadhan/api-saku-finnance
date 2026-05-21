package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/platform/mailer"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error
	GoogleLogin(ctx context.Context, req dto.GoogleLoginRequest) (*dto.AuthResponse, error)
	ForgotPassword(ctx context.Context, req dto.ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error
}

type service struct {
	users          user.Repository
	jwt            *jwt.Manager
	googleClientID string
	httpClient     *http.Client
	mailer         mailer.Mailer
}

func NewService(users user.Repository, j *jwt.Manager, googleClientID string, mailer mailer.Mailer) Service {
	return &service{
		users:          users,
		jwt:            j,
		googleClientID: googleClientID,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		mailer:         mailer,
	}
}

func (s *service) Login(_ context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	u, err := s.users.FindByEmail(req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if strings.EqualFold(u.Status, "suspended") {
		return nil, domain.ErrInvalidCredentials
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		Token: token,
		User:  dto.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role},
	}, nil
}

func (s *service) Register(_ context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	if existing, _ := s.users.FindByEmail(req.Email); existing != nil {
		return nil, domain.ErrAlreadyExists
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := domain.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
		Role:     "user",
	}
	if err := s.users.Create(&u); err != nil {
		return nil, err
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		Token: token,
		User:  dto.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role},
	}, nil
}

func (s *service) ChangePassword(_ context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error {
	u, err := s.users.FindByID(userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.CurrentPassword)); err != nil {
		return domain.ErrInvalidCredentials
	}
	if err := validateStrongPassword(req.NewPassword); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return s.users.Update(u)
}

func (s *service) ForgotPassword(_ context.Context, req dto.ForgotPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrEmailNotRegistered
		}
		return err
	}
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return err
	}
	code := (int(randomBytes[0])<<16 | int(randomBytes[1])<<8 | int(randomBytes[2])) % 1_000_000
	otp := fmt.Sprintf("%06d", code)
	hashedOTP, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	u.ResetOTP = string(hashedOTP)
	u.ResetOTPExpiresAt = &expires
	if err := s.users.Update(u); err != nil {
		return err
	}
	if s.mailer != nil {
		body := forgotPasswordEmailHTML(u.Name, otp)
		go func() {
			if err := s.mailer.Send(email, "Kode OTP pemulihan password SAKU", body); err != nil {
				log.Printf("auth: send forgot password otp email failed: %v", err)
			}
		}()
	}
	return nil
}
func forgotPasswordEmailHTML(name, otp string) string {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "Pengguna SAKU"
	}

	expiredAt := time.Now().Add(10 * time.Minute).Format("15:04 WIB")

	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<body style="margin:0;padding:0;background:#f3f6fb;font-family:Inter,Aptos,-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;color:#0f172a;">
  <div style="max-width:640px;margin:0 auto;padding:48px 20px;">
    <div style="background:#ffffff;border:1px solid #e5eaf1;border-radius:28px;overflow:hidden;box-shadow:0 24px 70px rgba(15,23,42,.08);">

      <div style="padding:34px 36px 28px;background:#ffffff;border-bottom:1px solid #eef2f7;">
        <div style="display:inline-flex;align-items:center;gap:12px;">
          <div style="width:46px;height:46px;border-radius:15px;background:#0f172a;color:#ffffff;text-align:center;line-height:46px;font-size:19px;font-weight:900;">
            S
          </div>
          <div>
            <div style="font-size:17px;font-weight:900;color:#0f172a;letter-spacing:-.02em;">SAKU</div>
            <div style="font-size:12px;color:#64748b;margin-top:2px;">Account Security</div>
          </div>
        </div>

        <h1 style="margin:30px 0 10px;font-size:28px;line-height:1.25;color:#0f172a;letter-spacing:-.04em;">
          Verifikasi reset password
        </h1>

        <p style="margin:0;color:#475569;font-size:15px;line-height:1.75;">
          Halo <strong style="color:#0f172a;">%s</strong>, gunakan kode OTP berikut untuk melanjutkan proses reset password akun SAKU Anda.
        </p>
      </div>

      <div style="padding:36px;">
        <div style="border:1px solid #dbe3ef;border-radius:24px;background:#f8fafc;padding:30px;text-align:center;">
          <div style="font-size:12px;font-weight:800;color:#64748b;letter-spacing:.14em;text-transform:uppercase;">
            Kode OTP Anda
          </div>

          <div style="margin:18px auto 0;display:inline-block;background:#ffffff;border:1px solid #e2e8f0;border-radius:18px;padding:18px 24px;">
            <span style="font-size:40px;font-weight:900;letter-spacing:.22em;color:#0f172a;line-height:1;">
              %s
            </span>
          </div>

          <div style="margin:22px auto 0;max-width:340px;border-radius:16px;background:#eff6ff;border:1px solid #bfdbfe;padding:14px 16px;">
            <div style="font-size:13px;color:#1e40af;line-height:1.6;">
              Kode ini berlaku selama <strong>10 menit</strong><br>
              Kedaluwarsa sekitar pukul <strong>%s</strong>
            </div>
          </div>
        </div>

        <div style="margin-top:24px;border:1px solid #fde68a;background:#fffbeb;border-radius:18px;padding:16px 18px;">
          <p style="margin:0;color:#92400e;font-size:13px;line-height:1.7;">
            Demi keamanan, jangan bagikan kode ini kepada siapa pun. SAKU tidak pernah meminta OTP melalui chat, telepon, atau email.
          </p>
        </div>

        <p style="margin:24px 0 0;color:#64748b;font-size:13px;line-height:1.7;">
          Jika Anda tidak meminta reset password, abaikan email ini. Password akun Anda tidak akan berubah.
        </p>
      </div>
    </div>

    <p style="margin:20px 0 0;text-align:center;color:#94a3b8;font-size:12px;line-height:1.6;">
      Email ini dikirim otomatis oleh SAKU. Mohon tidak membalas email ini.
    </p>
  </div>
</body>
</html>`,
		html.EscapeString(displayName),
		html.EscapeString(otp),
		expiredAt,
	)
}
func (s *service) ResetPassword(_ context.Context, req dto.ResetPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrEmailNotRegistered
		}
		return err
	}
	if u.ResetOTP == "" || u.ResetOTPExpiresAt == nil || time.Now().UTC().After(*u.ResetOTPExpiresAt) {
		return domain.ErrInvalidOTP
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.ResetOTP), []byte(strings.TrimSpace(req.OTP))); err != nil {
		return domain.ErrInvalidOTP
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		return nil
	}
	if err := validateStrongPassword(req.NewPassword); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	u.ResetOTP = ""
	u.ResetOTPExpiresAt = nil
	return s.users.Update(u)
}

func validateStrongPassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password baru minimal 8 karakter")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("password baru harus mengandung huruf besar, huruf kecil, dan angka")
	}
	return nil
}

func (s *service) GoogleLogin(ctx context.Context, req dto.GoogleLoginRequest) (*dto.AuthResponse, error) {
	claims, err := s.verifyGoogleIDToken(ctx, req.IDToken)
	if err != nil {
		log.Printf("auth: google token verification failed: %v", err)
		return nil, domain.ErrInvalidCredentials
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		return nil, domain.ErrInvalidCredentials
	}

	u, err := s.users.FindByEmail(email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if u == nil {
		if strings.ToLower(strings.TrimSpace(req.Mode)) != "register" {
			return nil, domain.ErrEmailNotRegistered
		}
		randomBytes := make([]byte, 24)
		if _, rerr := rand.Read(randomBytes); rerr != nil {
			return nil, rerr
		}
		hashed, herr := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(randomBytes)), bcrypt.DefaultCost)
		if herr != nil {
			return nil, herr
		}
		newUser := domain.User{
			Name:     coalesce(claims.Name, claims.GivenName, email),
			Email:    email,
			Password: string(hashed),
			Photo:    claims.Picture,
			Role:     "user",
			Status:   "active",
		}
		if err := s.users.Create(&newUser); err != nil {
			return nil, err
		}
		u = &newUser
	} else if u.Photo == "" && claims.Picture != "" {
		u.Photo = claims.Picture
		if err := s.users.Update(u); err != nil {
			return nil, err
		}
	}
	if strings.EqualFold(u.Status, "suspended") {
		return nil, domain.ErrInvalidCredentials
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		Token: token,
		User: dto.UserResponse{
			ID:       u.ID,
			Name:     u.Name,
			Email:    u.Email,
			Role:     u.Role,
			Photo:    u.Photo,
			PhotoURL: externalPhotoURL(u.Photo),
		},
	}, nil
}

func externalPhotoURL(photo string) string {
	if strings.HasPrefix(photo, "http://") || strings.HasPrefix(photo, "https://") {
		return photo
	}
	return ""
}

type googleClaims struct {
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	Picture       string `json:"picture"`
}

func (s *service) verifyGoogleIDToken(ctx context.Context, idToken string) (*googleClaims, error) {
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: token verification failed: status %d", res.StatusCode)
	}
	var c googleClaims
	if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
		return nil, err
	}
	if s.googleClientID != "" && c.Aud != s.googleClientID {
		return nil, errors.New("google: invalid audience")
	}
	if c.EmailVerified != "true" {
		return nil, errors.New("google: email not verified")
	}
	return &c, nil
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
