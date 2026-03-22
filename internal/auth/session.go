package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/models"
)

type SessionStore struct {
	DB *pgxpool.Pool
}

const sessionDuration = 7 * 24 * time.Hour

func (s *SessionStore) Create(ctx context.Context, userID string) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(sessionDuration)
	_, err = s.DB.Exec(ctx,
		`INSERT INTO auth_sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (s *SessionStore) Validate(ctx context.Context, sessionID string) (*models.User, error) {
	var u models.User
	err := s.DB.QueryRow(ctx,
		`SELECT u.id, u.email, u.name, u.avatar_url, u.auth_provider, u.auth_provider_id, u.created_at
		 FROM auth_sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.id = $1 AND s.expires_at > NOW()`,
		sessionID).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.AuthProvider, &u.AuthProviderID, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("validate session: %w", err)
	}
	return &u, nil
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, sessionID)
	return err
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= NOW()`)
	return err
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
