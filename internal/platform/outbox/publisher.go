package outbox

import (
	"context"
	"strconv"
	"time"

	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var outboxPublisherTracer = otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/platform/outbox")

// Publisher 将已提交的 Outbox 事件投递到外部消息系统。
type Publisher interface {
	Publish(ctx context.Context, messageKey, eventName string, payload []byte, traceID, traceContext string) error
}

// OutboxPublisher 负责扫描本地 Outbox 并把事件真正投递到外部消息系统。
// 它实现至少一次投递：成功发送但未成功标记 published_at 的事件会在下次扫描时重发。
type OutboxPublisher struct {
	repo      Store
	publisher Publisher
	logger    logger.Logger
	batchSize int
}

func NewOutboxPublisher(
	repo Store,
	publisher Publisher,
	logger logger.Logger,
	batchSize int,
) *OutboxPublisher {
	if batchSize <= 0 {
		batchSize = 100
	}

	return &OutboxPublisher{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		batchSize: batchSize,
	}
}

// PublishPending 每次抓取一批未发布事件，按顺序投递，成功后再回写 published 状态。
// 发布失败会停止当前批次；已成功发送的前缀仍会被标记，未发送部分留待下次扫描。
func (p *OutboxPublisher) PublishPending(ctx context.Context) (err error) {
	ctx, span := outboxPublisherTracer.Start(ctx, "outbox.publish_pending")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}()

	span.SetAttributes(attribute.Int("outbox.batch_size", p.batchSize))

	events, err := p.repo.ListUnpublished(ctx, p.batchSize)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	publishedIDs := make([]uint, 0, len(events))
	var publishErr error
	for _, event := range events {
		if err := p.publisher.Publish(
			ctx,
			uintString(event.GetID()),
			event.GetEventName(),
			[]byte(event.GetPayload()),
			event.GetTraceID(),
			event.GetTraceContext(),
		); err != nil {
			publishErr = err
			if p.logger != nil {
				p.logger.Error("outbox publish failed", "event", event.GetEventName(), "outbox_id", event.GetID(), "error", err)
			}
			break
		}
		publishedIDs = append(publishedIDs, event.GetID())
	}
	span.SetAttributes(
		attribute.Int("outbox.fetched", len(events)),
		attribute.Int("outbox.published", len(publishedIDs)),
	)

	if err := p.repo.MarkPublished(ctx, publishedIDs, time.Now()); err != nil {
		return err
	}

	if p.logger != nil && len(publishedIDs) > 0 {
		p.logger.Info("outbox events published", "count", len(publishedIDs))
	}

	return publishErr
}

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
