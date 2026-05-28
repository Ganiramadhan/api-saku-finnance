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
	now := time.Now().UTC()
	u.LastLoginAt = &now
	if err := s.users.Update(u); err != nil {
		return nil, err
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
	if name == "" || email == "" || !req.PrivacyAccepted {
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
	if err := s.ensurePasswordNotReused(u, req.NewPassword); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.users.AddPasswordHistory(u.ID, u.Password); err != nil {
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

	return mailer.BlueTemplate(mailer.BlueTemplateData{
		Title:       "Login Authentication",
		Preheader:   "Your SAKU verification code is valid for 5 minutes.",
		Badge:       "Verify Account",
		Greeting:    fmt.Sprintf("Hi %s,", displayName),
		Intro:       "Please use the OTP (One-Time Password) below to complete your SAKU account verification.",
		CodeLabel:   "OTP Code",
		Code:        cleanOTP,
		CodeHint:    "This code is valid for 5 minutes.",
		DetailLabel: "Account Email",
		DetailValue: accountEmail,
		Warning:     "Beware of fraud. Do not share this code with anyone. If you did not make this request, please ignore this email.",
		Footer:      "For further information, please contact SAKU support. This is an automated email, please do not reply.",
	})
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

	return mailer.BlueTemplate(mailer.BlueTemplateData{
		Title:       "Reset Password",
		Preheader:   "Your SAKU password reset OTP is valid for 10 minutes.",
		Badge:       "Account Security",
		Greeting:    fmt.Sprintf("Hi %s,", displayName),
		Intro:       "We received a password reset request for your SAKU account. Please use the OTP below to continue.",
		CodeLabel:   "OTP Code",
		Code:        cleanOTP,
		CodeHint:    "This code is valid for 10 minutes.",
		DetailLabel: "Account Email",
		DetailValue: accountEmail,
		Warning:     "Beware of fraud. Do not share this code with anyone. If you did not request a password reset, ignore this email and your account will remain safe.",
		Footer:      "For further information, please contact SAKU support. This is an automated email, please do not reply.",
	})
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
	if err := s.ensurePasswordNotReused(u, req.NewPassword); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.users.AddPasswordHistory(u.ID, u.Password); err != nil {
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
	if password != strings.TrimSpace(password) {
		return fmt.Errorf("password baru tidak boleh diawali atau diakhiri spasi")
	}
	if len(password) < 8 {
		return fmt.Errorf("password baru minimal 8 karakter")
	}
	if len(password) > 72 {
		return fmt.Errorf("password baru maksimal 72 karakter")
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

func (s *service) ensurePasswordNotReused(u *domain.User, password string) error {
	if strings.TrimSpace(u.Password) != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err == nil {
			return fmt.Errorf("password baru tidak boleh sama dengan password yang pernah digunakan")
		}
	}
	history, err := s.users.ListPasswordHistory(u.ID, 5)
	if err != nil {
		return err
	}
	for _, item := range history {
		if strings.TrimSpace(item.PasswordHash) == "" {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(item.PasswordHash), []byte(password)); err == nil {
			return fmt.Errorf("password baru tidak boleh sama dengan password yang pernah digunakan")
		}
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
	now := time.Now().UTC()
	u.LastLoginAt = &now
	if err := s.users.Update(u); err != nil {
		return nil, err
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
		Phone:        u.Phone,
		Role:         u.Role,
		AuthProvider: u.AuthProvider,
		Photo:        u.Photo,
		PhotoURL:     externalPhotoURL(u.Photo),
		Status:       u.Status,
		LastLoginAt:  u.LastLoginAt,
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
