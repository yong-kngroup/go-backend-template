package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"

	"github.com/redis/go-redis/v9"
)

// RefreshSessionStore 保存 auth 领域的刷新令牌会话。
type RefreshSessionStore struct {
	rdb *redis.Client
}

type refreshSessionPayload struct {
	UserID    uint      `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewRefreshSessionStore(rdb *redis.Client) *RefreshSessionStore {
	return &RefreshSessionStore{rdb: rdb}
}

var _ domainAuth.SessionStore = (*RefreshSessionStore)(nil)

func (s *RefreshSessionStore) Save(ctx context.Context, session *domainAuth.RefreshSession) error {
	ttl := time.Until(session.GetExpiresAt())
	if ttl <= 0 {
		return domainAuth.ErrInvalidSession
	}

	payload, err := json.Marshal(refreshSessionPayload{
		UserID:    session.GetUserID(),
		TokenHash: session.GetTokenHash(),
		ExpiresAt: session.GetExpiresAt(),
	})
	if err != nil {
		return err
	}

	if err := s.rdb.Set(ctx, s.sessionKey(session.GetID()), payload, ttl).Err(); err != nil {
		return err
	}

	return s.rdb.Set(ctx, s.userKey(session.GetUserID()), session.GetID(), ttl).Err()
}

func (s *RefreshSessionStore) FindByID(ctx context.Context, sessionID string) (*domainAuth.RefreshSession, error) {
	payload, err := s.rdb.Get(ctx, s.sessionKey(sessionID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	var decoded refreshSessionPayload
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, err
	}

	return domainAuth.ReconstituteRefreshSession(sessionID, decoded.UserID, decoded.TokenHash, decoded.ExpiresAt), nil
}

func (s *RefreshSessionStore) FindByUserID(ctx context.Context, userID uint) (*domainAuth.RefreshSession, error) {
	sessionID, err := s.rdb.Get(ctx, s.userKey(userID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	return s.FindByID(ctx, sessionID)
}

func (s *RefreshSessionStore) DeleteByID(ctx context.Context, sessionID string) error {
	session, err := s.FindByID(ctx, sessionID)
	if err != nil {
		if err == shared.ErrNotFound {
			return nil
		}
		return err
	}

	if err := s.rdb.Del(ctx, s.sessionKey(sessionID)).Err(); err != nil {
		return err
	}

	currentSessionID, err := s.rdb.Get(ctx, s.userKey(session.GetUserID())).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if currentSessionID == sessionID {
		return s.rdb.Del(ctx, s.userKey(session.GetUserID())).Err()
	}
	return nil
}

func (s *RefreshSessionStore) DeleteByUserID(ctx context.Context, userID uint) error {
	session, err := s.FindByUserID(ctx, userID)
	if err != nil {
		if err == shared.ErrNotFound {
			return nil
		}
		return err
	}
	return s.DeleteByID(ctx, session.GetID())
}

func (s *RefreshSessionStore) sessionKey(sessionID string) string {
	return fmt.Sprintf("auth:refresh:%s", sessionID)
}

func (s *RefreshSessionStore) userKey(userID uint) string {
	return fmt.Sprintf("auth:user_session:%d", userID)
}
