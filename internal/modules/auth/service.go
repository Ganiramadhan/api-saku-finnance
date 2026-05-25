package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

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
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error)
	VerifyRegistration(ctx context.Context, req dto.VerifyRegistrationRequest) (*dto.AuthResponse, error)
	ResendRegistrationOTP(ctx context.Context, req dto.ResendRegistrationOTPRequest) error
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
	email := sanitizeEmail(req.Email)
	u, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if strings.EqualFold(u.Status, "pending_verification") {
		return nil, domain.ErrAccountNotVerified
	}
	if !strings.EqualFold(u.Status, "active") {
		return nil, domain.ErrInvalidCredentials
	}
	if u.Referral == nil || u.Referral.Code == "" {
		code, err := s.generateReferralCode()
		if err != nil {
			return nil, err
		}
		ref, err := s.users.EnsureReferralCode(u.ID, code)
		if err != nil {
			return nil, err
		}
		u.Referral = ref
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{Token: token, User: authUserResponse(u)}, nil
}

func (s *service) Register(_ context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	name := sanitizeName(req.Name)
	email := sanitizeEmail(req.Email)
	if name == "" || email == "" {
		return nil, domain.ErrInvalidInput
	}
	if !isGmailAddress(email) {
		return nil, domain.ErrGmailRequired
	}
	if existing, _ := s.users.FindByEmail(email); existing != nil {
		if strings.EqualFold(existing.Status, "pending_verification") {
			if err := s.sendRegistrationOTP(existing); err != nil {
				return nil, err
			}
			return &dto.RegisterResponse{Email: email, ExpiresIn: 5 * 60}, nil
		}
		return nil, domain.ErrAlreadyExists
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	ownReferralCode, err := s.generateReferralCode()
	if err != nil {
		return nil, err
	}
	u := domain.User{
		Name:         name,
		Email:        email,
		Password:     string(hashed),
		AuthProvider: "password",
		Role:         "user",
		Status:       "pending_verification",
	}
	if err := s.users.Create(&u); err != nil {
		return nil, err
	}
	if _, err := s.users.EnsureReferralCode(u.ID, ownReferralCode); err != nil {
		return nil, err
	}
	if err := s.sendRegistrationOTP(&u); err != nil {
		return nil, err
	}
	return &dto.RegisterResponse{Email: email, ExpiresIn: 5 * 60}, nil
}

func (s *service) VerifyRegistration(_ context.Context, req dto.VerifyRegistrationRequest) (*dto.AuthResponse, error) {
	email := sanitizeEmail(req.Email)
	u, err := s.users.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(u.Status, "active") {
		return nil, domain.ErrAlreadyExists
	}
	otp, err := s.users.FindOTP(u.ID, "email_verification")
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidOTP
		}
		return nil, err
	}
	if otp.CodeHash == "" || time.Now().UTC().After(otp.ExpiresAt) {
		return nil, domain.ErrInvalidOTP
	}
	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash), []byte(strings.TrimSpace(req.OTP))); err != nil {
		return nil, domain.ErrInvalidOTP
	}
	u.Status = "active"
	if err := s.users.Update(u); err != nil {
		return nil, err
	}
	if err := s.users.ClearOTP(u.ID, "email_verification"); err != nil {
		return nil, err
	}
	if u.Referral == nil || u.Referral.Code == "" {
		code, err := s.generateReferralCode()
		if err != nil {
			return nil, err
		}
		ref, err := s.users.EnsureReferralCode(u.ID, code)
		if err != nil {
			return nil, err
		}
		u.Referral = ref
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{Token: token, User: authUserResponse(u)}, nil
}

func (s *service) ResendRegistrationOTP(_ context.Context, req dto.ResendRegistrationOTPRequest) error {
	email := sanitizeEmail(req.Email)
	u, err := s.users.FindByEmail(email)
	if err != nil {
		return err
	}
	if strings.EqualFold(u.Status, "active") {
		return domain.ErrAlreadyExists
	}
	return s.sendRegistrationOTP(u)
}

func (s *service) sendRegistrationOTP(u *domain.User) error {
	otp, err := generateOTP()
	if err != nil {
		return err
	}
	hashedOTP, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(5 * time.Minute)
	if err := s.users.UpsertOTP(u.ID, "email_verification", string(hashedOTP), expires); err != nil {
		return err
	}
	if s.mailer != nil {
		body := registrationEmailHTML(u.Name, u.Email, otp)
		if err := s.mailer.Send(u.Email, "Verify your SAKU account", body); err != nil {
			log.Printf("auth: queue registration otp email failed: %v", err)
		}
	}
	return nil
}

func generateOTP() (string, error) {
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	code := (int(randomBytes[0])<<16 | int(randomBytes[1])<<8 | int(randomBytes[2])) % 1_000_000
	return fmt.Sprintf("%06d", code), nil
}

func (s *service) ChangePassword(_ context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error {
	u, err := s.users.FindByID(userID)
	if err != nil {
		return err
	}
	isGoogleOnly := strings.EqualFold(u.AuthProvider, "google")
	if !isGoogleOnly {
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.CurrentPassword)); err != nil {
			return domain.ErrInvalidCredentials
		}
	}
	if err := validateStrongPassword(req.NewPassword); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	u.AuthProvider = "password"
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
	otp, err := generateOTP()
	if err != nil {
		return err
	}
	hashedOTP, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if err := s.users.UpsertResetOTP(u.ID, string(hashedOTP), expires); err != nil {
		return err
	}
	if s.mailer != nil {
		body := forgotPasswordEmailHTML(u.Name, email, otp)
		if err := s.mailer.Send(email, "Your SAKU password reset OTP", body); err != nil {
			log.Printf("auth: queue forgot password otp email failed: %v", err)
		}
	}
	return nil
}

func registrationEmailHTML(name, email, otp string) string {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "SAKU user"
	}
	accountEmail := strings.TrimSpace(email)
	if accountEmail == "" {
		accountEmail = "-"
	}
	cleanOTP := strings.TrimSpace(otp)
	if cleanOTP == "" {
		cleanOTP = "------"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>Verify SAKU Account</title>
</head>
<body style="margin:0;padding:0;background:#f8fafc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Inter,Arial,sans-serif;color:#0f172a;">
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;font-size:1px;color:#f8fafc;line-height:1px;">
    Your SAKU account verification code is valid for 5 minutes.
  </div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f8fafc;">
    <tr>
      <td align="center" style="padding:28px 16px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:520px;background:#ffffff;border:1px solid #e2e8f0;border-radius:22px;overflow:hidden;box-shadow:0 16px 40px rgba(15,23,42,.08);">
          <tr>
            <td style="height:7px;background:#2563eb;line-height:7px;font-size:0;">&nbsp;</td>
          </tr>
          <tr>
            <td style="padding:24px 26px 0;">
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td style="vertical-align:middle;">
                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                      <tr>
                        <td style="width:42px;height:42px;border-radius:12px;background:#2563eb;text-align:center;vertical-align:middle;">
                          <span style="font-size:20px;font-weight:900;line-height:42px;color:#ffffff;">S</span>
                        </td>
                        <td style="padding-left:12px;vertical-align:middle;">
                          <div style="font-size:18px;font-weight:900;color:#0f172a;letter-spacing:.02em;">SAKU</div>
                          <div style="font-size:10px;font-weight:800;letter-spacing:.12em;color:#2563eb;text-transform:uppercase;">Account Verification</div>
                        </td>
                      </tr>
                    </table>
                  </td>
                  <td align="right" style="vertical-align:middle;">
                    <span style="display:inline-block;padding:7px 12px;border-radius:999px;background:#eff6ff;border:1px solid #bfdbfe;font-size:10px;font-weight:800;color:#1d4ed8;text-transform:uppercase;letter-spacing:.06em;">
                      Register
                    </span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:24px 26px 26px;">
              <h1 style="margin:0 0 12px;font-size:20px;font-weight:900;line-height:1.35;color:#0f172a;">Verify your account</h1>
              <p style="margin:0 0 22px;font-size:14px;line-height:1.75;color:#475569;">
                Hi <strong style="color:#0f172a;">%s</strong>, use this OTP to complete your SAKU registration.
              </p>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f8fbff;border:1px solid #dbeafe;border-radius:18px;margin-bottom:16px;">
                <tr>
                  <td align="center" style="padding:22px;">
                    <div style="font-size:10px;font-weight:900;letter-spacing:.18em;color:#2563eb;margin-bottom:12px;text-transform:uppercase;">OTP Code</div>
                    <span style="display:inline-block;padding:13px 24px;border-radius:13px;background:#ffffff;border:1px solid #dbeafe;font-size:32px;letter-spacing:.20em;font-weight:900;color:#1e3a5f;font-family:'Courier New',Courier,monospace;line-height:1;">%s</span>
                    <div style="margin-top:13px;font-size:12px;font-weight:800;color:#2563eb;">Valid for 5 minutes</div>
                  </td>
                </tr>
              </table>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f0fdf4;border:1px solid #bbf7d0;border-radius:13px;margin-bottom:14px;">
                <tr>
                  <td style="padding:13px;">
                    <div style="font-size:10px;font-weight:900;color:#15803d;margin-bottom:4px;letter-spacing:.08em;text-transform:uppercase;">Account Email</div>
                    <div style="font-size:13px;font-weight:800;color:#166534;word-break:break-word;">%s</div>
                  </td>
                </tr>
              </table>
              <div style="background:#fff7ed;border-left:3px solid #f97316;border-radius:11px;padding:13px;margin-bottom:16px;">
                <p style="margin:0;font-size:12px;line-height:1.7;color:#9a3412;">
                  Never share this code. If you did not create a SAKU account, ignore this email.
                </p>
              </div>
              <p style="margin:0;font-size:11px;line-height:1.7;color:#94a3b8;">SAKU will never ask for your OTP through chat, phone, or email.</p>
            </td>
          </tr>
          <tr>
            <td style="padding:15px 18px;border-top:1px solid #eef2f7;text-align:center;background:#ffffff;">
              <p style="margin:0;font-size:11px;color:#64748b;">&copy; 2026 SAKU Finance &middot; Automated email, please do not reply</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, displayName, cleanOTP, accountEmail)
}

func forgotPasswordEmailHTML(name, email, otp string) string {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "SAKU user"
	}

	accountEmail := strings.TrimSpace(email)
	if accountEmail == "" {
		accountEmail = "-"
	}

	cleanOTP := strings.TrimSpace(otp)
	if cleanOTP == "" {
		cleanOTP = "------"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>Reset Password SAKU</title>
  <style>
    body, table, td, a {
      -webkit-text-size-adjust:100%%;
      -ms-text-size-adjust:100%%;
    }

    body {
      margin:0;
      padding:0;
      width:100%% !important;
      background:#ffffff;
      font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,Arial,sans-serif;
    }

    table {
      border-collapse:separate;
      mso-table-lspace:0pt;
      mso-table-rspace:0pt;
    }
  </style>
</head>

<body style="margin:0;padding:0;background:#ffffff;">
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;font-size:1px;color:#ffffff;line-height:1px;">
    Your SAKU password reset OTP is valid for 10 minutes. Do not share this code with anyone.
  </div>

  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#ffffff;">
    <tr>
      <td align="center" style="padding:28px 16px;">

        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:500px;background:#ffffff;border:1px solid #e5e7eb;border-radius:20px;overflow:hidden;box-shadow:0 6px 20px rgba(15,23,42,.05);">

          <tr>
            <td style="background:#1d4ed8;padding:20px 24px;">
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td style="vertical-align:middle;">
                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                      <tr>
                        <td style="width:40px;height:40px;background:#ffffff;border-radius:10px;text-align:center;vertical-align:middle;">
                          <span style="font-size:18px;font-weight:900;line-height:40px;color:#1d4ed8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">S</span>
                        </td>
                        <td style="padding-left:12px;vertical-align:middle;">
                          <div style="font-size:17px;font-weight:800;color:#ffffff;letter-spacing:.03em;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">SAKU</div>
                          <div style="font-size:11px;color:#c7d2fe;letter-spacing:.08em;text-transform:uppercase;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">Account Security</div>
                        </td>
                      </tr>
                    </table>
                  </td>

                  <td align="right" style="vertical-align:middle;">
                    <span style="display:inline-block;padding:8px 14px;border-radius:999px;background:rgba(255,255,255,.12);border:1px solid rgba(255,255,255,.16);font-size:11px;font-weight:700;color:#ffffff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                      Reset Password
                    </span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <tr>
            <td style="padding:28px;">
              <h1 style="margin:0 0 12px;font-size:18px;font-weight:800;color:#0f172a;line-height:1.35;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                Verify your identity
              </h1>

              <p style="margin:0 0 22px;font-size:14px;line-height:1.8;color:#475569;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                Hi <strong style="color:#0f172a;">%s</strong>, we received a password reset request. Use the code below and do not share it with anyone.
              </p>

              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f8fbff;border:1px solid #dbeafe;border-radius:16px;margin-bottom:14px;">
                <tr>
                  <td align="center" style="padding:22px;">
                    <div style="font-size:11px;font-weight:800;letter-spacing:.18em;color:#2563eb;margin-bottom:14px;text-transform:uppercase;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                      OTP Code
                    </div>

                    <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:0 auto;">
                      <tr>
                        <td style="padding:14px 28px;background:#ffffff;border:1px solid #dbeafe;border-radius:12px;text-align:center;">
                          <span style="font-size:32px;font-weight:900;letter-spacing:.24em;color:#1e3a5f;font-family:'Courier New',Courier,monospace;line-height:1;">
                            %s
                          </span>
                        </td>
                      </tr>
                    </table>

                    <div style="margin-top:14px;font-size:13px;font-weight:700;color:#2563eb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                      Valid for 10 minutes
                    </div>
                  </td>
                </tr>
              </table>

              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom:14px;">
                <tr>
                  <td width="100%%" style="vertical-align:top;">
                    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f0fdf4;border:1px solid #bbf7d0;border-radius:12px;">
                      <tr>
                        <td style="padding:14px;">
                          <div style="font-size:10px;font-weight:800;color:#15803d;margin-bottom:4px;letter-spacing:.08em;text-transform:uppercase;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">Your Email</div>
                          <div style="font-size:13px;font-weight:700;color:#166534;word-break:break-word;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">%s</div>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>

              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#fff7ed;border-left:3px solid #f97316;border-radius:10px;margin-bottom:18px;">
                <tr>
                  <td style="padding:14px;">
                    <p style="margin:0;font-size:13px;line-height:1.7;color:#9a3412;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                      <strong>Not you?</strong> Ignore this email and your account will remain safe.
                    </p>
                  </td>
                </tr>
              </table>

              <p style="margin:0;font-size:12px;line-height:1.7;color:#94a3b8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                SAKU will never ask for your OTP through chat, phone, or email.
              </p>
            </td>
          </tr>

          <tr>
            <td style="padding:16px;border-top:1px solid #eef2f7;text-align:center;background:#ffffff;">
              <p style="margin:0;font-size:11px;color:#64748b;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;">
                &copy; 2026 SAKU &middot; Automated email, please do not reply
              </p>
            </td>
          </tr>

        </table>

      </td>
    </tr>
  </table>
</body>
</html>`, displayName, cleanOTP, accountEmail)
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
	otp, err := s.users.FindOTP(u.ID, "password_reset")
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrInvalidOTP
		}
		return err
	}
	if otp.CodeHash == "" || time.Now().UTC().After(otp.ExpiresAt) {
		return domain.ErrInvalidOTP
	}
	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash), []byte(strings.TrimSpace(req.OTP))); err != nil {
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
	u.AuthProvider = "password"
	if err := s.users.Update(u); err != nil {
		return err
	}
	return s.users.ClearResetOTP(u.ID)
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
		randomBytes := make([]byte, 24)
		if _, rerr := rand.Read(randomBytes); rerr != nil {
			return nil, rerr
		}
		hashed, herr := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(randomBytes)), bcrypt.DefaultCost)
		if herr != nil {
			return nil, herr
		}
		newUser := domain.User{
			Name:         sanitizeName(coalesce(claims.Name, claims.GivenName, email)),
			Email:        email,
			Password:     string(hashed),
			AuthProvider: "google",
			Photo:        claims.Picture,
			Role:         "user",
			Status:       "active",
		}
		code, cerr := s.generateReferralCode()
		if cerr != nil {
			return nil, cerr
		}
		if err := s.users.Create(&newUser); err != nil {
			return nil, err
		}
		ref, err := s.users.EnsureReferralCode(newUser.ID, code)
		if err != nil {
			return nil, err
		}
		newUser.Referral = ref
		u = &newUser
	} else if req.Mode == "register" && strings.EqualFold(u.Status, "active") {
		return nil, domain.ErrAlreadyExists
	} else {
		changed := false
		if u.Photo == "" && claims.Picture != "" {
			u.Photo = claims.Picture
			changed = true
		}
		if !strings.EqualFold(u.AuthProvider, "google") {
			u.AuthProvider = "google"
			changed = true
		}
		if strings.EqualFold(u.Status, "pending_verification") {
			u.Status = "active"
			changed = true
		}
		if changed {
			if err := s.users.Update(u); err != nil {
				return nil, err
			}
		}
	}
	if strings.EqualFold(u.Status, "pending_verification") {
		return nil, domain.ErrAccountNotVerified
	}
	if !strings.EqualFold(u.Status, "active") {
		return nil, domain.ErrInvalidCredentials
	}
	if u.Referral == nil || u.Referral.Code == "" {
		code, cerr := s.generateReferralCode()
		if cerr != nil {
			return nil, cerr
		}
		ref, err := s.users.EnsureReferralCode(u.ID, code)
		if err != nil {
			return nil, err
		}
		u.Referral = ref
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{Token: token, User: authUserResponse(u)}, nil
}

func authUserResponse(u *domain.User) dto.UserResponse {
	resp := dto.UserResponse{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		Role:         u.Role,
		AuthProvider: u.AuthProvider,
		Photo:        u.Photo,
		PhotoURL:     externalPhotoURL(u.Photo),
		Status:       u.Status,
	}
	if u.Referral != nil {
		resp.ReferralCode = u.Referral.Code
		resp.ReferralReward = u.Referral.Reward
	}
	return resp
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

func sanitizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isGmailAddress(email string) bool {
	email = sanitizeEmail(email)
	return strings.HasSuffix(email, "@gmail.com") || strings.HasSuffix(email, "@googlemail.com")
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		out.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(out.String())
}

func (s *service) generateReferralCode() (string, error) {
	for i := 0; i < 8; i++ {
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", err
		}
		code := "SAKU" + strings.ToUpper(hex.EncodeToString(randomBytes))
		if existing, _ := s.users.FindByReferralCode(code); existing == nil {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate referral code")
}
