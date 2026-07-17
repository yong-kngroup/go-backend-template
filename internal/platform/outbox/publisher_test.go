package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

func TestNewOutboxPublisher(t *testing.T) {
	t.Parallel()

	publisher := NewOutboxPublisher(nil, nil, nil, 0)
	if publisher.batchSize != 100 {
		t.Fatalf("batchSize = %d, want %d", publisher.batchSize, 100)
	}
}

func TestPublishPending(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	event1 := newTestOutboxEvent(1, "user.registered", `{"id":1}`, "trace-1", `{"traceparent":"00-1-2-01"}`)
	event2 := newTestOutboxEvent(2, "audit.log.requested", `{"id":2}`, "trace-2", `{"traceparent":"00-3-4-01"}`)
	event3 := newTestOutboxEvent(3, "email.verify.requested", `{"id":3}`, "trace-3", `{"traceparent":"00-5-6-01"}`)

	t.Run("returns list error", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{listErr: errors.New("db unavailable")}
		usecase := NewOutboxPublisher(repo, &stubOutboxPublisher{}, logger.Noop(), 10)

		err := usecase.PublishPending(ctx)
		if err == nil || err.Error() != "db unavailable" {
			t.Fatalf("PublishPending() error = %v, want db unavailable", err)
		}
	})

	t.Run("does nothing when no unpublished events exist", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{}
		publisher := &stubOutboxPublisher{}
		usecase := NewOutboxPublisher(repo, publisher, logger.Noop(), 10)

		err := usecase.PublishPending(ctx)
		if err != nil {
			t.Fatalf("PublishPending() error = %v", err)
		}
		if publisher.publishCalls != 0 {
			t.Fatalf("publish calls = %d, want 0", publisher.publishCalls)
		}
		if repo.markPublishedCalled {
			t.Fatal("MarkPublished() should not be called when no events are fetched")
		}
	})

	t.Run("publishes all events and marks them published", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{
			events: []*Event{event1, event2, event3},
		}
		publisher := &stubOutboxPublisher{}
		usecase := NewOutboxPublisher(repo, publisher, logger.Noop(), 2)

		err := usecase.PublishPending(ctx)
		if err != nil {
			t.Fatalf("PublishPending() error = %v", err)
		}
		if repo.listLimit != 2 {
			t.Fatalf("ListUnpublished() limit = %d, want %d", repo.listLimit, 2)
		}
		if publisher.publishCalls != 3 {
			t.Fatalf("publish calls = %d, want 3", publisher.publishCalls)
		}
		if !repo.markPublishedCalled {
			t.Fatal("MarkPublished() was not called")
		}
		assertPublishedIDs(t, repo.markedIDs, []uint{1, 2, 3})
		assertPublishedCall(t, publisher.calls[0], event1)
		assertPublishedCall(t, publisher.calls[1], event2)
		assertPublishedCall(t, publisher.calls[2], event3)
		if repo.markedAt.IsZero() {
			t.Fatal("MarkPublished() timestamp should not be zero")
		}
	})

	t.Run("returns publish error and only marks successfully published events", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{
			events: []*Event{event1, event2, event3},
		}
		publisher := &stubOutboxPublisher{
			failAtCall: 2,
			publishErr: errors.New("kafka unavailable"),
		}
		usecase := NewOutboxPublisher(repo, publisher, logger.Noop(), 10)

		err := usecase.PublishPending(ctx)
		if err == nil || err.Error() != "kafka unavailable" {
			t.Fatalf("PublishPending() error = %v, want kafka unavailable", err)
		}
		if publisher.publishCalls != 2 {
			t.Fatalf("publish calls = %d, want 2", publisher.publishCalls)
		}
		if !repo.markPublishedCalled {
			t.Fatal("MarkPublished() was not called")
		}
		assertPublishedIDs(t, repo.markedIDs, []uint{1})
	})

	t.Run("returns publish error when first publish fails and marks no ids", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{
			events: []*Event{event1, event2},
		}
		publisher := &stubOutboxPublisher{
			failAtCall: 1,
			publishErr: errors.New("kafka unavailable"),
		}
		usecase := NewOutboxPublisher(repo, publisher, logger.Noop(), 10)

		err := usecase.PublishPending(ctx)
		if err == nil || err.Error() != "kafka unavailable" {
			t.Fatalf("PublishPending() error = %v, want kafka unavailable", err)
		}
		if publisher.publishCalls != 1 {
			t.Fatalf("publish calls = %d, want 1", publisher.publishCalls)
		}
		if !repo.markPublishedCalled {
			t.Fatal("MarkPublished() should still be called for consistency")
		}
		assertPublishedIDs(t, repo.markedIDs, nil)
	})

	t.Run("returns mark published error", func(t *testing.T) {
		t.Parallel()

		repo := &stubOutboxRepository{
			events:  []*Event{event1, event2},
			markErr: errors.New("db update failed"),
		}
		publisher := &stubOutboxPublisher{}
		usecase := NewOutboxPublisher(repo, publisher, logger.Noop(), 10)

		err := usecase.PublishPending(ctx)
		if err == nil || err.Error() != "db update failed" {
			t.Fatalf("PublishPending() error = %v, want db update failed", err)
		}
		if publisher.publishCalls != 2 {
			t.Fatalf("publish calls = %d, want 2", publisher.publishCalls)
		}
		assertPublishedIDs(t, repo.markedIDs, []uint{1, 2})
	})
}

type stubOutboxRepository struct {
	events              []*Event
	listErr             error
	markErr             error
	listLimit           int
	markedIDs           []uint
	markedAt            time.Time
	markPublishedCalled bool
}

func (r *stubOutboxRepository) Create(context.Context, ...*Event) error {
	return nil
}

func (r *stubOutboxRepository) ListUnpublished(_ context.Context, limit int) ([]*Event, error) {
	r.listLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.events, nil
}

func (r *stubOutboxRepository) MarkPublished(_ context.Context, ids []uint, publishedAt time.Time) error {
	r.markPublishedCalled = true
	r.markedIDs = append([]uint(nil), ids...)
	r.markedAt = publishedAt
	return r.markErr
}

type stubOutboxPublisher struct {
	publishCalls int
	failAtCall   int
	publishErr   error
	calls        []publishCall
}

type publishCall struct {
	messageKey   string
	eventName    string
	payload      []byte
	traceID      string
	traceContext string
}

func (p *stubOutboxPublisher) Publish(_ context.Context, messageKey, eventName string, payload []byte, traceID, traceContext string) error {
	p.publishCalls++
	p.calls = append(p.calls, publishCall{
		messageKey:   messageKey,
		eventName:    eventName,
		payload:      append([]byte(nil), payload...),
		traceID:      traceID,
		traceContext: traceContext,
	})
	if p.failAtCall > 0 && p.publishCalls == p.failAtCall {
		return p.publishErr
	}
	return nil
}

func newTestOutboxEvent(id uint, eventName, payload, traceID, traceContext string) *Event {
	return ReconstituteEvent(id, eventName, payload, traceID, traceContext, nil, time.Now())
}

func assertPublishedIDs(t *testing.T, got, want []uint) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("marked ids length = %d, want %d; got=%v want=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("marked ids = %v, want %v", got, want)
		}
	}
}

func assertPublishedCall(t *testing.T, got publishCall, event *Event) {
	t.Helper()

	if got.messageKey != uintString(event.GetID()) {
		t.Fatalf("message key = %q, want %q", got.messageKey, uintString(event.GetID()))
	}
	if got.eventName != event.GetEventName() {
		t.Fatalf("event name = %q, want %q", got.eventName, event.GetEventName())
	}
	if string(got.payload) != event.GetPayload() {
		t.Fatalf("payload = %q, want %q", string(got.payload), event.GetPayload())
	}
	if got.traceID != event.GetTraceID() {
		t.Fatalf("trace id = %q, want %q", got.traceID, event.GetTraceID())
	}
	if got.traceContext != event.GetTraceContext() {
		t.Fatalf("trace context = %q, want %q", got.traceContext, event.GetTraceContext())
	}
}
