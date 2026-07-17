//go:build integration

package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRefreshSessionStoreIntegrationLifecycle(t *testing.T) {
	ctx := context.Background()
	rdb := testsupport.OpenRedis(t)
	store := NewRefreshSessionStore(rdb)
	seed := time.Now().UnixNano()
	userID := uint(seed%1_000_000_000) + 1
	first, _ := domainAuth.NewRefreshSession(fmt.Sprintf("integration-%d-first", seed), userID, "hash-first", time.Now().Add(time.Hour))
	second, _ := domainAuth.NewRefreshSession(fmt.Sprintf("integration-%d-second", seed), userID, "hash-second", time.Now().Add(time.Hour))
	t.Cleanup(func() {
		_ = rdb.Del(ctx, store.sessionKey(first.GetID()), store.sessionKey(second.GetID()), store.userKey(userID)).Err()
	})

	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	byID, err := store.FindByID(ctx, first.GetID())
	if err != nil || byID.GetUserID() != userID || byID.GetTokenHash() != "hash-first" {
		t.Fatalf("FindByID() = %#v, %v", byID, err)
	}
	ttl, err := rdb.PTTL(ctx, store.sessionKey(first.GetID())).Result()
	if err != nil || ttl <= 0 {
		t.Fatalf("session TTL = %v, %v", ttl, err)
	}

	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}
	byUser, err := store.FindByUserID(ctx, userID)
	if err != nil || byUser.GetID() != second.GetID() {
		t.Fatalf("FindByUserID() = %#v, %v", byUser, err)
	}
	if err := store.DeleteByID(ctx, first.GetID()); err != nil {
		t.Fatalf("DeleteByID(first) error = %v", err)
	}
	byUser, err = store.FindByUserID(ctx, userID)
	if err != nil || byUser.GetID() != second.GetID() {
		t.Fatalf("current session after deleting old = %#v, %v", byUser, err)
	}

	if err := store.DeleteByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteByUserID() error = %v", err)
	}
	if _, err := store.FindByID(ctx, second.GetID()); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("FindByID() after delete error = %v", err)
	}
	if _, err := store.FindByUserID(ctx, userID); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("FindByUserID() after delete error = %v", err)
	}
}
