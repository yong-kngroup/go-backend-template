//go:build integration

package idempotency

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	modelIdempotency "github.com/freeDog-wy/go-backend-template/internal/model/idempotency"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRepositoryIntegrationClaimAndComplete(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	if err := db.AutoMigrate(&modelIdempotency.Record{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	repo := New(db)
	actorID := uint(time.Now().UnixNano())
	method, route, key := "POST", "/integration/idempotency", fmt.Sprintf("key-%d", actorID)
	t.Cleanup(func() {
		_ = db.Where("actor_user_id = ?", actorID).Delete(&modelIdempotency.Record{}).Error
	})

	first, claimed, err := repo.Claim(context.Background(), actorID, method, route, key, "first-hash")
	if err != nil || !claimed {
		t.Fatalf("first Claim() = (%#v, %t, %v), want claimed record", first, claimed, err)
	}

	existing, claimed, err := repo.Claim(context.Background(), actorID, method, route, key, "second-hash")
	if err != nil || claimed {
		t.Fatalf("repeat Claim() = (%#v, %t, %v), want existing record", existing, claimed, err)
	}
	if existing.ID != first.ID || existing.RequestHash != "first-hash" {
		t.Fatalf("repeat Claim() record = %#v, want original request", existing)
	}

	if err := repo.Complete(context.Background(), first.ID, []byte(`{"created":true}`), 201); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	if err := repo.Complete(context.Background(), first.ID, []byte(`{"overwritten":true}`), 202); err != nil {
		t.Fatalf("repeat Complete() error = %v", err)
	}

	completed, claimed, err := repo.Claim(context.Background(), actorID, method, route, key, "first-hash")
	if err != nil || claimed {
		t.Fatalf("completed Claim() = (%#v, %t, %v), want existing completed record", completed, claimed, err)
	}
	if completed.CompletedAt == nil || completed.StatusCode != 201 || !bytes.Equal(completed.ResponseBody, []byte(`{"created":true}`)) {
		t.Fatalf("completed record = %#v, want first completed response", completed)
	}
}
