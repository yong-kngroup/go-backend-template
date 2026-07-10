# MQ Infra 说明

本文档说明 `internal/infra/mq` 当前的职责边界与实现现状。

## 职责边界

`internal/infra/mq` 只负责本项目的消息语义适配，不负责业务编排。

当前目录分为两层：

- 抽象层
  - `message.go`
  - `consumer.go`
  - `publisher.go`
  - `dead_letter.go`
  - `errors.go`
  - `outbox_adapter.go`
  - `trace.go`
- 适配层
  - `publisher_adapter.go`
  - `kafka_consumer.go`
  - `dead_letter_adapter.go`

其中纯 Kafka 技术能力已经下沉到 `pkg/kafka`：

- `reader.go`
- `writer.go`
- `consumer.go`
- `publisher.go`
- `headers.go`
- `dead_letter.go`

## 当前链路

当前模板的异步链路为：

1. 业务事务先写 `outbox_events`
2. `cmd/cron` 定时扫描 outbox
3. `cmd/cron` 通过 `mq.Publisher` 把消息投递到 Kafka
4. `cmd/worker` 通过 `mq.Consumer` 消费 Kafka 消息

职责划分如下：

- `outbox`
  - 负责本地事务一致性
- `mq`
  - 负责本项目消息语义与 Kafka 的适配

## 统一契约

### Message

```go
type Message struct {
    Key     string
    Event   string
    Payload []byte
    TraceID string
}
```

### Publisher

```go
type Publisher interface {
    Publish(ctx context.Context, message Message) error
}
```

### Consumer

```go
type Consumer interface {
    Handle(eventName string, fn EventHandler)
    Run(ctx context.Context) error
}
```

### Dead Letter

`dead_letter.go` 定义项目死信巡检与回放使用的契约：

- `DeadLetterInspector`
- `DeadLetterReplayer`
- `DeadLetterReplayRequest`
- `DeadLetterReplayResult`

## Publisher 适配

`publisher_adapter.go` 把项目 `mq.Message` 映射成 `pkg/kafka.Message`，再调用 `pkg/kafka.Publisher` 发布。

映射规则：

- `Key` -> Kafka message key
- `Payload` -> Kafka message value
- `event` -> Kafka header
- `trace_id` -> Kafka header

## Consumer 适配

`kafka_consumer.go` 仍然负责本项目的消费治理语义，已接入数据库消费记录表 `message_consumptions`：

- 先用 `consumer_group + message_key` 到 DB 抢占消费权
- 消费记录状态包括：
  - `processing`
  - `done`
  - `failed`
  - `dead`
- 已经是 `done` 或 `dead`
  - 直接提交当前 offset
- `processing` 且锁未过期
  - 当前 worker 返回错误退出竞争
- handler 成功
  - 标记 `done`
  - 提交 offset
- handler 失败
  - 默认视为可重试错误
  - 未超过 `MaxRetries` 时转发到分层 retry topic
  - 超过 `MaxRetries` 或显式不可重试时进入 DLQ topic

当前 retry 策略：

- 分层 retry topic
  - 例如 `retry.30s / retry.5m / retry.30m`
- 每层有独立 reader
- 每层有固定 delay
- delay 只阻塞对应 retry 层，不阻塞主 topic

## DLQ 适配

`dead_letter_adapter.go` 把项目死信语义适配到 `pkg/kafka.DeadLetterQueue`。

拆分后边界如下：

- `pkg/kafka/dead_letter.go`
  - 负责读取 DLQ 原始记录
  - 负责按目标 topic 回放原始记录
  - 不理解本项目的 `event / retry_count / reason` 等业务头语义
- `internal/infra/mq/dead_letter_adapter.go`
  - 负责把原始 Kafka 记录解码成 `mq.DeadLetterMessage`
  - 负责构造本项目回放消息头

当前回放语义有两个关键点：

- 回放消息会生成新的 `message_key`
  - 否则原消息在 `message_consumptions` 里通常已经是 `dead`
  - 复用原 key 会被消费幂等直接短路
- 原始 key 会保留在 `original_message_key` header
  - 同时补充 `replayed_from_dlq`、`dlq_replayed_at` 等回放上下文

这意味着：

- DLQ 回放是“重新投递一条新消息”
- 不是“重置旧消费记录再原地重跑”

## 当前结论

现在这一层的职责已经更清楚：

- `pkg/kafka`
  - 纯 Kafka 技术件
- `internal/infra/mq`
  - 本项目消息模型与 Kafka 的适配

Redis 不再承担消息队列职责，只保留给缓存等其他基础设施使用。
