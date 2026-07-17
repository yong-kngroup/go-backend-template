//go:build integration

package outbox

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRepositoryIntegrationPublishLifecycle(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	if err := db.AutoMigrate(&eventModel{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repo := New(db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	eventOneName := "outbox.integration.one." + suffix
	eventTwoName := "outbox.integration.two." + suffix
	eventOne, _ := NewEvent(eventOneName, `{"id":1}`, "trace-1", "")
	eventTwo, _ := NewEvent(eventTwoName, `{"id":2}`, "trace-2", "")
	if err := repo.Create(context.Background(), eventOne, eventTwo); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Where("event_name IN ?", []string{eventOneName, eventTwoName}).Delete(&eventModel{}).Error
	})

	events, err := repo.ListUnpublished(context.Background(), 10000)
	if err != nil {
		t.Fatalf("ListUnpublished() error = %v", err)
	}
	var firstID, secondID uint
	for _, event := range events {
		if event.GetEventName() == eventOneName {
			firstID = event.GetID()
		}
		if event.GetEventName() == eventTwoName {
			secondID = event.GetID()
		}
	}
	if firstID == 0 || secondID == 0 || firstID >= secondID {
		t.Fatalf("outbox IDs = %d, %d", firstID, secondID)
	}
	publishedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.MarkPublished(context.Background(), []uint{firstID}, publishedAt); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}
	if err := repo.MarkPublished(context.Background(), []uint{firstID}, publishedAt.Add(time.Hour)); err != nil {
		t.Fatalf("repeat MarkPublished() error = %v", err)
	}
	remaining, err := repo.ListUnpublished(context.Background(), 10000)
	if err != nil {
		t.Fatalf("ListUnpublished() after mark error = %v", err)
	}
	for _, event := range remaining {
		if event.GetID() == firstID {
			t.Fatal("published event is still unpublished")
		}
		if event.GetID() == secondID {
			return
		}
	}
	t.Fatal("second event was not returned as unpublished")
}
