package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/radityajayantara/go-observability-boilerplate/internal/model"
	"github.com/radityajayantara/go-observability-boilerplate/internal/repository"

	"go.opentelemetry.io/otel"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo *repository.UserRepository
	tokenSvc *TokenService
}

// NewAuthService creates a new auth service.
func NewAuthService(userRepo *repository.UserRepository, tokenSvc *TokenService) *AuthService {
	return &AuthService{userRepo: userRepo, tokenSvc: tokenSvc}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password string) (*model.User, error) {
	ctx, span := otel.Tracer("service").Start(ctx, "AuthService.Register")
	defer span.End()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, email, string(hash))
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Login validates credentials and returns a token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	ctx, span := otel.Tracer("service").Start(ctx, "AuthService.Login")
	defer span.End()

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.tokenSvc.GenerateTokenPair(ctx, user.ID, user.Email)
}

// GetProfile returns the user profile by ID.
func (s *AuthService) GetProfile(ctx context.Context, userID string) (*model.User, error) {
	ctx, span := otel.Tracer("service").Start(ctx, "AuthService.GetProfile")
	defer span.End()

	return s.userRepo.GetByID(ctx, userID)
}

// Refresh rotates the refresh token and issues new tokens.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, email string) (*TokenPair, error) {
	ctx, span := otel.Tracer("service").Start(ctx, "AuthService.Refresh")
	defer span.End()

	return s.tokenSvc.RefreshTokens(ctx, refreshToken, email)
}

// Logout blacklists the current access token.
func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	ctx, span := otel.Tracer("service").Start(ctx, "AuthService.Logout")
	defer span.End()

	return s.tokenSvc.BlacklistAccessToken(ctx, accessToken)
}
