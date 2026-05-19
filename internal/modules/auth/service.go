package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error
	GoogleLogin(ctx context.Context, req dto.GoogleLoginRequest) (*dto.AuthResponse, error)
}

type service struct {
	users          user.Repository
	jwt            *jwt.Manager
	googleClientID string
	httpClient     *http.Client
}

func NewService(users user.Repository, j *jwt.Manager, googleClientID string) Service {
	return &service{
		users:          users,
		jwt:            j,
		googleClientID: googleClientID,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
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
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return s.users.Update(u)
}

func (s *service) GoogleLogin(ctx context.Context, req dto.GoogleLoginRequest) (*dto.AuthResponse, error) {
	claims, err := s.verifyGoogleIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, err
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
