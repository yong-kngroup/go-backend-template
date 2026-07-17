//go:build integration

package messaging

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	"gorm.io/gorm"
)

func TestRepositoryIntegrationBegin(t *testing.T) {
	db := openConsumptionTestDB(t)
	repo := New(db)

	t.Run("acquires new message and creates processing record", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "begin-new")
		messageKey := "message-1"
		attemptedAt := time.Now().UTC().Truncate(time.Second)
		lockedUntil := attemptedAt.Add(5 * time.Minute)

		result, err := repo.Begin(context.Background(), mq.ConsumptionBegin{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-1",
			AttemptedAt:   attemptedAt,
			LockedUntil:   lockedUntil,
		})
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if result.Decision != mq.ConsumptionDecisionAcquired {
			t.Fatalf("decision = %q, want %q", result.Decision, mq.ConsumptionDecisionAcquired)
		}
		if result.AttemptCount != 1 {
			t.Fatalf("attempt count = %d, want 1", result.AttemptCount)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.Status != string(mq.ConsumptionStatusProcessing) {
			t.Fatalf("status = %q, want %q", record.Status, mq.ConsumptionStatusProcessing)
		}
		if record.AttemptCount != 1 {
			t.Fatalf("stored attempt count = %d, want 1", record.AttemptCount)
		}
		if record.EventName != "user.registered" {
			t.Fatalf("event name = %q, want %q", record.EventName, "user.registered")
		}
		if record.TraceID != "trace-1" {
			t.Fatalf("trace id = %q, want %q", record.TraceID, "trace-1")
		}
		if record.LockedUntil == nil || !record.LockedUntil.Equal(lockedUntil) {
			t.Fatalf("locked until = %v, want %v", record.LockedUntil, lockedUntil)
		}
	})

	t.Run("returns locked when processing record lock is still active", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "begin-locked")
		messageKey := "message-1"
		attemptedAt := time.Now().UTC().Truncate(time.Second)
		lockedUntil := attemptedAt.Add(5 * time.Minute)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-1",
			Status:        string(mq.ConsumptionStatusProcessing),
			AttemptCount:  2,
			LockedUntil:   timePtr(lockedUntil),
			CreatedAt:     attemptedAt.Add(-10 * time.Minute),
			UpdatedAt:     attemptedAt.Add(-1 * time.Minute),
		})

		result, err := repo.Begin(context.Background(), mq.ConsumptionBegin{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-2",
			AttemptedAt:   attemptedAt,
			LockedUntil:   attemptedAt.Add(10 * time.Minute),
		})
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if result.Decision != mq.ConsumptionDecisionLocked {
			t.Fatalf("decision = %q, want %q", result.Decision, mq.ConsumptionDecisionLocked)
		}
		if result.AttemptCount != 2 {
			t.Fatalf("attempt count = %d, want 2", result.AttemptCount)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.AttemptCount != 2 {
			t.Fatalf("stored attempt count = %d, want 2", record.AttemptCount)
		}
		if record.TraceID != "trace-1" {
			t.Fatalf("trace id = %q, want original value", record.TraceID)
		}
	})

	t.Run("reacquires failed record and increments attempt count", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "begin-reacquire")
		messageKey := "message-1"
		attemptedAt := time.Now().UTC().Truncate(time.Second)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "old.event",
			TraceID:       "old-trace",
			Status:        string(mq.ConsumptionStatusFailed),
			AttemptCount:  3,
			LastError:     "boom",
			LockedUntil:   timePtr(attemptedAt.Add(-1 * time.Minute)),
			ProcessedAt:   timePtr(attemptedAt.Add(-2 * time.Minute)),
			CreatedAt:     attemptedAt.Add(-10 * time.Minute),
			UpdatedAt:     attemptedAt.Add(-2 * time.Minute),
		})

		result, err := repo.Begin(context.Background(), mq.ConsumptionBegin{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "new.event",
			TraceID:       "new-trace",
			AttemptedAt:   attemptedAt,
			LockedUntil:   attemptedAt.Add(5 * time.Minute),
		})
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if result.Decision != mq.ConsumptionDecisionAcquired {
			t.Fatalf("decision = %q, want %q", result.Decision, mq.ConsumptionDecisionAcquired)
		}
		if result.AttemptCount != 4 {
			t.Fatalf("attempt count = %d, want 4", result.AttemptCount)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.Status != string(mq.ConsumptionStatusProcessing) {
			t.Fatalf("status = %q, want %q", record.Status, mq.ConsumptionStatusProcessing)
		}
		if record.AttemptCount != 4 {
			t.Fatalf("stored attempt count = %d, want 4", record.AttemptCount)
		}
		if record.EventName != "new.event" {
			t.Fatalf("event name = %q, want %q", record.EventName, "new.event")
		}
		if record.TraceID != "new-trace" {
			t.Fatalf("trace id = %q, want %q", record.TraceID, "new-trace")
		}
		if record.LastError != "" {
			t.Fatalf("last error = %q, want empty", record.LastError)
		}
		if record.ProcessedAt != nil {
			t.Fatalf("processed at = %v, want nil", record.ProcessedAt)
		}
	})

	t.Run("returns done when message is already done", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "begin-done")
		messageKey := "message-1"
		attemptedAt := time.Now().UTC().Truncate(time.Second)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-1",
			Status:        string(mq.ConsumptionStatusDone),
			AttemptCount:  2,
			ProcessedAt:   timePtr(attemptedAt.Add(-1 * time.Minute)),
			CreatedAt:     attemptedAt.Add(-10 * time.Minute),
			UpdatedAt:     attemptedAt.Add(-1 * time.Minute),
		})

		result, err := repo.Begin(context.Background(), mq.ConsumptionBegin{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-2",
			AttemptedAt:   attemptedAt,
			LockedUntil:   attemptedAt.Add(5 * time.Minute),
		})
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if result.Decision != mq.ConsumptionDecisionDone {
			t.Fatalf("decision = %q, want %q", result.Decision, mq.ConsumptionDecisionDone)
		}
		if result.AttemptCount != 2 {
			t.Fatalf("attempt count = %d, want 2", result.AttemptCount)
		}
	})
}

func TestRepositoryIntegrationStatusUpdates(t *testing.T) {
	db := openConsumptionTestDB(t)
	repo := New(db)

	t.Run("MarkDone updates status and processed metadata", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "mark-done")
		messageKey := "message-1"
		now := time.Now().UTC().Truncate(time.Second)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			TraceID:       "trace-1",
			Status:        string(mq.ConsumptionStatusProcessing),
			AttemptCount:  1,
			LastError:     "boom",
			LockedUntil:   timePtr(now.Add(5 * time.Minute)),
			CreatedAt:     now.Add(-10 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		})

		err := repo.MarkDone(context.Background(), consumerGroup, messageKey, now)
		if err != nil {
			t.Fatalf("MarkDone() error = %v", err)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.Status != string(mq.ConsumptionStatusDone) {
			t.Fatalf("status = %q, want %q", record.Status, mq.ConsumptionStatusDone)
		}
		if record.ProcessedAt == nil || !record.ProcessedAt.Equal(now) {
			t.Fatalf("processed at = %v, want %v", record.ProcessedAt, now)
		}
		if record.LockedUntil != nil {
			t.Fatalf("locked until = %v, want nil", record.LockedUntil)
		}
		if record.LastError != "" {
			t.Fatalf("last error = %q, want empty", record.LastError)
		}
	})

	t.Run("MarkFailed updates failure state", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "mark-failed")
		messageKey := "message-1"
		now := time.Now().UTC().Truncate(time.Second)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			Status:        string(mq.ConsumptionStatusProcessing),
			AttemptCount:  2,
			LockedUntil:   timePtr(now.Add(5 * time.Minute)),
			CreatedAt:     now.Add(-10 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		})

		err := repo.MarkFailed(context.Background(), consumerGroup, messageKey, "handler failed", now)
		if err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.Status != string(mq.ConsumptionStatusFailed) {
			t.Fatalf("status = %q, want %q", record.Status, mq.ConsumptionStatusFailed)
		}
		if record.LockedUntil != nil {
			t.Fatalf("locked until = %v, want nil", record.LockedUntil)
		}
		if record.LastError != "handler failed" {
			t.Fatalf("last error = %q, want %q", record.LastError, "handler failed")
		}
	})

	t.Run("MarkDead updates dead state", func(t *testing.T) {
		consumerGroup := uniqueConsumerGroup(t, "mark-dead")
		messageKey := "message-1"
		now := time.Now().UTC().Truncate(time.Second)
		seedConsumptionRecord(t, db, &RecordModel{
			ConsumerGroup: consumerGroup,
			MessageKey:    messageKey,
			EventName:     "user.registered",
			Status:        string(mq.ConsumptionStatusProcessing),
			AttemptCount:  3,
			LockedUntil:   timePtr(now.Add(5 * time.Minute)),
			CreatedAt:     now.Add(-10 * time.Minute),
			UpdatedAt:     now.Add(-5 * time.Minute),
		})

		err := repo.MarkDead(context.Background(), consumerGroup, messageKey, "moved to dlq", now)
		if err != nil {
			t.Fatalf("MarkDead() error = %v", err)
		}

		record := findConsumptionRecord(t, db, consumerGroup, messageKey)
		if record.Status != string(mq.ConsumptionStatusDead) {
			t.Fatalf("status = %q, want %q", record.Status, mq.ConsumptionStatusDead)
		}
		if record.LockedUntil != nil {
			t.Fatalf("locked until = %v, want nil", record.LockedUntil)
		}
		if record.LastError != "moved to dlq" {
			t.Fatalf("last error = %q, want %q", record.LastError, "moved to dlq")
		}
	})
}

func TestRepositoryIntegrationValidation(t *testing.T) {
	db := openConsumptionTestDB(t)
	repo := New(db)

	_, err := repo.Begin(context.Background(), mq.ConsumptionBegin{})
	if !errors.Is(err, errInvalidConsumptionRecord) {
		t.Fatalf("Begin() error = %v, want %v", err, errInvalidConsumptionRecord)
	}

	if err := repo.MarkDone(context.Background(), "", "message-1", time.Now()); !errors.Is(err, errInvalidConsumptionRecord) {
		t.Fatalf("MarkDone() error = %v, want %v", err, errInvalidConsumptionRecord)
	}
	if err := repo.MarkFailed(context.Background(), "group", "", "failed", time.Now()); !errors.Is(err, errInvalidConsumptionRecord) {
		t.Fatalf("MarkFailed() error = %v, want %v", err, errInvalidConsumptionRecord)
	}
	if err := repo.MarkDead(context.Background(), "group", "", "dead", time.Now()); !errors.Is(err, errInvalidConsumptionRecord) {
		t.Fatalf("MarkDead() error = %v, want %v", err, errInvalidConsumptionRecord)
	}
}

func openConsumptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := testsupport.OpenPostgres(t)

	if err := db.AutoMigrate(&RecordModel{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	return db
}

func uniqueConsumerGroup(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("consumption_repo_it_%s_%d", suffix, time.Now().UnixNano())
}

func seedConsumptionRecord(t *testing.T, db *gorm.DB, record *RecordModel) {
	t.Helper()

	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed record error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Where("consumer_group = ? AND message_key = ?", record.ConsumerGroup, record.MessageKey).
			Delete(&RecordModel{}).Error
	})
}

func findConsumptionRecord(t *testing.T, db *gorm.DB, consumerGroup, messageKey string) RecordModel {
	t.Helper()

	var record RecordModel
	if err := db.Where("consumer_group = ? AND message_key = ?", consumerGroup, messageKey).First(&record).Error; err != nil {
		t.Fatalf("find record error = %v", err)
	}
	return record
}

func timePtr(value time.Time) *time.Time {
	return &value
}
