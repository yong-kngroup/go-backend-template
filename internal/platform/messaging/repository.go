package messaging

import (
	"context"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/consumer"
	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 基于 GORM 实现消费记录表的读写。
type Repository struct {
	db *gorm.DB
}

var _ consumer.ConsumptionStore = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Begin 使用唯一约束建立首个消费记录；冲突时持有行锁判断锁状态或恢复可重试记录。
// 该过程必须是原子的，避免同一 consumer group 内的多个 worker 并发执行副作用。
func (r *Repository) Begin(ctx context.Context, command consumer.ConsumptionBegin) (consumer.ConsumptionBeginResult, error) {
	if !validConsumptionBegin(command) {
		return consumer.ConsumptionBeginResult{}, errInvalidConsumptionRecord
	}

	var result consumer.ConsumptionBeginResult
	err := repositorytx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		model := &RecordModel{
			ConsumerGroup: command.ConsumerGroup,
			MessageKey:    command.MessageKey,
			EventName:     command.EventName,
			TraceID:       strings.TrimSpace(command.TraceID),
			Status:        string(consumer.ConsumptionStatusProcessing),
			AttemptCount:  1,
			LockedUntil:   new(command.LockedUntil),
			CreatedAt:     command.AttemptedAt,
			UpdatedAt:     command.AttemptedAt,
		}

		createResult := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "consumer_group"}, {Name: "message_key"}},
			DoNothing: true,
		}).Create(model)
		if createResult.Error != nil {
			return createResult.Error
		}
		if createResult.RowsAffected > 0 {
			result = consumer.ConsumptionBeginResult{
				Decision:     consumer.ConsumptionDecisionAcquired,
				AttemptCount: 1,
			}
			return nil
		}

		var existing RecordModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("consumer_group = ? AND message_key = ?", command.ConsumerGroup, command.MessageKey).
			First(&existing).Error; err != nil {
			return err
		}

		switch consumer.ConsumptionStatus(existing.Status) {
		case consumer.ConsumptionStatusDone, consumer.ConsumptionStatusDead:
			result = consumer.ConsumptionBeginResult{
				Decision:     consumer.ConsumptionDecisionDone,
				AttemptCount: existing.AttemptCount,
			}
			return nil
		case consumer.ConsumptionStatusProcessing:
			if existing.LockedUntil != nil && existing.LockedUntil.After(command.AttemptedAt) {
				result = consumer.ConsumptionBeginResult{
					Decision:     consumer.ConsumptionDecisionLocked,
					AttemptCount: existing.AttemptCount,
				}
				return nil
			}
		}

		nextAttempt := existing.AttemptCount + 1
		if err := tx.Model(&RecordModel{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"event_name":    command.EventName,
				"trace_id":      strings.TrimSpace(command.TraceID),
				"status":        string(consumer.ConsumptionStatusProcessing),
				"attempt_count": nextAttempt,
				"last_error":    "",
				"locked_until":  command.LockedUntil,
				"processed_at":  nil,
				"updated_at":    command.AttemptedAt,
			}).Error; err != nil {
			return err
		}

		result = consumer.ConsumptionBeginResult{
			Decision:     consumer.ConsumptionDecisionAcquired,
			AttemptCount: nextAttempt,
		}
		return nil
	})
	return result, err
}

// MarkDone 将消息置为终态。调用方应在业务处理成功、提交 offset 前调用它。
func (r *Repository) MarkDone(ctx context.Context, consumerGroup, messageKey string, processedAt time.Time) error {
	if strings.TrimSpace(consumerGroup) == "" || strings.TrimSpace(messageKey) == "" || processedAt.IsZero() {
		return errInvalidConsumptionRecord
	}

	return repositorytx.DB(ctx, r.db).
		Model(&RecordModel{}).
		Where("consumer_group = ? AND message_key = ?", consumerGroup, messageKey).
		Updates(map[string]any{
			"status":       string(consumer.ConsumptionStatusDone),
			"processed_at": processedAt,
			"locked_until": nil,
			"last_error":   "",
			"updated_at":   processedAt,
		}).Error
}

// MarkFailed 记录已转发到重试链路的失败，允许锁过期后重新领取。
func (r *Repository) MarkFailed(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error {
	if strings.TrimSpace(consumerGroup) == "" || strings.TrimSpace(messageKey) == "" || failedAt.IsZero() {
		return errInvalidConsumptionRecord
	}

	return repositorytx.DB(ctx, r.db).
		Model(&RecordModel{}).
		Where("consumer_group = ? AND message_key = ?", consumerGroup, messageKey).
		Updates(map[string]any{
			"status":       string(consumer.ConsumptionStatusFailed),
			"locked_until": nil,
			"last_error":   strings.TrimSpace(lastError),
			"updated_at":   failedAt,
		}).Error
}

// MarkDead 记录已转发到死信队列的终态，后续重复投递不再执行业务处理。
func (r *Repository) MarkDead(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error {
	if strings.TrimSpace(consumerGroup) == "" || strings.TrimSpace(messageKey) == "" || failedAt.IsZero() {
		return errInvalidConsumptionRecord
	}

	return repositorytx.DB(ctx, r.db).
		Model(&RecordModel{}).
		Where("consumer_group = ? AND message_key = ?", consumerGroup, messageKey).
		Updates(map[string]any{
			"status":       string(consumer.ConsumptionStatusDead),
			"locked_until": nil,
			"last_error":   strings.TrimSpace(lastError),
			"updated_at":   failedAt,
		}).Error
}

func validConsumptionBegin(command consumer.ConsumptionBegin) bool {
	return strings.TrimSpace(command.ConsumerGroup) != "" &&
		strings.TrimSpace(command.MessageKey) != "" &&
		strings.TrimSpace(command.EventName) != "" &&
		!command.AttemptedAt.IsZero() && !command.LockedUntil.IsZero()
}
