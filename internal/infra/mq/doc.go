// Package mq 将统一消息模型、Kafka 收发、重试和死信读写适配到 Kafka。
//
// 它只依赖第三方库和本包契约。消费状态的 PostgreSQL 实现与死信巡检流程属于
// platform/messaging；调用方通过 ConsumptionStore 将状态能力注入消费者。
package mq
