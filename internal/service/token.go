package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenBlacklisted = errors.New("token has been revoked")
)

// AccessClaims holds the JWT claims for access tokens.
type AccessClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// TokenPair holds both access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// TokenService handles JWT and refresh token operations.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	redis      *redis.Client
}

// NewTokenService creates a new token service.
func NewTokenService(secret string, accessTTL, refreshTTL time.Duration, redisClient *redis.Client) *TokenService {
	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		redis:      redisClient,
	}
}

// GenerateTokenPair creates a new access + refresh token pair.
func (s *TokenService) GenerateTokenPair(ctx context.Context, userID, email string) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.accessTTL)

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
		},
		Email: email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf("signing access token: %w", err)
	}

	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	// Store refresh token in Redis
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	err = s.redis.Set(ctx, refreshKey, userID, s.refreshTTL).Err()
	if err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

// ValidateAccessToken validates a JWT access token and checks the blacklist.
func (s *TokenService) ValidateAccessToken(ctx context.Context, tokenStr string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check blacklist
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenStr)
	exists, err := s.redis.Exists(ctx, blacklistKey).Result()
	if err == nil && exists > 0 {
		return nil, ErrTokenBlacklisted
	}

	return claims, nil
}

// RefreshTokens validates a refresh token and issues a new token pair.
func (s *TokenService) RefreshTokens(ctx context.Context, refreshToken, email string) (*TokenPair, error) {
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	userID, err := s.redis.Get(ctx, refreshKey).Result()
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Delete the old refresh token (rotate)
	s.redis.Del(ctx, refreshKey)

	return s.GenerateTokenPair(ctx, userID, email)
}

// BlacklistAccessToken adds an access token to the blacklist until its expiry.
func (s *TokenService) BlacklistAccessToken(ctx context.Context, tokenStr string) error {
	// Parse to get expiry
	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil {
		return ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok {
		return ErrInvalidToken
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // Already expired
	}

	blacklistKey := fmt.Sprintf("blacklist:%s", tokenStr)
	return s.redis.Set(ctx, blacklistKey, "1", ttl).Err()
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
